package auditor

import (
	"codeqlAI/configs"
	"codeqlAI/pkg/logger"
	"context"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// Client 封装兼容 OpenAI 格式的 AI 接口调用
type Client struct {
	client     *openai.Client
	model      string
	maxTokens  int
	timeoutSec int
}

// NewClient 初始化 AI 客户端，支持自定义 BaseURL（兼容三方接口）
func NewClient(cfg configs.AIConfig) (*Client, error) {
	if cfg.APIKey == "" || cfg.BaseURL == "" {
		return nil, errors.New("AI api_key or base_url is empty, please check config.yaml")
	}

	// 使用自定义 BaseURL 替换默认的 OpenAI 地址
	clientCfg := openai.DefaultConfig(cfg.APIKey)
	clientCfg.BaseURL = cfg.BaseURL

	client := openai.NewClientWithConfig(clientCfg)

	logger.Info("AI client initialized",
		zap.String("provider", cfg.Provider),
		zap.String("base_url", cfg.BaseURL),
		zap.String("model", cfg.Model),
	)

	return &Client{
		client:     client,
		model:      cfg.Model,
		maxTokens:  cfg.MaxTokens,
		timeoutSec: cfg.TimeoutSec,
	}, nil
}

// Chat 发送单次对话请求，返回 AI 的文本回复
func (c *Client) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, int, int, error) {
	// 预分配 messages 容量
	messages := make([]openai.ChatCompletionMessage, 0, 2)
	messages = append(messages,
		openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userPrompt,
		},
	)

	req := openai.ChatCompletionRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages:  messages,
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("AI request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", 0, 0, errors.New("AI returned empty choices")
	}

	content := resp.Choices[0].Message.Content
	promptTokens := resp.Usage.PromptTokens
	completionTokens := resp.Usage.CompletionTokens

	return content, promptTokens, completionTokens, nil
}