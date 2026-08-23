package matching

import (
	"math"
	"testing"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
)

// TestHasSkill 技能命中（双向包含匹配）表驱动测试。
func TestHasSkill(t *testing.T) {
	tests := []struct {
		name   string
		skill  string
		skills []string
		text   string
		want   bool
	}{
		{"精确命中", "Go", []string{"Go", "PostgreSQL"}, "", true},
		{"大小写不敏感", "golang", []string{"Golang"}, "", true},
		{"空格与标点归一", "PostgreSQL", []string{"PostgreSQL 15"}, "", true},
		{"候选技能包含岗位技能（反向包含）", "微服务", []string{"微服务架构"}, "", true},
		{"岗位技能包含候选技能（正向包含）", "微服务架构", []string{"微服务"}, "", true},
		{"命中简历文本", "Redis", []string{}, "熟悉 Redis 缓存与集群", true},
		{"文本带空格标点归一", "kubernetes", []string{}, "Kubernetes, Docker", true},
		{"未命中", "Java", []string{"Go"}, "精通 C++", false},
		{"空技能串视为命中", "", []string{}, "", true},
		{"多技能候选命中其一", "TypeScript", []string{"React", "TypeScript"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasSkill(tt.skill, tt.skills, tt.text); got != tt.want {
				t.Errorf("HasSkill(%q, %v, %q) = %v, want %v", tt.skill, tt.skills, tt.text, got, tt.want)
			}
		})
	}
}

