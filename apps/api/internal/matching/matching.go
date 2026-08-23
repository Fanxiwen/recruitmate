// Package matching 实现简历匹配的纯规则引擎（硬性条件检查 + 规则分 + 综合分）。
//
// 本包全部为纯函数，不依赖外部 I/O，保证可解释、可单测。
// 评分模型（与 docs/technical-design.md 第 8 节一致）：
//
//	ruleScore = round(60*mustHitRate + 25*niceHitRate + 15*yearsFactor)
//	composite(AI)   = round(0.45*llm + 0.30*semantic + 0.25*rule)
//	composite(rule) = rule（有 embedding 时 0.6*rule + 0.4*semantic）
package matching

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
)

// 学历等级 → 数值 rank（any=0, associate=1, bachelor=2, master=3, doctor=4）。
var educationRank = map[string]int{
	string(domain.EducationAny):       0,
	string(domain.EducationAssociate): 1,
	string(domain.EducationBachelor):  2,
	string(domain.EducationMaster):    3,
	string(domain.EducationDoctor):    4,
}

// EducationRank 返回岗位学历要求的等级数值；未知等级按 0（不限）处理。
func EducationRank(level string) int {
	if r, ok := educationRank[level]; ok {
		return r
	}
	return 0
}

// normalizeText 归一化文本：小写、去除空白与常见标点，用于技能命中比较。
func normalizeText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// NormalizeSkill 归一化单个技能（与 normalizeText 相同规则，导出供测试使用）。
func NormalizeSkill(s string) string { return normalizeText(s) }

// HasSkill 判断单个必备技能是否命中：
// 标准：归一化后技能串出现在候选技能列表任一元素中，或出现在简历文本中；
// 双向包含匹配（a 包含 b 或 b 包含 a）均算命中，例如「微服务」命中「微服务架构」。
func HasSkill(skill string, skills []string, text string) bool {
	ns := normalizeText(skill)
	if ns == "" {
		return true
	}
	for _, s := range skills {
		cand := normalizeText(s)
		if cand != "" && (strings.Contains(ns, cand) || strings.Contains(cand, ns)) {
			return true
		}
	}
	if strings.Contains(normalizeText(text), ns) {
		return true
	}
	return false
}

// HitRate 命中率：命中的技能数 / 技能总数；技能列表为空视为 1（无约束即满分）。
func HitRate(skills []string, candidateSkills []string, text string) float64 {
	if len(skills) == 0 {
		return 1
	}
	hit := 0
	for _, s := range skills {
		if HasSkill(s, candidateSkills, text) {
			hit++
		}
	}
	return float64(hit) / float64(len(skills))
}

// CandidateEducationRank 从解析结果推导候选人最高学历等级：
// 按教育经历 level 字符串包含「博士/硕士/研究生/本科/大专」映射，解析不到记 0。
func CandidateEducationRank(p domain.ParsedResume) int {
	best := 0
	for _, e := range p.Education {
		r := 0
		switch {
		case strings.Contains(e.Level, "博士"):
			r = 4
		case strings.Contains(e.Level, "硕士"), strings.Contains(e.Level, "研究生"):
			r = 3
		case strings.Contains(e.Level, "本科"):
			r = 2
		case strings.Contains(e.Level, "大专"):
			r = 1
		}
		if r > best {
			best = r
		}
	}
	return best
}

// YearsFactor 年限达标系数：minYears<=0 时为 1，否则 min(1, years/minYears)。
func YearsFactor(years float64, minYears int) float64 {
	if minYears <= 0 {
		return 1
	}
	f := years / float64(minYears)
	if f > 1 {
		return 1
	}
	if f < 0 {
		return 0
	}
	return f
}

// Result 规则引擎计算结果。
type Result struct {
	HardChecks  []domain.HardCheck
	HardPass    bool
	RuleScore   int
	MustHitRate float64
	NiceHitRate float64
	YearsFactor float64
}

// Evaluate 执行硬性条件检查与规则分计算（纯函数）。
//
// 硬性条件（任一不满足 → hard_pass=true）：
//  1. 必备技能逐项命中
//  2. 学历等级 ≥ 岗位要求（any 不设限）
//  3. 工作年限 ≥ 最低年限
//
// 规则分 = round(60*mustHitRate + 25*niceHitRate + 15*yearsFactor)。
func Evaluate(req domain.JobRequirements, parsed domain.ParsedResume, resumeText string) Result {
	skills := parsed.Skills
	if skills == nil {
		skills = []string{}
	}

	var checks []domain.HardCheck

	// 1. 必备技能
	mustHit := 0
	mustTotal := len(req.MustSkills)
	for _, s := range req.MustSkills {
		pass := HasSkill(s, skills, resumeText)
		if pass {
			mustHit++
		}
		checks = append(checks, domain.HardCheck{
			Name:   "must_skill",
			Pass:   pass,
			Detail: skillCheckDetail(s, pass),
		})
	}
	mustHitRate := 1.0
	if mustTotal > 0 {
		mustHitRate = float64(mustHit) / float64(mustTotal)
	}

	// 2. 加分技能（只影响分数，不构成硬性条件）
	niceHitRate := HitRate(req.NiceSkills, skills, resumeText)

	// 3. 学历
	requiredRank := EducationRank(req.MinEducation)
	candidateRank := CandidateEducationRank(parsed)
	eduPass := requiredRank == 0 || candidateRank >= requiredRank
	checks = append(checks, domain.HardCheck{
		Name:   "education",
		Pass:   eduPass,
		Detail: educationCheckDetail(req.MinEducation, candidateRank),
	})

	// 4. 年限
	yearsFactor := YearsFactor(parsed.YearsOfExperience, req.MinYears)
	yearsPass := req.MinYears <= 0 || parsed.YearsOfExperience >= float64(req.MinYears)
	checks = append(checks, domain.HardCheck{
		Name:   "years",
		Pass:   yearsPass,
		Detail: yearsCheckDetail(req.MinYears, parsed.YearsOfExperience),
	})

	hardPass := false
	for _, c := range checks {
		if !c.Pass {
			hardPass = true
			break
		}
	}

	ruleScore := int(math.Round(60*mustHitRate + 25*niceHitRate + 15*yearsFactor))
	if ruleScore > 100 {
		ruleScore = 100
	}
	if ruleScore < 0 {
		ruleScore = 0
	}

	return Result{
		HardChecks:  checks,
		HardPass:    hardPass,
		RuleScore:   ruleScore,
		MustHitRate: mustHitRate,
		NiceHitRate: niceHitRate,
		YearsFactor: yearsFactor,
	}
}

