package main

import (
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/coolleng2525/hubterm/internal/center/handler"
	"github.com/coolleng2525/hubterm/internal/center/middleware"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/center/service"
	"github.com/coolleng2525/hubterm/internal/pkg/config"
	"github.com/coolleng2525/hubterm/internal/pkg/health"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
	"github.com/gin-gonic/gin"
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

	// handlers
	authH := &handler.AuthHandler{DB: model.GetDB()}
	portH := &handler.SerialPortHandler{DB: model.GetDB()}
	auditH := &handler.AuditLogHandler{DB: model.GetDB()}
	agentWSH := handler.NewAgentWSHandler(model.GetDB())
	terminalH := &handler.TerminalHandler{RecordingDir: "recordings"}
	nodeH := &handler.NodeHandler{DB: model.GetDB(), AgentWS: agentWSH}
	sessionH := &handler.SessionHandler{DB: model.GetDB(), AgentWS: agentWSH}

	// public routes
	r.POST("/api/auth/login", authH.Login)
	r.POST("/api/auth/register", middleware.AuthRequired(), middleware.AdminRequired(), authH.Register)

	// node report routes — Report 自己处理首次注册（无 token 时自动生成）
	r.POST("/api/nodes/report", nodeH.Report)
	r.GET("/api/nodes/pending-commands", handler.NodeTokenRequired(model.GetDB()), nodeH.GetPendingCommands)

	// authenticated routes
	api := r.Group("/api", middleware.AuthRequired())
	{
		api.GET("/auth/profile", authH.Profile)
		api.POST("/auth/refresh", authH.RefreshToken)

		api.GET("/nodes", nodeH.List)
		api.GET("/nodes/:id", nodeH.Get)
		api.POST("/nodes/:id/command", middleware.OperatorRequired(), nodeH.Command)
		api.POST("/nodes/:id/exec", middleware.OperatorRequired(), nodeH.ExecCommand)
		api.GET("/nodes/:id/exec/:cmd_id", middleware.OperatorRequired(), nodeH.GetExecResult)
		api.POST("/nodes/:id/regenerate-token", middleware.AdminRequired(), nodeH.RegenerateToken)

		api.GET("/serial-ports", portH.List)

		api.GET("/sessions", sessionH.List)
		api.POST("/sessions/:id/kick", middleware.OperatorRequired(), sessionH.Kick)
		api.POST("/sessions/:id/assign-master", middleware.OperatorRequired(), sessionH.AssignMaster)

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
	r.GET("/api/v1/terminal/connect", func(c *gin.Context) {
		claims, err := handler.AuthenticateWebSocket(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if claims.Role != "admin" && claims.Role != "operator" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "operator required"})
			return
		}
		terminalH.HandleTerminal(c)
	})
	r.GET("/api/v1/terminal/monitor/:session_id", func(c *gin.Context) {
		if _, err := handler.AuthenticateWebSocket(c.Request); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		terminalH.HandleMonitor(c)
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

	distDir := filepath.Clean("web/dist")
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		cleanURLPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+c.Request.URL.Path)), "/")
		requested := filepath.Join(distDir, filepath.FromSlash(cleanURLPath))
		if info, err := os.Stat(requested); err == nil && !info.IsDir() {
			c.File(requested)
			return
		}
		index := filepath.Join(distDir, "index.html")
		if _, err := os.Stat(index); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "frontend is not built"})
			return
		}
		c.File(index)
	})

	addr := cfg.Server.Addr()
	mainLog.Info("Center service starting on "+addr, log.String("addr", addr))
	if err := r.Run(addr); err != nil {
		mainLog.Error("failed to start", log.Err(err))
	}
}