// TestEducationRank 学历等级映射。
func TestEducationRank(t *testing.T) {
	tests := []struct {
		level string
		want  int
	}{
		{"any", 0},
		{"associate", 1},
		{"bachelor", 2},
		{"master", 3},
		{"doctor", 4},
		{"unknown", 0},
	}
	for _, tt := range tests {
		if got := EducationRank(tt.level); got != tt.want {
			t.Errorf("EducationRank(%q) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

// TestCandidateEducationRank 候选人学历解析（按 level 字符串）。
func TestCandidateEducationRank(t *testing.T) {
	tests := []struct {
		name  string
		parsed domain.ParsedResume
		want  int
	}{
		{"本科", domain.ParsedResume{Education: []domain.EducationItem{{Level: "本科"}}}, 2},
		{"硕士", domain.ParsedResume{Education: []domain.EducationItem{{Level: "硕士"}}}, 3},
		{"研究生", domain.ParsedResume{Education: []domain.EducationItem{{Level: "硕士研究生"}}}, 3},
		{"博士", domain.ParsedResume{Education: []domain.EducationItem{{Level: "博士"}}}, 4},
		{"大专", domain.ParsedResume{Education: []domain.EducationItem{{Level: "大专"}}}, 1},
		{"取最高学历", domain.ParsedResume{Education: []domain.EducationItem{{Level: "大专"}, {Level: "硕士"}}}, 3},
		{"解析不到记 0", domain.ParsedResume{Education: []domain.EducationItem{{Level: "高中"}}}, 0},
		{"无教育经历", domain.ParsedResume{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CandidateEducationRank(tt.parsed); got != tt.want {
				t.Errorf("CandidateEducationRank() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestYearsFactor 年限达标系数。
func TestYearsFactor(t *testing.T) {
	tests := []struct {
		years   float64
		minYears int
		want    float64
	}{
		{5, 3, 1},
		{3, 3, 1},
		{1.5, 3, 0.5},
		{0, 3, 0},
		{2, 0, 1},
		{0, 0, 1},
	}
	for _, tt := range tests {
		if got := YearsFactor(tt.years, tt.minYears); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("YearsFactor(%v, %d) = %v, want %v", tt.years, tt.minYears, got, tt.want)
		}
	}
}

func bachelorResume() domain.ParsedResume {
	return domain.ParsedResume{
		Name:              "张三",
		YearsOfExperience: 5,
		Education:         []domain.EducationItem{{Level: "本科", School: "某某大学", Major: "计算机"}},
		Skills:            []string{"Go", "PostgreSQL", "Redis", "Kubernetes"},
	}
}

// TestEvaluate_HardPass 硬性条件检查与 hard_pass 标记。
func TestEvaluate_HardPass(t *testing.T) {
	req := domain.JobRequirements{
		MustSkills:   []string{"Go", "PostgreSQL", "Redis"},
		NiceSkills:   []string{"Kubernetes"},
		MinEducation: "bachelor",
		MinYears:     3,
	}
	t.Run("全部满足", func(t *testing.T) {
		r := Evaluate(req, bachelorResume(), "")
		if r.HardPass {
			t.Errorf("期望 hard_pass=false，得到 %v（%+v）", r.HardPass, r.HardChecks)
		}
		if r.RuleScore != 100 {
			t.Errorf("ruleScore = %d, want 100", r.RuleScore)
		}
	})
	t.Run("缺少必备技能", func(t *testing.T) {
		resume := bachelorResume()
		resume.Skills = []string{"Go", "PostgreSQL"} // 缺 Redis
		r := Evaluate(req, resume, "")
		if !r.HardPass {
			t.Error("期望 hard_pass=true（缺 Redis）")
		}
		for _, c := range r.HardChecks {
			if c.Name == "must_skill" && !c.Pass {
				if got := c.Detail; got == "" {
					t.Error("hard check detail 不应为空")
				}
				return
			}
		}
		t.Error("未找到未命中的 must_skill 检查项")
	})
	t.Run("学历不足", func(t *testing.T) {
		resume := bachelorResume()
		resume.Education = []domain.EducationItem{{Level: "大专"}}
		r := Evaluate(req, resume, "")
		if !r.HardPass {
			t.Error("期望 hard_pass=true（学历不足）")
		}
	})
	t.Run("年限不足", func(t *testing.T) {
		resume := bachelorResume()
		resume.YearsOfExperience = 1
		r := Evaluate(req, resume, "")
		if !r.HardPass {
			t.Error("期望 hard_pass=true（年限不足）")
		}
	})
	t.Run("应届生投 3 年岗", func(t *testing.T) {
		resume := bachelorResume()
		resume.YearsOfExperience = 0
		r := Evaluate(req, resume, "")
		if !r.HardPass {
			t.Error("期望 hard_pass=true（应届投 3 年岗）")
		}
	})
}

// TestEvaluate_RuleScore 规则分计算。
func TestEvaluate_RuleScore(t *testing.T) {
	t.Run("必备全中加分半中年限满", func(t *testing.T) {
		req := domain.JobRequirements{
			MustSkills: []string{"Go", "PostgreSQL"},
			NiceSkills: []string{"Kubernetes", "微服务"},
			MinYears:   3,
		}
		resume := bachelorResume()
		resume.Skills = []string{"Go", "PostgreSQL", "Kubernetes", "微服务架构"}
		resume.YearsOfExperience = 5
		r := Evaluate(req, resume, "")
		// mustHitRate=1, niceHitRate=1, yearsFactor=1 → 100
		if r.RuleScore != 100 {
			t.Errorf("ruleScore = %d, want 100", r.RuleScore)
		}
	})
	t.Run("必备半中加分零年限半", func(t *testing.T) {
		req := domain.JobRequirements{
			MustSkills: []string{"Go", "Java"},
			NiceSkills: []string{"Kubernetes"},
			MinYears:   4,
		}
		resume := bachelorResume()
		resume.Skills = []string{"Go", "PostgreSQL"}
		resume.YearsOfExperience = 2 // yearsFactor = 0.5
		r := Evaluate(req, resume, "")
		want := int(math.Round(60*0.5 + 25*0 + 15*0.5)) // 30 + 0 + 7.5 = 37.5 → 38
		if r.RuleScore != want {
			t.Errorf("ruleScore = %d, want %d", r.RuleScore, want)
		}
		if !r.HardPass {
			t.Error("缺少必备技能 Java 应 hard_pass=true")
		}
	})
	t.Run("无必备技能约束", func(t *testing.T) {
		req := domain.JobRequirements{NiceSkills: []string{"Go"}, MinYears: 0}
		r := Evaluate(req, domain.ParsedResume{}, "")
		// mustHitRate=1, niceHitRate=0, yearsFactor=1 → 60+0+15=75
		if r.RuleScore != 75 {
			t.Errorf("ruleScore = %d, want 75", r.RuleScore)
		}
		if r.HardPass {
			t.Error("无必备约束时不应 hard_pass")
		}
	})
}

// TestComposite 综合分公式。
func TestComposite(t *testing.T) {
	llm, sem, rule := 80, 60, 70

	if got := CompositeAI(llm, &sem, rule); got != int(math.Round(0.45*80+0.30*60+0.25*70)) {
		t.Errorf("CompositeAI = %d", got)
	}
	if got := CompositeAI(llm, nil, rule); got != int(math.Round(0.75*80+0.25*70)) {
		t.Errorf("CompositeAI(nil semantic) = %d", got)
	}
	if got := CompositeRule(rule, &sem); got != int(math.Round(0.6*70+0.4*60)) {
		t.Errorf("CompositeRule = %d", got)
	}
	if got := CompositeRule(rule, nil); got != 70 {
		t.Errorf("CompositeRule(nil semantic) = %d, want 70", got)
	}
	// clamp
	if got := CompositeAI(100, &sem, 100); got > 100 {
		t.Errorf("clamp 上界失败: %d", got)
	}
}

// TestSemanticScore 语义分。
func TestSemanticScore(t *testing.T) {
	if got := SemanticScore(0.85); got != 85 {
		t.Errorf("SemanticScore(0.85) = %d, want 85", got)
	}
	if got := SemanticScore(-0.2); got != 0 {
		t.Errorf("SemanticScore(-0.2) = %d, want 0", got)
	}
	if got := SemanticScore(1.05); got != 100 {
		t.Errorf("SemanticScore(1.05) = %d, want 100", got)
	}
}

// TestCosine 余弦相似度。
func TestCosine(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0}
	if got := Cosine(a, b); math.Abs(got-1) > 1e-6 {
		t.Errorf("Cosine = %v, want 1", got)
	}
	orth := []float32{0, 1}
	if got := Cosine(a, orth); math.Abs(got) > 1e-6 {
		t.Errorf("Cosine orthogonal = %v, want 0", got)
	}
	if got := Cosine(nil, b); got != 0 {
		t.Errorf("Cosine empty = %v, want 0", got)
	}
	if got := Cosine(a, []float32{1}); got != 0 {
		t.Errorf("Cosine dim mismatch = %v, want 0", got)
	}
}
