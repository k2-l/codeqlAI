package main

import (
	"codeqlAI/configs"
	"codeqlAI/internal/auditor"
	"codeqlAI/internal/database"
	"codeqlAI/pkg/logger"
	"codeqlAI/internal/model"

	"go.uber.org/zap"
)

func main() {
	// 1. 加载配置
	cfg, err := configs.Load("configs/config.yaml")
	if err != nil {
		panic("failed to load config: " + err.Error())
	}
	logger.Init(cfg.App.Debug)
	logger.Info("config loaded")

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
	database.Migrate(db)
	logger.Info("database ready")

	// 3. 初始化 AI 客户端
	aiClient, err := auditor.NewClient(cfg.AI)
	if err != nil {
		logger.Fatal("failed to init AI client", zap.Error(err))
	}

	// 4. 初始化审计器
	auditEngine := auditor.NewAuditor(aiClient, db)

	// 5. 从数据库取第一条 pending 的 Finding 来测试
	var finding model.Finding
	if err := db.Where("audit_status = ?", model.AuditStatusPending).First(&finding).Error; err != nil {
		logger.Fatal("no pending findings found, please run the scan first", zap.Error(err))
	}

	logger.Info("auditing finding",
		zap.String("finding_id", finding.ID.String()),
		zap.String("rule_id", finding.RuleID),
		zap.String("file", finding.FilePath),
		zap.Int("line", finding.StartLine),
	)

	// 6. 发起 AI 审计
	result, err := auditEngine.AuditFinding(finding.ID)
	if err != nil {
		logger.Fatal("audit failed", zap.Error(err))
	}

	logger.Info("audit saved to database",
		zap.String("ai_result_id", result.ID.String()),
		zap.Bool("is_exploitable", result.IsExploitable),
		zap.Float64("confidence", result.Confidence),
		zap.Int("prompt_tokens", result.PromptTokens),
		zap.Int("completion_tokens", result.CompletionTokens),
	)
}