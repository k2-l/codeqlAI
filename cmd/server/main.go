package main

import (
	"codeqlAI/configs"
	v1 "codeqlAI/internal/api/v1"
	"codeqlAI/internal/analyzer"
	"codeqlAI/internal/auditor"
	"codeqlAI/internal/database"
	"codeqlAI/internal/queue"
	"codeqlAI/internal/service"
	"codeqlAI/pkg/logger"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func main() {
	// 1. 加载配置
	cfg, err := configs.Load("configs/config.yaml")
	if err != nil {
		panic("failed to load config: " + err.Error())
	}
	logger.Init(cfg.App.Debug)
	logger.Info("config loaded", zap.String("app", cfg.App.Name))

	// 2. 连接数据库
	db, err := database.NewDB(database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
		TimeZone: cfg.Database.TimeZone,
	})
	if err != nil {
		logger.Fatal("failed to connect database", zap.Error(err))
	}
	if err := database.Migrate(db); err != nil {
		logger.Fatal("migration failed", zap.Error(err))
	}
	logger.Info("database ready")

	// 3. 初始化 CodeQL Executor
	executor, err := analyzer.NewExecutor(cfg.CodeQL)
	if err != nil {
		logger.Fatal("failed to init CodeQL executor", zap.Error(err))
	}

	// 4. 初始化 AI 客户端和审计器
	aiClient, err := auditor.NewClient(cfg.AI)
	if err != nil {
		logger.Fatal("failed to init AI client", zap.Error(err))
	}
	auditEngine := auditor.NewAuditor(aiClient, db)

	// 5. 初始化 Redis / Asynq 客户端
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	asynqClient := asynq.NewClient(redisOpt)

	// 6. 初始化业务层（需在 processor 之前，ruleService 注入 processor）
	inspector    := asynq.NewInspector(redisOpt)
	ruleService  := service.NewCustomRuleService(db, "storage/custom_rules")
	scanService  := service.NewScanService(db, asynqClient, inspector)

	// 7. 初始化 Asynq Worker
	concurrency := runtime.NumCPU()
	worker := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{"default": 1},
	})

	processor := queue.NewProcessor(db, executor, auditEngine, ruleService)
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeCodeQLScan, processor.HandleCodeQLScan)
	mux.HandleFunc(queue.TypeAIAudit, processor.HandleAIAudit)

	go func() {
		logger.Info("asynq worker starting...")
		if err := worker.Run(mux); err != nil {
			logger.Fatal("asynq worker failed", zap.Error(err))
		}
	}()
	time.Sleep(500 * time.Millisecond)
	logger.Info("asynq worker started", zap.Int("concurrency", concurrency))

	// 8. 初始化 Gin HTTP Server
	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	apiV1 := router.Group("/api/v1")

	// 扫描任务
	v1.NewHandler(scanService).RegisterRoutes(apiV1)
	// 自定义 QL 规则
	v1.NewRuleHandler(ruleService).RegisterRoutes(apiV1)
	// 漏洞地图 + 任务列表
	v1.NewVulnMapHandler(db).RegisterRoutes(apiV1)
	// AI 设置
	settingsService := service.NewAISettingsService("configs/config.yaml")
	v1.NewSettingsHandler(settingsService).RegisterSettingsRoutes(apiV1)

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		logger.Info("HTTP server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()
	logger.Info("server is ready, press Ctrl+C to stop")

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}
	worker.Shutdown()
	asynqClient.Close()
	logger.Info("server exited")
}
