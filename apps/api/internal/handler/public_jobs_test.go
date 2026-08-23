package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Fanxiwen/recruitmate/apps/api/internal/domain"
	"github.com/gin-gonic/gin"
)

// stubPublicJobLister 测试替身：实现 PublicJobLister 接口。
type stubPublicJobLister struct {
	listFn func(ctx context.Context, q domain.JobListQuery) (*domain.Paginated[domain.JobPosting], error)
	getFn  func(ctx context.Context, id string) (*domain.JobPosting, error)
}

func (s *stubPublicJobLister) ListPublic(ctx context.Context, q domain.JobListQuery) (*domain.Paginated[domain.JobPosting], error) {
	return s.listFn(ctx, q)
}
func (s *stubPublicJobLister) GetPublic(ctx context.Context, id string) (*domain.JobPosting, error) {
	return s.getFn(ctx, id)
}

// stubApplyService 测试替身：实现 ApplyService 接口。
type stubApplyService struct {
	applyFn func(ctx context.Context, jobID string, in *domain.ApplyInput) (*domain.ApplyResult, error)
}

func (s *stubApplyService) Apply(ctx context.Context, jobID string, in *domain.ApplyInput) (*domain.ApplyResult, error) {
	return s.applyFn(ctx, jobID, in)
}

func init() { gin.SetMode(gin.TestMode) }

func sampleJob(id, title string) *domain.JobPosting {
	now := time.Now()
	return &domain.JobPosting{
		ID:             id,
		Title:          title,
		DepartmentID:   "dept-1",
		DepartmentName: "技术部",
		Location:       "上海",
		JobType:        "full_time",
		Headcount:      1,
		Description:    "岗位描述",
		Requirements:   domain.JobRequirements{MustSkills: []string{"Go"}, MinEducation: "bachelor"},
		Status:         "open",
		OwnerID:        "owner-1",
		OwnerName:      "张敏",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func TestPublicJobList(t *testing.T) {
	stub := &stubPublicJobLister{
		listFn: func(ctx context.Context, q domain.JobListQuery) (*domain.Paginated[domain.JobPosting], error) {
			if q.Q != "go" {
				t.Errorf("q = %q, want \"go\"", q.Q)
			}
			return &domain.Paginated[domain.JobPosting]{
				Items:    []domain.JobPosting{*sampleJob("job-1", "后端工程师（Go）")},
				Total:    1,
				Page:     q.Page,
				PageSize: q.PageSize,
			}, nil
		},
	}
	h := NewPublicJobHandler(stub, &stubApplyService{applyFn: func(ctx context.Context, jobID string, in *domain.ApplyInput) (*domain.ApplyResult, error) {
		return nil, nil
	}})
	r := gin.New()
	r.GET("/public/jobs", h.List)

	req := httptest.NewRequest(http.MethodGet, "/public/jobs?q=go&page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body domain.Paginated[domain.JobPosting]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 {
		t.Fatalf("total=%d items=%d, want 1/1", body.Total, len(body.Items))
	}
	if body.Items[0].Title != "后端工程师（Go）" {
		t.Errorf("title = %q", body.Items[0].Title)
	}
}

func TestPublicJobGet(t *testing.T) {
	t.Run("存在返回岗位", func(t *testing.T) {
		stub := &stubPublicJobLister{
			getFn: func(ctx context.Context, id string) (*domain.JobPosting, error) {
				if id != "job-1" {
					t.Errorf("id = %q, want job-1", id)
				}
				return sampleJob("job-1", "产品经理"), nil
			},
		}
		h := NewPublicJobHandler(stub, &stubApplyService{applyFn: func(ctx context.Context, jobID string, in *domain.ApplyInput) (*domain.ApplyResult, error) {
			return nil, nil
		}})
		r := gin.New()
		r.GET("/public/jobs/:id", h.Get)

		req := httptest.NewRequest(http.MethodGet, "/public/jobs/job-1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var job domain.JobPosting
		if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if job.Title != "产品经理" {
			t.Errorf("title = %q, want 产品经理", job.Title)
		}
	})

	t.Run("不存在返回 404 统一错误体", func(t *testing.T) {
		stub := &stubPublicJobLister{
			getFn: func(ctx context.Context, id string) (*domain.JobPosting, error) {
				return nil, domain.NewError(http.StatusNotFound, domain.CodeNotFound, "岗位不存在")
			},
		}
		h := NewPublicJobHandler(stub, &stubApplyService{applyFn: func(ctx context.Context, jobID string, in *domain.ApplyInput) (*domain.ApplyResult, error) {
			return nil, nil
		}})
		r := gin.New()
		r.GET("/public/jobs/:id", h.Get)

		req := httptest.NewRequest(http.MethodGet, "/public/jobs/missing", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
		var body map[string]map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if body["error"]["code"] != domain.CodeNotFound {
			t.Errorf("error.code = %q, want %q", body["error"]["code"], domain.CodeNotFound)
		}
	})
}

func TestPublicJobApply(t *testing.T) {
	apply := &stubApplyService{
		applyFn: func(ctx context.Context, jobID string, in *domain.ApplyInput) (*domain.ApplyResult, error) {
			if jobID != "job-1" {
				t.Errorf("jobID = %q, want job-1", jobID)
			}
			if in.Email != "candidate@example.com" {
				t.Errorf("email = %q", in.Email)
			}
			if in.ResumeFile == nil || in.ResumeFile.FileName != "resume.pdf" {
				t.Errorf("resume file = %+v", in.ResumeFile)
			}
			return &domain.ApplyResult{ID: "app-1"}, nil
		},
	}
	h := NewPublicJobHandler(&stubPublicJobLister{}, apply)
	r := gin.New()
	r.POST("/public/jobs/:id/applications", h.Apply)

	body := &multipartBody{}
	body.AddField("name", "候选人")
	body.AddField("email", "candidate@example.com")
	body.AddField("phone", "13800000000")
	body.AddFile("resume", "resume.pdf", []byte("%PDF-1.4 fake pdf"))

	req := httptest.NewRequest(http.MethodPost, "/public/jobs/job-1/applications", body.Reader())
	req.Header.Set("Content-Type", body.ContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", w.Code, w.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ID != "app-1" {
		t.Errorf("id = %q, want app-1", resp.ID)
	}
}
