package domain

import "testing"

// TestCanTransition 阶段流转状态机：合法流转、非法跳步、同阶段幂等。
func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to ApplicationStage
		want     bool
	}{
		// 合法流转
		{StageNew, StageScreening, true},
		{StageNew, StageRejected, true},
		{StageScreening, StageRejected, true},
		{StageInterview, StageRejected, true},
		{StageManagerInterview, StageRejected, true},
		{StageOfferPending, StageOffered, true},
		{StageOfferPending, StageManagerInterview, true}, // 驳回回退部门负责人面
		{StageOffered, StageHired, true},
		{StageOffered, StageRejected, true},
		{StageRejected, StageNew, true}, // 误杀恢复
		// 同阶段幂等
		{StageNew, StageNew, true},
		// 非法：面试阶段由「安排面试/完成面试」动作驱动，不可手动流转
		{StageNew, StageInterview, false},
		{StageNew, StageManagerInterview, false},
		{StageNew, StageOfferPending, false},
		{StageNew, StageOffered, false},
		{StageNew, StageHired, false},
		{StageScreening, StageInterview, false},        // 须安排 HR 面
		{StageScreening, StageManagerInterview, false}, // 不能跳过 HR 面
		{StageInterview, StageManagerInterview, false}, // 须 HR 面通过后安排负责人面
		{StageInterview, StageOffered, false},
		{StageInterview, StageHired, false},
		{StageManagerInterview, StageOfferPending, false}, // 须负责人面通过后发起 Offer
		{StageManagerInterview, StageOffered, false},
		{StageOfferPending, StageNew, false},
		{StageHired, StageNew, false},
		{StageHired, StageRejected, false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

// TestCandidateStatusFromStage 求职者视角状态映射。
func TestCandidateStatusFromStage(t *testing.T) {
	cases := map[ApplicationStage]string{
		StageNew:              "processing",
		StageScreening:        "screening",
		StageInterview:        "interviewing",
		StageManagerInterview: "interviewing", // 两轮面试对求职者统一展示「面试中」
		StageOfferPending:     "offered",
		StageOffered:          "offered",
		StageHired:            "hired",
		StageRejected:         "rejected",
	}
	for stage, want := range cases {
		if got := CandidateStatusFromStage(string(stage)); got != want {
			t.Errorf("CandidateStatusFromStage(%s) = %s, want %s", stage, got, want)
		}
	}
}
