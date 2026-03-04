package service

import (
	"codeqlAI/internal/model"
	"codeqlAI/pkg/logger"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// allowedLanguages 支持的语言白名单（包级常量，避免重复创建）
var allowedLanguages = map[string]bool{
	"java":       true,
	"go":         true,
	"python":     true,
	"javascript": true,
	"cpp":        true,
}

// CustomRuleService 自定义 QL 规则管理
type CustomRuleService struct {
	db       *gorm.DB
	rulesDir string // 规则文件存放目录
}

// NewCustomRuleService 初始化，rulesDir 为规则文件落盘目录
func NewCustomRuleService(db *gorm.DB, rulesDir string) (*CustomRuleService, error) {
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create rules directory: %w", err)
	}
	return &CustomRuleService{db: db, rulesDir: rulesDir}, nil
}

// CreateRuleRequest 创建规则的请求参数
type CreateRuleRequest struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description"`
	Language    string `json:"language"    binding:"required"`
	Content     string `json:"content"     binding:"required"` // QL 查询内容
}

// UpdateRuleRequest 更新规则
type UpdateRuleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	IsEnabled   *bool  `json:"is_enabled"`
}

// CreateRule 创建规则：写入数据库 + 落盘 .ql 文件
func (s *CustomRuleService) CreateRule(req CreateRuleRequest) (*model.CustomRule, error) {
	if !allowedLanguages[req.Language] {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}

	rule := model.CustomRule{
		Name:        req.Name,
		Description: req.Description,
		Language:    model.Language(req.Language),
		Content:     req.Content,
		IsEnabled:   true,
	}

	if err := s.db.Create(&rule).Error; err != nil {
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}

	// 落盘 .ql 文件
	if err := s.writeRuleFile(&rule); err != nil {
		logger.Warn("failed to write rule file", zap.String("rule_id", rule.ID.String()), zap.Error(err))
	}

	return &rule, nil
}

// ListRules 按语言筛选规则列表（language="" 则返回全部）
func (s *CustomRuleService) ListRules(language string) ([]model.CustomRule, error) {
	var rules []model.CustomRule
	q := s.db.Order("created_at DESC")
	if language != "" {
		q = q.Where("language = ?", language)
	}
	if err := q.Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}
	return rules, nil
}

// GetRule 按 ID 查询单条规则
func (s *CustomRuleService) GetRule(id uuid.UUID) (*model.CustomRule, error) {
	var rule model.CustomRule
	if err := s.db.First(&rule, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}
	return &rule, nil
}

// UpdateRule 更新规则内容并同步到磁盘文件
func (s *CustomRuleService) UpdateRule(id uuid.UUID, req UpdateRuleRequest) (*model.CustomRule, error) {
	var rule model.CustomRule
	if err := s.db.First(&rule, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}

	// 检查是否有更新内容
	if req.Name == "" && req.Description == "" && req.Content == "" && req.IsEnabled == nil {
		return &rule, nil // 无更新，直接返回
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.IsEnabled != nil {
		updates["is_enabled"] = *req.IsEnabled
	}

	if err := s.db.Model(&rule).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}

	// 如果内容有更新，重新落盘
	if req.Content != "" {
		rule.Content = req.Content
		if err := s.writeRuleFile(&rule); err != nil {
			logger.Warn("failed to update rule file", zap.Error(err))
		}
	}

	return &rule, nil
}

// DeleteRule 删除规则及磁盘文件
func (s *CustomRuleService) DeleteRule(id uuid.UUID) error {
	var rule model.CustomRule
	if err := s.db.First(&rule, "id = ?", id).Error; err != nil {
		return fmt.Errorf("rule not found: %w", err)
	}

	// 删磁盘文件
	if rule.FilePath != "" {
		if err := os.Remove(rule.FilePath); err != nil && !os.IsNotExist(err) {
			logger.Warn("failed to delete rule file", zap.String("path", rule.FilePath), zap.Error(err))
		}
	}

	if err := s.db.Delete(&rule).Error; err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	logger.Info("custom rule deleted", zap.String("rule_id", id.String()))
	return nil
}

// GetRuleFilePath 获取规则对应的 .ql 文件路径（供 executor 使用）
func (s *CustomRuleService) GetRuleFilePath(id uuid.UUID) (string, error) {
	rule, err := s.GetRule(id)
	if err != nil {
		return "", err
	}
	if rule.FilePath == "" {
		// 文件不存在则重新生成
		if err := s.writeRuleFile(rule); err != nil {
			return "", err
		}
	}
	return rule.FilePath, nil
}

// writeRuleFile 将 QL 内容写入磁盘，文件名为 {id}.ql
func (s *CustomRuleService) writeRuleFile(rule *model.CustomRule) error {
	filename := fmt.Sprintf("%s.ql", rule.ID.String())
	path := filepath.Join(s.rulesDir, string(rule.Language), filename)

	// 确保语言子目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create language directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(rule.Content), 0644); err != nil {
		return fmt.Errorf("failed to write ql file: %w", err)
	}

	// 更新数据库中的文件路径
	s.db.Model(rule).Update("file_path", path)
	rule.FilePath = path

	logger.Info("rule file written", zap.String("path", path))
	return nil
}