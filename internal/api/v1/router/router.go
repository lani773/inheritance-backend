// Package router sets up all Gin routes for the Inheritance Choir API.
package router

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/api/v1/handlers"
	"github.com/inheritance-choir/backend/internal/api/v1/middleware"
	"github.com/inheritance-choir/backend/internal/config"
	"github.com/inheritance-choir/backend/internal/notifications"
	"github.com/inheritance-choir/backend/internal/realtime"
	"github.com/inheritance-choir/backend/internal/repository"
	"github.com/inheritance-choir/backend/internal/services"
	jwtpkg "github.com/inheritance-choir/backend/pkg/jwt"
)

// Setup creates and configures the Gin engine with all routes.
func Setup(
	cfg *config.Config,
	db *repository.DB,
	log *zap.Logger,
	jwtMgr *jwtpkg.Manager,
	authSvc *services.AuthService,
	mailer *notifications.Mailer,
	hub *realtime.Hub,
) *gin.Engine {

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// ── Global middleware ─────────────────────────────────────
	r.Use(middleware.Recovery(log))
	r.Use(requestid.New())
	r.Use(middleware.RequestLogger(log))
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// ── CORS ──────────────────────────────────────────────────
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID", "ngrok-skip-browser-warning"},
		ExposeHeaders:    []string{"X-Request-ID", "X-Process-Time"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// ── System routes ─────────────────────────────────────────
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"app":     cfg.AppName,
			"version": cfg.AppVersion,
			"env":     cfg.Env,
		})
	})
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    cfg.AppName + " — Go Gin Backend",
			"version": cfg.AppVersion,
			"api":     "/api/v1",
			"health":  "/health",
			"docs":    "https://github.com/inheritance-choir/backend",
		})
	})
	r.GET("/ws", hub.HandleWebSocket(db, jwtMgr))

	// ── Handler instances ─────────────────────────────────────
	authH := handlers.NewAuthHandler(authSvc)
	memberH := handlers.NewMemberHandler(db, mailer, log)
	contribH := handlers.NewContributionHandler(db, log)
	eventH := handlers.NewEventHandler(db, log)
	attendH := handlers.NewAttendanceHandler(db, log)
	msgH := handlers.NewMessageHandler(db, log)
	analH := handlers.NewAnalyticsHandler(db, log)
	welfareH := handlers.NewWelfareHandler(db, log)
	postH := handlers.NewPostHandler(db, log)
	songH := handlers.NewSongHandler(db, log)
	notifH := handlers.NewNotificationHandler(db, log)
	settH := handlers.NewSettingsHandler(db, log)
	adminH := handlers.NewAdminHandler(db, authSvc, mailer, log)
	searchH := handlers.NewSearchHandler(db, log)
	setlistH := handlers.NewSetlistHandler(db, hub, log)
	budgetH := handlers.NewBudgetHandler(db, hub, log)
	pledgeH := handlers.NewPledgeHandler(db, hub, log)
	autoH := handlers.NewAutomationHandler(db, hub, log)
	apiKeyH := handlers.NewAPIKeyHandler(db, log)
	webhookH := handlers.NewWebhookHandler(db, log)
	uploadH := handlers.NewUploadHandler(db, hub, log)
	prayerH := handlers.NewPrayerHandler(db, hub, log)
	chatH := handlers.NewChatHandler(db, hub, log)
	intelH := handlers.NewIntelligenceHandler(db, log)

	// ── Auth middleware shorthands ─────────────────────────────
	auth := middleware.Authenticate(db, jwtMgr)
	admin := middleware.RequireAdmin()

	// ── API v1 group ──────────────────────────────────────────
	v1 := r.Group("/api/v1")
	v1.Use(middleware.SimpleRateLimiter(cfg.RateLimitRequests))
	v1.GET("/search", auth, searchH.Search)

	// ── AUTH ──────────────────────────────────────────────────
	a := v1.Group("/auth")
	{
		a.POST("/login", authH.Login)
		a.POST("/register", authH.Register)
		a.POST("/verify-otp", authH.VerifyOTP)
		a.POST("/refresh", authH.RefreshToken)
		a.POST("/forgot-password", authH.ForgotPassword)
		a.POST("/reset-password", authH.ResetPassword)
		// Authenticated auth routes
		a.POST("/logout", auth, authH.Logout)
		a.POST("/change-password", auth, authH.ChangePassword)
		a.GET("/me", auth, authH.GetMe)
	}

	// ── MEMBERS ───────────────────────────────────────────────
	m := v1.Group("/members", auth)
	{
		m.GET("", memberH.List)
		m.GET("/stats", memberH.Stats)
		m.GET("/export", admin, memberH.Export)
		m.POST("/import", admin, memberH.Import)
		m.POST("", admin, memberH.Create)
		m.GET("/:id", memberH.GetByID)
		m.PUT("/:id", memberH.Update)
		m.DELETE("/:id", admin, memberH.Delete)
		m.POST("/:id/approve", admin, memberH.Approve)
		m.GET("/:id/qr", memberH.GetQR)
	}

	// ── CONTRIBUTIONS ─────────────────────────────────────────
	ct := v1.Group("/contributions", auth)
	{
		ct.GET("", contribH.List)
		ct.GET("/stats", contribH.Stats)
		ct.GET("/trend", contribH.Trend)
		ct.GET("/export", admin, contribH.Export)
		ct.POST("", contribH.Create)
		ct.POST("/bulk-verify", admin, contribH.BulkVerify)
		ct.GET("/:id", contribH.GetByID)
		ct.PUT("/:id", contribH.Update)
		ct.DELETE("/:id", admin, contribH.Delete)
		ct.POST("/:id/verify", admin, contribH.Verify)
	}

	// ── EVENTS ────────────────────────────────────────────────
	ev := v1.Group("/events", auth)
	{
		ev.GET("", eventH.List)
		ev.GET("/upcoming", eventH.Upcoming)
		ev.GET("/calendar", eventH.Calendar)
		ev.POST("", admin, eventH.Create)
		ev.GET("/:id", eventH.GetByID)
		ev.PUT("/:id", admin, eventH.Update)
		ev.DELETE("/:id", admin, eventH.Delete)
		ev.GET("/:id/attendance", eventH.GetAttendance)
	}

	// ── ATTENDANCE ────────────────────────────────────────────
	at := v1.Group("/attendance", auth)
	{
		at.POST("/bulk", admin, attendH.BulkMark)
		at.GET("", attendH.List)
		at.GET("/stats", attendH.Stats)
		at.POST("/qr-checkin", admin, attendH.QRCheckin)
		at.DELETE("/:id", admin, attendH.Delete)
		at.GET("/excuses", attendH.ListExcuses)
		at.POST("/excuses", attendH.SubmitExcuse)
		at.PUT("/excuses/:id", admin, attendH.ReviewExcuse)
	}

	// ── MESSAGES ──────────────────────────────────────────────
	ms := v1.Group("/messages", auth)
	{
		ms.GET("", msgH.List)
		ms.POST("", msgH.Send)
		ms.GET("/:id", msgH.GetByID)
		ms.PUT("/:id/react", msgH.React)
		ms.DELETE("/:id", msgH.Delete)
	}

	// ── ANALYTICS ─────────────────────────────────────────────
	an := v1.Group("/analytics", auth)
	{
		an.GET("/dashboard", analH.Dashboard)
		an.GET("/contributions", analH.Contributions)
		an.GET("/attendance", analH.Attendance)
		an.GET("/members", analH.Members)
		an.GET("/yoy", analH.YoY)
		an.GET("/audit-log", analH.AuditLog)
	}

	// ── WELFARE ───────────────────────────────────────────────
	wf := v1.Group("/welfare", auth)
	{
		wf.GET("", welfareH.List)
		wf.POST("", admin, welfareH.Create)
		wf.GET("/:id", welfareH.GetByID)
		wf.PUT("/:id", admin, welfareH.Update)
		wf.POST("/:id/timeline", admin, welfareH.AddTimeline)
	}

	// ── POSTS ─────────────────────────────────────────────────
	po := v1.Group("/posts", auth)
	{
		po.GET("", postH.List)
		po.POST("", postH.Create)
		po.GET("/:id", postH.GetByID)
		po.PUT("/:id", postH.Update)
		po.DELETE("/:id", postH.Delete)
		po.POST("/:id/like", postH.Like)
		po.POST("/:id/comments", postH.AddComment)
	}

	// ── SONGS ─────────────────────────────────────────────────
	so := v1.Group("/songs", auth)
	{
		so.GET("", songH.List)
		so.POST("", admin, songH.Create)
		so.GET("/:id", songH.GetByID)
		so.PUT("/:id", admin, songH.Update)
		so.DELETE("/:id", admin, songH.Delete)
	}

	// ── NOTIFICATIONS ─────────────────────────────────────────
	nt := v1.Group("/notifications", auth)
	{
		nt.GET("", notifH.List)
		nt.POST("/read-all", notifH.MarkAllRead)
		nt.POST("/:id/read", notifH.MarkRead)
		nt.DELETE("/:id", notifH.Delete)
	}

	// ── SETTINGS ──────────────────────────────────────────────
	st := v1.Group("/settings", auth)
	{
		st.GET("", settH.Get)
		st.PUT("", admin, settH.Update)
	}

	// ── ADMIN ─────────────────────────────────────────────────
	ad := v1.Group("/admin", auth, admin)
	{
		ad.GET("/registrations", adminH.ListRegistrations)
		ad.POST("/registrations/:id/approve", adminH.ApproveRegistration)
		ad.POST("/registrations/:id/reject", adminH.RejectRegistration)
		ad.GET("/system-stats", adminH.SystemStats)
		ad.POST("/broadcast", adminH.Broadcast)
		ad.DELETE("/tokens/:id", adminH.RevokeSessions)
	}

	sl := v1.Group("/setlists", auth)
	{
		sl.GET("", setlistH.List)
		sl.POST("", admin, setlistH.Create)
		sl.GET("/:id", setlistH.Get)
		sl.PUT("/:id", admin, setlistH.Update)
		sl.DELETE("/:id", admin, setlistH.Delete)
	}
	bg := v1.Group("/budgets", auth)
	{
		bg.GET("", budgetH.List)
		bg.POST("", admin, budgetH.Upsert)
	}
	pc := v1.Group("/pledges", auth)
	{
		pc.GET("/campaigns", pledgeH.ListCampaigns)
		pc.POST("/campaigns", admin, pledgeH.CreateCampaign)
		pc.PUT("/campaigns/:id", admin, pledgeH.UpdateCampaign)
		pc.GET("", pledgeH.ListPledges)
		pc.POST("", pledgeH.CreatePledge)
		pc.PUT("/:id", pledgeH.UpdatePledge)
	}
	au := v1.Group("/automation", auth, admin)
	{
		au.GET("", autoH.List)
		au.POST("", autoH.Create)
		au.PUT("/:id", autoH.Update)
		au.DELETE("/:id", autoH.Delete)
		au.POST("/:id/run", autoH.Run)
	}
	ak := v1.Group("/api-keys", auth, admin)
	{
		ak.GET("", apiKeyH.List)
		ak.POST("", apiKeyH.Create)
		ak.POST("/:id/revoke", apiKeyH.Revoke)
	}
	wh := v1.Group("/webhooks", auth, admin)
	{
		wh.GET("", webhookH.List)
		wh.POST("", webhookH.Create)
		wh.PUT("/:id", webhookH.Update)
		wh.DELETE("/:id", webhookH.Delete)
		wh.POST("/:id/test", webhookH.Test)
	}
	up := v1.Group("/uploads", auth)
	{
		up.GET("", uploadH.List)
		up.POST("", uploadH.Create)
	}
	pr := v1.Group("/prayer", auth)
	{
		pr.GET("", prayerH.List)
		pr.POST("", prayerH.Create)
		pr.POST("/:id/pray", prayerH.Pray)
		pr.PUT("/:id", prayerH.Update)
	}
	ch := v1.Group("/chat", auth)
	{
		ch.GET("/messages", chatH.List)
		ch.POST("/messages", chatH.Send)
	}
	in := v1.Group("/intelligence", auth)
	{
		in.GET("/forecast", intelH.Forecast)
	}

	return r
}
