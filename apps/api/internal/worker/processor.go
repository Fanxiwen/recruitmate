package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/ai"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/file"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/matching"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/repo"
	"github.com/hibiken/asynq"
)

// ProcessorConfig worker 处理配置（评分权重等）。
type ProcessorConfig struct {
	AILLMW        float64
	AISemanticW   float64
	AIRuleW       float64
	AITopN        int
	DeepSeekModel string
}

// Processor AI 匹配流水线处理器。
type Processor struct {
	Apps    repo.ApplicationRepo
	Jobs    repo.JobRepo
	AI      ai.Client
	Storage file.Storage
	Queue   *Queue
	Config  ProcessorConfig
}

// HandleResumeProcess resume:process 任务处理（完整匹配流水线）。
func (p *Processor) HandleResumeProcess(ctx context.Context, t *asynq.Task) error {
	var payload PayloadResumeProcess
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("worker: unmarshal resume:process payload: %w", err)
	}
	log := slog.With("application_id", payload.ApplicationID, "version", payload.Version)
	log.Info("resume:process start")

	// 0. 加载投递与岗位
	data, err := p.Apps.GetProcessData(ctx, payload.ApplicationID)
	if err != nil {
		return fmt.Errorf("worker: load application: %w", err)
	}
	job, err := p.Jobs.Get(ctx, data.JobID)
	if err != nil {
		return fmt.Errorf("worker: load job: %w", err)
	}

	// 1. 文本提取（文件优先，失败退回 resume_text）
	text, err := p.extractText(ctx, data)
	if err != nil {
		log.Warn("resume:process text extraction failed, fallback or abort", "error", err)
		if strings.TrimSpace(data.ResumeText) == "" {
			// 无文件文本也无提交文本：标记解析失败并结束
			if err := p.Apps.SetParseFailed(ctx, payload.ApplicationID); err != nil {
				return fmt.Errorf("worker: mark parse failed: %w", err)
			}
			log.Warn("resume:process aborted: no resume text available")
			return nil
		}
		text = data.ResumeText
	}

	// 2. LLM 结构化解析（无解析结果或此前解析失败落库的空壳时重试）
	parsed := data.ParsedResume
	parseFailed := false
	if parsed == nil || parsedIsEmpty(parsed) {
		parsed, err = p.parseResume(ctx, text)
		if err != nil {
			parseFailed = true
			parsed = &domain.ParsedResume{}
			log.Warn("resume:process LLM parse failed, continue with raw text", "error", err)
		}
	}

	// 3. 硬性条件检查 + 规则分（纯函数）
	rule := matching.Evaluate(job.Requirements, *parsed, text)

	// 4. Embedding：简历文本（截断 8000 字符）→ SiliconFlow；岗位向量按需生成并缓存
	var semantic *int
	resumeVec, err := p.embedResume(ctx, data.ResumeEmbedding, text)
	if err == nil && len(resumeVec) > 0 {
		jobVec, err2 := p.embedJob(ctx, job)
		if err2 == nil && len(jobVec) > 0 {
			score := matching.SemanticScore(matching.Cosine(resumeVec, jobVec))
			semantic = &score
		} else if err2 != nil {
			log.Warn("resume:process job embedding failed", "error", err2)
		}
	} else if err != nil {
		log.Warn("resume:process resume embedding failed", "error", err)
	}

	// 5. 判断是否走 LLM 评委：该岗位 prelim 高于当前值的投递数 < AI_TOPN（漏斗式成本控制）
	var llmScore *int
	var judge *judgeResult
	prelim := matching.Prelim(rule.RuleScore, semantic)
	higher := p.countHigherPrelim(ctx, data.JobID, payload.ApplicationID, prelim)
	chatOK := p.chatEnabled()
	if chatOK && higher < p.Config.AITopN {
		var err error
		judge, err = p.llmJudge(ctx, job, parsed)
		if err != nil {
			log.Warn("resume:process LLM judge failed, fallback to rule", "error", err)
		} else {
			s := clampScore(judge.Score)
			llmScore = &s
		}
	}

	// 6. 综合分 + 写回
	detail := &domain.MatchDetail{
		RuleScore:     rule.RuleScore,
		SemanticScore: semantic,
		LLMScore:      llmScore,
		Strengths:     []string{},
		Gaps:          []string{},
		HardChecks:    rule.HardChecks,
		Engine:        "rule",
		ScoredAt:      time.Now().UTC(),
	}
	composite := 0
	if llmScore != nil {
		detail.Engine = "ai"
		detail.Model = &p.Config.DeepSeekModel
		if len(judge.Strengths) > 0 {
			detail.Strengths = judge.Strengths
		}
		if len(judge.Gaps) > 0 {
			detail.Gaps = judge.Gaps
		}
		detail.Risk = judge.Risk
		detail.Summary = judge.Summary
		composite = matching.CompositeAIW(*llmScore, semantic, rule.RuleScore,
			p.Config.AILLMW, p.Config.AISemanticW, p.Config.AIRuleW)
	} else {
		composite = matching.CompositeRuleW(rule.RuleScore, semantic, p.Config.AIRuleW, p.Config.AISemanticW)
	}
	detail.Score = composite

	if err := p.Apps.UpdateMatch(ctx, payload.ApplicationID, float64(composite), detail,
		rule.HardPass, parsed, resumeVec, parseFailed); err != nil {
		return fmt.Errorf("worker: persist match: %w", err)
	}
	log.Info("resume:process done",
		"composite", composite, "engine", detail.Engine,
		"rule", rule.RuleScore, "semantic", semantic, "llm", llmScore, "hard_pass", rule.HardPass)
	return nil
}

