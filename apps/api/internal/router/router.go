// Package router 组装 Gin 路由。
package router

import (
	"github.com/Fanxiwen/recruitmate/apps/api/internal/handler"
	"github.com/Fanxiwen/recruitmate/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Deps 路由依赖（handler 实例与 JWT 密钥）。
type Deps struct {
	JWTSecret string

	PublicJobs   *handler.PublicJobHandler
	PublicAuth   *handler.PublicAuthHandler
	InternalAuth *handler.InternalAuthHandler
	InternalJobs *handler.InternalJobHandler
	InternalApps *handler.InternalApplicationHandler
}

// New 创建 Gin 引擎并注册全部路由。
func New(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1")

	// ============ 公开接口（无需认证） ============
	pub := v1.Group("/public")
	{
		pub.GET("/jobs", d.PublicJobs.List)
		pub.GET("/jobs/:id", d.PublicJobs.Get)
		pub.POST("/jobs/:id/applications", d.PublicJobs.Apply)
		pub.GET("/departments", d.PublicJobs.ListDepartments)

		pub.POST("/auth/email-code", d.PublicAuth.SendEmailCode)
		pub.POST("/auth/verify", d.PublicAuth.Verify)

		// 候选人 JWT 保护
		pub.GET("/my/applications", middleware.RequireCandidate(d.JWTSecret), d.PublicAuth.MyApplications)
	}

	// ============ 内部接口（JWT + RBAC） ============
	internal := v1.Group("/internal")
	{
		// 登录无需鉴权
		internal.POST("/auth/login", d.InternalAuth.Login)

		authGroup := internal.Group("", middleware.RequireInternal(d.JWTSecret))
		{
			authGroup.GET("/me", d.InternalAuth.Me)
			authGroup.GET("/departments", d.InternalAuth.ListDepartments)

			authGroup.POST("/jobs", d.InternalJobs.Create)
			authGroup.GET("/jobs", d.InternalJobs.List)
			authGroup.GET("/jobs/:id", d.InternalJobs.Get)
			authGroup.PATCH("/jobs/:id", d.InternalJobs.Update)
			authGroup.POST("/jobs/:id/submit", d.InternalJobs.Submit)
			authGroup.POST("/jobs/:id/approve", d.InternalJobs.Approve)
			authGroup.POST("/jobs/:id/reject", d.InternalJobs.Reject)
			authGroup.POST("/jobs/:id/close", d.InternalJobs.Close)
			authGroup.POST("/jobs/:id/reopen", d.InternalJobs.Reopen)
			authGroup.GET("/jobs/:id/applications", d.InternalJobs.ListApplications)
			authGroup.GET("/jobs/:id/stats", d.InternalJobs.Stats)

			authGroup.GET("/applications/:id", d.InternalApps.Get)
			authGroup.PATCH("/applications/:id/stage", d.InternalApps.SetStage)
			authGroup.POST("/applications/batch", d.InternalApps.Batch)
			authGroup.GET("/applications/:id/resume-url", d.InternalApps.ResumeURL)
		}
	}

	return r
}