func skillCheckDetail(skill string, pass bool) string {
	if pass {
		return "命中必备技能「" + skill + "」"
	}
	return "缺少必备技能「" + skill + "」"
}

func educationCheckDetail(required string, candidateRank int) string {
	return "岗位要求学历≥" + required + "，候选人学历等级 " + strconv.Itoa(candidateRank)
}

func yearsCheckDetail(minYears int, years float64) string {
	return "岗位要求年限≥" + strconv.Itoa(minYears) + " 年，候选人 " + strconv.FormatFloat(years, 'f', 1, 64) + " 年"
}

// ============ 语义与综合分 ============

// Cosine 计算两个向量的余弦相似度；任一向量为空返回 0。
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// SemanticScore 余弦相似度 → 0-100 分（负值截断为 0）。
func SemanticScore(cosine float64) int {
	if cosine <= 0 {
		return 0
	}
	s := int(math.Round(cosine * 100))
	if s > 100 {
		s = 100
	}
	return s
}

// Prelim 预筛选分：ruleScore*0.6 + semanticScore*0.4；无语义分时退化为 ruleScore。
// 用于判断是否值得调用 LLM 评委（漏斗式成本控制）。
func Prelim(ruleScore int, semanticScore *int) int {
	if semanticScore == nil {
		return ruleScore
	}
	return int(math.Round(float64(ruleScore)*0.6 + float64(*semanticScore)*0.4))
}

// CompositeAI 综合分（AI 评委可用）：
//   - semantic 可用：round(0.45*llm + 0.30*semantic + 0.25*rule)
//   - semantic 缺失：按 0.75*llm + 0.25*rule 重新归一
func CompositeAI(llmScore int, semanticScore *int, ruleScore int) int {
	var s float64
	if semanticScore == nil {
		s = 0.75*float64(llmScore) + 0.25*float64(ruleScore)
	} else {
		s = 0.45*float64(llmScore) + 0.30*float64(*semanticScore) + 0.25*float64(ruleScore)
	}
	return clampScore(int(math.Round(s)))
}

// CompositeRule 综合分（无 AI 评委）：
//   - semantic 可用：round(0.6*rule + 0.4*semantic)
//   - semantic 缺失：rule
func CompositeRule(ruleScore int, semanticScore *int) int {
	if semanticScore == nil {
		return clampScore(ruleScore)
	}
	return clampScore(int(math.Round(0.6*float64(ruleScore) + 0.4*float64(*semanticScore))))
}

// CompositeAIW 综合分（AI 评委，权重可配置）：
//   - semantic 可用：round((llmW*llm + semW*sem + ruleW*rule) / (llmW+semW+ruleW))
//   - semantic 缺失：按 llm/rule 权重重新归一
//
// 默认权重 0.45/0.30/0.25 时与 CompositeAI 一致（总和为 1）。
func CompositeAIW(llmScore int, semanticScore *int, ruleScore int, llmW, semW, ruleW float64) int {
	if semanticScore == nil {
		total := llmW + ruleW
		if total <= 0 {
			total = 1
		}
		return clampScore(int(math.Round(llmW/total*float64(llmScore) + ruleW/total*float64(ruleScore))))
	}
	total := llmW + semW + ruleW
	if total <= 0 {
		total = 1
	}
	return clampScore(int(math.Round((llmW*float64(llmScore) + semW*float64(*semanticScore) + ruleW*float64(ruleScore)) / total)))
}

// CompositeRuleW 综合分（无 AI 评委，权重可配置）：
//   - semantic 可用：round((ruleW*rule + semW*sem) / (ruleW+semW))
//   - semantic 缺失：rule
func CompositeRuleW(ruleScore int, semanticScore *int, ruleW, semW float64) int {
	if semanticScore == nil {
		return clampScore(ruleScore)
	}
	total := ruleW + semW
	if total <= 0 {
		total = 1
	}
	return clampScore(int(math.Round((ruleW*float64(ruleScore) + semW*float64(*semanticScore)) / total)))
}

func clampScore(s int) int {
	if s > 100 {
		return 100
	}
	if s < 0 {
		return 0
	}
	return s
}