// HandleJobRescore job:rescore 任务处理：重新生成岗位向量并批量重算全部投递。
func (p *Processor) HandleJobRescore(ctx context.Context, t *asynq.Task) error {
	var payload PayloadJobRescore
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("worker: unmarshal job:rescore payload: %w", err)
	}
	log := slog.With("job_id", payload.JobID)
	log.Info("job:rescore start")

	job, err := p.Jobs.Get(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("worker: load job: %w", err)
	}

	// 重新生成岗位 embedding（JD 文本 = 标题 + 描述 + 结构化要求）
	jdText := buildJDText(job)
	if p.embedEnabled() {
		if vec, err := p.AI.Embed(ctx, truncateText(jdText, 8000)); err == nil && len(vec) > 0 {
			if err := p.Jobs.SetEmbedding(ctx, job.ID, vec); err != nil {
				log.Error("job:rescore set embedding failed", "error", err)
			} else {
				log.Info("job:rescore embedding regenerated", "dim", len(vec))
			}
		} else if err != nil {
			log.Warn("job:rescore embedding failed", "error", err)
		}
	}

	// 重新读取岗位（SetEmbedding 会刷新 updated_at，用作新版本号）
	job, err = p.Jobs.Get(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("worker: reload job: %w", err)
	}

	// 全部投递重新入队 resume:process（worker 会跳过文本提取与 LLM 解析）
	ids, err := p.Apps.ListIDsByJob(ctx, job.ID)
	if err != nil {
		return fmt.Errorf("worker: list applications: %w", err)
	}
	enqueued := 0
	for _, id := range ids {
		if err := p.Queue.EnqueueResumeProcess(ctx, id, job.UpdatedAt.Unix()); err != nil {
			log.Error("job:rescore enqueue application failed", "application_id", id, "error", err)
			continue
		}
		enqueued++
	}
	log.Info("job:rescore done", "applications", len(ids), "enqueued", enqueued)
	return nil
}

// extractText 从投递数据提取文本：有文件先下载并提取，失败退回 resume_text。
func (p *Processor) extractText(ctx context.Context, data *repo.ApplicationProcessData) (string, error) {
	if data.ResumeFileKey != "" {
		content, err := p.Storage.Download(ctx, data.ResumeFileKey)
		if err != nil {
			return "", fmt.Errorf("worker: download resume: %w", err)
		}
		text, err := file.ExtractText(data.ResumeFileKey, content)
		if err != nil {
			return "", fmt.Errorf("worker: extract resume text: %w", err)
		}
		if strings.TrimSpace(text) != "" {
			return text, nil
		}
	}
	return strings.TrimSpace(data.ResumeText), nil
}

// parseResume LLM 结构化解析简历，失败重试一次。
func (p *Processor) parseResume(ctx context.Context, text string) (*domain.ParsedResume, error) {
	const system = `你是简历解析器。请把用户提供的简历文本解析为严格的 JSON 对象，字段如下：
{
  "name": "姓名",
  "email": "邮箱",
  "phone": "电话",
  "yearsOfExperience": 数字（工作年限，无法判断填 0）,
  "education": [{"level": "学历（如 本科/硕士/博士/大专）", "school": "学校", "major": "专业", "endYear": 毕业年份数字或 null}],
  "skills": ["技能列表"],
  "workExperience": [{"company": "公司", "title": "职位", "startDate": "开始时间", "endDate": "结束时间", "description": "工作描述"}],
  "summary": "个人总结"
}
找不到的字段一律用空值（字符串用 ""，数组用 []，数字用 0）。只输出 JSON，不要输出任何其他内容。`
	user := truncateText(text, 12000)

	var out domain.ParsedResume
	err := p.AI.ChatJSON(ctx, system, user, &out)
	if err != nil {
		// 重试一次
		err = p.AI.ChatJSON(ctx, system, user, &out)
	}
	if err != nil {
		return nil, err
	}
	// 健壮性修复
	if out.Skills == nil {
		out.Skills = []string{}
	}
	if out.Education == nil {
		out.Education = []domain.EducationItem{}
	}
	if out.WorkExperience == nil {
		out.WorkExperience = []domain.WorkExperienceItem{}
	}
	return &out, nil
}

