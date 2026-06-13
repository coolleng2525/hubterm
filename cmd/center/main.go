package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/coolleng2525/hubterm/internal/center/handler"
	"github.com/coolleng2525/hubterm/internal/center/middleware"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/center/service"
	"github.com/coolleng2525/hubterm/internal/pkg/config"
	"github.com/coolleng2525/hubterm/internal/pkg/health"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

var mainLog = log.New("center")

func init() {
	// Register health checks
	health.Register("database", func() health.CheckResult {
		db := model.GetDB()
		if db == nil {
			return health.CheckResult{Name: "database", Status: "down", Detail: "DB not initialized"}
		}
		sqlDB, err := db.DB()
		if err != nil {
			return health.CheckResult{Name: "database", Status: "down", Detail: err.Error()}
		}
		if err := sqlDB.Ping(); err != nil {
			return health.CheckResult{Name: "database", Status: "down", Detail: err.Error()}
		}
		return health.CheckResult{Name: "database", Status: "ok"}
	})
}

func main() {
	configPath := flag.String("config", "", "path to config file (yaml)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		mainLog.Error("failed to load config", log.Err(err))
		return
	}

	// Propagate config values to environment for downstream consumers
	if cfg.Auth.JWTSecret != "" {
		os.Setenv("JWT_SECRET", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.AdminPassword != "" {
		os.Setenv("ADMIN_PASSWORD", cfg.Auth.AdminPassword)
	}

	// init database
	if err := model.InitDB(cfg.Database.Path); err != nil {
		mainLog.Error("failed to init db", log.Err(err))
		return
	}

	// ensure default admin
	service.EnsureAdminExists()

	r := gin.Default()

	// CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// handlers
	authH := &handler.AuthHandler{DB: model.GetDB()}
	nodeH := &handler.NodeHandler{DB: model.GetDB()}
	portH := &handler.SerialPortHandler{DB: model.GetDB()}
	sessionH := &handler.SessionHandler{DB: model.GetDB()}
	auditH := &handler.AuditLogHandler{DB: model.GetDB()}
	agentWSH := handler.NewAgentWSHandler(model.GetDB())

	// public routes
	r.POST("/api/auth/login", authH.Login)
	r.POST("/api/auth/register", middleware.AuthRequired(), middleware.AdminRequired(), authH.Register)

	// FIXED: node report routes protected by NodeTokenRequired
	r.POST("/api/nodes/report", handler.NodeTokenRequired(model.GetDB()), nodeH.Report)
	r.GET("/api/nodes/pending-commands", handler.NodeTokenRequired(model.GetDB()), nodeH.GetPendingCommands)

	// authenticated routes
	api := r.Group("/api", middleware.AuthRequired())
	{
		api.GET("/auth/profile", authH.Profile)
		api.POST("/auth/refresh", authH.RefreshToken)

		api.GET("/nodes", nodeH.List)
		api.GET("/nodes/:id", nodeH.Get)
		api.POST("/nodes/:id/command", nodeH.Command)
		api.POST("/nodes/:id/exec", func(c *gin.Context) {
			c.Set("agent_ws_handler", agentWSH)
			nodeH.ExecCommand(c)
		})
		api.GET("/nodes/:id/exec/:cmd_id", nodeH.GetExecResult)
		api.POST("/nodes/:id/regenerate-token", middleware.AdminRequired(), nodeH.RegenerateToken)

		api.GET("/serial-ports", portH.List)

		api.GET("/sessions", sessionH.List)
		api.POST("/sessions/:id/kick", sessionH.Kick)
		api.POST("/sessions/:id/assign-master", sessionH.AssignMaster)

		api.GET("/audit-logs", auditH.List)
	}

	// WebSocket — browser clients
	r.GET("/api/ws", func(c *gin.Context) {
		handler.HandleWS(c.Request, c.Writer)
	})

	// WebSocket — agent connections
	r.GET("/api/ws/agent", func(c *gin.Context) {
		agentWSH.HandleAgentWS(c.Writer, c.Request)
	})

	// Health check
	r.GET("/api/health", func(c *gin.Context) {
		results := health.RunAll()
		status := http.StatusOK
		for _, r := range results {
			if r.Status == "down" {
				status = http.StatusServiceUnavailable
				break
			}
		}
		c.JSON(status, gin.H{"checks": results})
	})

	// FIXED: Log upload endpoint for agents
	r.POST("/api/logs", handler.NodeTokenRequired(model.GetDB()), auditH.UploadLogs)

	addr := cfg.Server.Addr()
	mainLog.Info("Center service starting on "+addr, log.String("addr", addr))
	if err := r.Run(addr); err != nil {
		mainLog.Error("failed to start", log.Err(err))
	}
}
