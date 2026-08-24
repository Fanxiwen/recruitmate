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
		{StageScreening, StageInterview, true},
		{StageInterview, StageOfferPending, true},
		{StageOfferPending, StageOffered, true},
		{StageOfferPending, StageInterview, true},
		{StageOffered, StageHired, true},
		{StageOffered, StageRejected, true},
		{StageRejected, StageNew, true}, // 误杀恢复
		// 同阶段幂等
		{StageNew, StageNew, true},
		// 非法跳步
		{StageNew, StageInterview, false},
		{StageNew, StageOfferPending, false},
		{StageNew, StageOffered, false},
		{StageNew, StageHired, false},
		{StageScreening, StageOffered, false},
		{StageInterview, StageHired, false},
		{StageHired, StageNew, false},
		{StageHired, StageRejected, false},
		{StageOfferPending, StageNew, false},
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
		StageNew:          "processing",
		StageScreening:    "screening",
		StageInterview:    "interviewing",
		StageOfferPending: "offered",
		StageOffered:      "offered",
		StageHired:        "hired",
		StageRejected:     "rejected",
	}
	for stage, want := range cases {
		if got := CandidateStatusFromStage(string(stage)); got != want {
			t.Errorf("CandidateStatusFromStage(%s) = %s, want %s", stage, got, want)
		}
	}
}