// parsedIsEmpty 判断结构化简历是否为「空壳」（解析失败时的落库形态），
// 空壳需要在下一次处理时重新解析。
func parsedIsEmpty(p *domain.ParsedResume) bool {
	if p == nil {
		return true
	}
	return p.Name == "" && p.Email == "" && p.Phone == "" &&
		p.YearsOfExperience == 0 && len(p.Skills) == 0 &&
		len(p.Education) == 0 && len(p.WorkExperience) == 0 && p.Summary == ""
}

// judgeResult LLM 评委输出。
type judgeResult struct {
	Score     int      `json:"score"`
	Strengths []string `json:"strengths"`
	Gaps      []string `json:"gaps"`
	Risk      string   `json:"risk"`
	Summary   string   `json:"summary"`
}

// llmJudge LLM 评委打分：按固定 rubric 输出 JSON。
func (p *Processor) llmJudge(ctx context.Context, job *domain.JobPosting, parsed *domain.ParsedResume) (*judgeResult, error) {
	const system = `你是资深招聘专家。请严格按照以下评分标准评估候选人是否匹配该岗位：
- 技能匹配：40%
- 经验匹配：30%
- 教育背景：15%
- 综合潜力：15%
输出严格的 JSON 对象（不要输出其他内容）：
{
  "score": 0到100的整数,
  "strengths": ["最多3条候选人优势"],
  "gaps": ["最多3条候选人不足"],
  "risk": "用人风险或注意事项，一句话",
  "summary": "不超过100字的综合评语"
}`

	resumeJSON, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	user := fmt.Sprintf("岗位标题：%s\n岗位职责：%s\n岗位要求：\n%s\n\n候选人简历（结构化解析结果）：\n%s",
		job.Title, job.Description, formatRequirements(job.Requirements), string(resumeJSON))

	var out judgeResult
	if err := p.AI.ChatJSON(ctx, system, user, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// embedResume 简历向量化（已有向量则复用）。
func (p *Processor) embedResume(ctx context.Context, existing []float32, text string) ([]float32, error) {
	if len(existing) > 0 {
		return existing, nil
	}
	if !p.embedEnabled() {
		return nil, ai.ErrDisabled
	}
	return p.AI.Embed(ctx, truncateText(text, 8000))
}

// embedJob 岗位向量化（按需生成并缓存到 job_postings.embedding）。
func (p *Processor) embedJob(ctx context.Context, job *domain.JobPosting) ([]float32, error) {
	if vec, err := p.Jobs.GetEmbedding(ctx, job.ID); err == nil && len(vec) > 0 {
		return vec, nil
	} else if err != nil && err != repo.ErrNotFound {
		return nil, err
	}
	if !p.embedEnabled() {
		return nil, ai.ErrDisabled
	}
	vec, err := p.AI.Embed(ctx, truncateText(buildJDText(job), 8000))
	if err != nil {
		return nil, err
	}
	if err := p.Jobs.SetEmbedding(ctx, job.ID, vec); err != nil {
		slog.Warn("worker: cache job embedding failed", "job_id", job.ID, "error", err)
	}
	return vec, nil
}

// countHigherPrelim 统计岗位下 prelim 高于当前值的投递数（排除自身）。
func (p *Processor) countHigherPrelim(ctx context.Context, jobID, currentID string, prelim int) int {
	scored, err := p.Apps.ListScoredByJob(ctx, jobID)
	if err != nil {
		slog.Warn("worker: list scored applications failed", "job_id", jobID, "error", err)
		return 0
	}
	n := 0
	for _, s := range scored {
		if s.ID == currentID {
			continue
		}
		if matching.Prelim(s.RuleScore, s.SemanticScore) > prelim {
			n++
		}
	}
	return n
}

// chatEnabled AI LLM 评委是否可用（缺 Key 时降级 engine='rule'）。
func (p *Processor) chatEnabled() bool {
	chat, _ := ai.Capabilities(p.AI)
	return p.AI != nil && chat
}

// embedEnabled AI 向量化是否可用。
func (p *Processor) embedEnabled() bool {
	_, embed := ai.Capabilities(p.AI)
	return p.AI != nil && embed
}

// buildJDText 构造岗位 JD 文本（用于向量化）。
func buildJDText(job *domain.JobPosting) string {
	return fmt.Sprintf("岗位：%s\n职责：%s\n要求：%s",
		job.Title, job.Description, formatRequirements(job.Requirements))
}

// formatRequirements 格式化结构化要求。
func formatRequirements(r domain.JobRequirements) string {
	return fmt.Sprintf("必备技能：%s；加分技能：%s；最低学历：%s；最低年限：%d 年",
		strings.Join(r.MustSkills, "、"),
		strings.Join(r.NiceSkills, "、"),
		r.MinEducation, r.MinYears)
}

// truncateText 截断文本到指定字符数（按 rune 截取避免截断中文）。
func truncateText(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// clampScore 分数限幅 0-100。
func clampScore(s int) int {
	if s > 100 {
		return 100
	}
	if s < 0 {
		return 0
	}
	return s
}
