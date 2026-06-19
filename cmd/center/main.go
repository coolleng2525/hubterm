package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/coolleng2525/hubterm/internal/center/handler"
	"github.com/coolleng2525/hubterm/internal/center/middleware"
	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/center/service"
	"github.com/coolleng2525/hubterm/internal/pkg/config"
	"github.com/coolleng2525/hubterm/internal/pkg/script"
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
	scriptH := handler.NewScriptHandler(model.GetDB(), script.NewEngine())
	deviceSvc := service.NewDeviceService(model.GetDB())
	aiH := handler.NewAIHandler(model.GetDB(), deviceSvc, agentWSH)

	// P4-P6 handlers
	topoH := handler.NewTopologyHandler(model.GetDB())
	aliasH := handler.NewAliasHandler(model.GetDB())
	proxyH := handler.NewProxyHandler(model.GetDB())
	centerH := handler.NewRemoteCenterHandler(model.GetDB())
	devMgmtH := handler.NewDeviceMgmtHandler(model.GetDB())
	batchH := handler.NewBatchHandler(model.GetDB(), agentWSH)
	groupH := handler.NewGroupHandler(model.GetDB(), agentWSH)

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
		api.PUT("/auth/password", authH.ChangePassword)

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

		api.POST("/scripts", scriptH.Create)
		api.POST("/scripts/:id/execute", scriptH.Execute)
		api.POST("/scripts/:id/execute-on-node/:node_id", scriptH.ExecuteOnNode)
		api.GET("/scripts", scriptH.List)
		api.GET("/scripts/:id", scriptH.Get)
		api.DELETE("/scripts/:id", scriptH.Delete)
		api.GET("/scripts/:id/results", scriptH.Results)

		// AI-friendly API v1 routes
		v1 := api.Group("/v1")
		{
			v1.GET("/devices", aiH.Discover)
			v1.GET("/devices/:id", aiH.GetDevice)
			v1.GET("/devices/:id/capabilities", aiH.GetCapabilities)
			v1.POST("/devices/:id/exec", aiH.Execute)
			v1.GET("/devices/:id/exec/:cmd_id", aiH.GetResult)
			v1.POST("/scripts", aiH.UploadAndExecute)
		}

		// P4 — 拓扑
		api.GET("/topology", topoH.GetTopology)
		api.GET("/topology/nodes/:id", topoH.GetNodeTopology)
		api.GET("/topology/route", topoH.FindRoute)
		api.GET("/topology/health", topoH.CheckHealth)
		api.POST("/topology/heal", topoH.Heal)
		api.GET("/topology/graph", topoH.GetGraph)

		// P5 — 别名
		api.GET("/aliases", aliasH.List)
		api.POST("/aliases", aliasH.Create)
		api.DELETE("/aliases/:id", aliasH.Delete)
		api.GET("/aliases/resolve", aliasH.Resolve)

		// P5 — 代理
		api.POST("/proxy/connect", proxyH.Connect)
		api.POST("/proxy/disconnect/:session_id", proxyH.Disconnect)
		api.GET("/proxy/sessions", proxyH.ListSessions)

		// P5 — 远程中心
		api.GET("/centers", centerH.List)
		api.GET("/centers/:id", centerH.Get)
		api.POST("/centers", centerH.Create)
		api.PUT("/centers/:id", centerH.Update)
		api.DELETE("/centers/:id", centerH.Delete)
		api.POST("/centers/:id/sync", centerH.Sync)

		// P6 — 设备管理
		api.GET("/devices", devMgmtH.List)
		api.POST("/devices", devMgmtH.Create)
		api.PUT("/devices/:id", devMgmtH.Update)
		api.DELETE("/devices/:id", devMgmtH.Delete)
		api.PATCH("/devices/:id/tags", devMgmtH.UpdateTags)
		api.PATCH("/devices/:id/capabilities", devMgmtH.UpdateCapabilities)

		// P6 — 批量命令
		api.POST("/batch/exec", batchH.Exec)
		api.GET("/batch/exec/:batch_id", batchH.GetResult)

		// P6 — 设备分组
		api.GET("/groups", groupH.ListGroups)
		api.GET("/groups/:id", groupH.GetGroup)
		api.POST("/groups", groupH.CreateGroup)
		api.PUT("/groups/:id", groupH.UpdateGroup)
		api.DELETE("/groups/:id", groupH.DeleteGroup)
		api.POST("/groups/:id/members", groupH.AddMember)
		api.DELETE("/groups/:id/members/:device_id", groupH.RemoveMember)
		api.POST("/groups/:id/exec", groupH.ExecOnGroup)
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

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		mainLog.Info("Center service starting on "+addr, log.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			mainLog.Error("failed to start", log.Err(err))
		}
	}()

	sig := <-sigCh
	mainLog.Info("received shutdown signal", log.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		mainLog.Error("forced shutdown", log.Err(err))
	}

	mainLog.Info("center service stopped")
}
