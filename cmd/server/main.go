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
		logger.Fatal("未能初始化 CodeQL 执行器", zap.Error(err))
	}

	// 4. 初始化 AI 客户端和审计器
	aiClient, err := auditor.NewClient(cfg.AI)
	if err != nil {
		logger.Fatal("未能初始化人工智能客户端", zap.Error(err))
	}
	auditEngine := auditor.NewAuditor(aiClient, db)

	// 5. 初始化 Asynq 客户端（用于 API 层推送任务）
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	// 6. 初始化 Asynq Worker（在后台消费任务）
	concurrency := runtime.NumCPU()
	worker := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues: map[string]int{
			"default": 1,
		},
	})

	processor := queue.NewProcessor(db, executor, auditEngine)
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeCodeQLScan, processor.HandleCodeQLScan)
	mux.HandleFunc(queue.TypeAIAudit, processor.HandleAIAudit)

	// Worker 在 goroutine 中异步运行
	go func() {
		logger.Info("asynq 工作进程正在启动...")
		if err := worker.Run(mux); err != nil {
			logger.Fatal("asynq 工作进程出现故障", zap.Error(err))
		}
	}()

	// 给 Worker 一点时间完成初始化再打印日志
	time.Sleep(500 * time.Millisecond)
	logger.Info("asynq 工作进程已启动", zap.Int("concurrency", concurrency))

	// 7. 初始化业务逻辑层
	inspector := asynq.NewInspector(redisOpt)
	scanService := service.NewScanService(db, asynqClient, inspector)

	// 8. 初始化 Gin HTTP Server
	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// 注册路由
	apiV1 := router.Group("/api/v1")
	handler := v1.NewHandler(scanService)
	handler.RegisterRoutes(apiV1)

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	logger.Info("HTTP 服务器已启动", zap.String("addr", addr))

	// 用标准库 http.Server 包裹 Gin，支持优雅退出
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// 在 goroutine 里启动 HTTP Server
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 服务器故障", zap.Error(err))
		}
	}()
	logger.Info("服务器已准备就绪,请按 Ctrl+C 停止服务")

	// 监听系统信号（Ctrl+C 或 kill 命令）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // 阻塞，直到收到信号

	logger.Info("正在关闭服务器……")

	// 给正在处理的请求最多 10 秒时间完成
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 停止接收新请求，等待存量请求完成
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("服务器被迫关闭", zap.Error(err))
	}

	// 停止 Asynq Worker（等待当前任务执行完）
	worker.Shutdown()
	asynqClient.Close()

	logger.Info("服务器已退出")
}