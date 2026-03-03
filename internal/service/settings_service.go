package service

import (
	"codeqlAI/configs"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AISettingsService 负责读写 AI 配置
type AISettingsService struct {
	configPath string
}

// NewAISettingsService 初始化，configPath 为 config.yaml 的路径
func NewAISettingsService(configPath string) *AISettingsService {
	return &AISettingsService{configPath: configPath}
}

// AISettingsResponse 返回给前端的 AI 配置（API Key 脱敏）
type AISettingsResponse struct {
	Provider   string `json:"provider"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`   // 脱敏：只返回前4位+****
	Model      string `json:"model"`
	MaxTokens  int    `json:"max_tokens"`
	TimeoutSec int    `json:"timeout_sec"`
	RateLimit  int    `json:"rate_limit"`
}

// UpdateAISettingsRequest 前端提交的更新请求
type UpdateAISettingsRequest struct {
	Provider   string `json:"provider"    binding:"required"`
	BaseURL    string `json:"base_url"    binding:"required"`
	APIKey     string `json:"api_key"`    // 空字符串表示不修改
	Model      string `json:"model"       binding:"required"`
	MaxTokens  int    `json:"max_tokens"`
	TimeoutSec int    `json:"timeout_sec"`
	RateLimit  int    `json:"rate_limit"`
}

// GetAISettings 读取当前 AI 配置
func (s *AISettingsService) GetAISettings() (*AISettingsResponse, error) {
	cfg, err := configs.Load(s.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &AISettingsResponse{
		Provider:   cfg.AI.Provider,
		BaseURL:    cfg.AI.BaseURL,
		APIKey:     maskAPIKey(cfg.AI.APIKey),
		Model:      cfg.AI.Model,
		MaxTokens:  cfg.AI.MaxTokens,
		TimeoutSec: cfg.AI.TimeoutSec,
		RateLimit:  cfg.AI.RateLimit,
	}, nil
}

// UpdateAISettings 更新 AI 配置并写回 config.yaml
func (s *AISettingsService) UpdateAISettings(req UpdateAISettingsRequest) error {
	// 1. 读取完整的 yaml 文件（保留其他配置不变）
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 2. 解析为通用 map，保留所有字段
	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return fmt.Errorf("failed to parse config yaml: %w", err)
	}

	// 3. 取出现有 ai 节点
	aiSection, _ := rawCfg["ai"].(map[string]interface{})
	if aiSection == nil {
		aiSection = map[string]interface{}{}
	}

	// 4. 只更新允许修改的字段
	aiSection["provider"] = req.Provider
	aiSection["base_url"] = req.BaseURL
	aiSection["model"] = req.Model

	// API Key：空字符串表示不修改，保留原有值
	if req.APIKey != "" && !isMasked(req.APIKey) {
		aiSection["api_key"] = req.APIKey
	}

	if req.MaxTokens > 0 {
		aiSection["max_tokens"] = req.MaxTokens
	}
	if req.TimeoutSec > 0 {
		aiSection["timeout_sec"] = req.TimeoutSec
	}
	if req.RateLimit > 0 {
		aiSection["rate_limit"] = req.RateLimit
	}

	rawCfg["ai"] = aiSection

	// 5. 序列化回 yaml
	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 6. 原子写入（先写临时文件，再 rename，防止写到一半崩溃）
	tmpPath := s.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	if err := os.Rename(tmpPath, s.configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

// maskAPIKey 脱敏 API Key，只显示前4位
func maskAPIKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-4)
}

// isMasked 判断前端传回的 key 是否是脱敏值（含 *），是则不更新
func isMasked(key string) bool {
	return strings.Contains(key, "*")
}
