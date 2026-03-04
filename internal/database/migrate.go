package database

import (
	"fmt"
	"log"

	"codeqlAI/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 数据库连接配置
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	TimeZone string
}

// NewDB 初始化数据库连接
func NewDB(cfg Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode, cfg.TimeZone,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// Migrate 执行数据库迁移，按依赖顺序建表
// 顺序很重要：被依赖的表必须先创建（projects -> tasks -> findings -> ai_results）
func Migrate(db *gorm.DB) error {
	log.Println("[Migration] Starting database migration...")

	err := db.AutoMigrate(
		&model.Project{},    // 无外键依赖，最先建
		&model.Task{},       // 依赖 projects
		&model.Finding{},    // 依赖 tasks
		&model.AiResult{},   // 依赖 findings
		&model.CustomRule{}, // 独立表，无外键
	)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("[Migration] All tables migrated successfully.")
	return nil
}