package auditor

import (
	"codeqlAI/internal/model"
	"codeqlAI/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Auditor 核心审计器，串联 AI 客户端、Prompt 模板和数据库写入
type Auditor struct {
	client *Client
	db     *gorm.DB
}

// NewAuditor 初始化审计器
func NewAuditor(client *Client, db *gorm.DB) *Auditor {
	return &Auditor{
		client: client,
		db:     db,
	}
}

// aiResponse AI 返回的 JSON 结构
type aiResponse struct {
	IsExploitable bool    `json:"is_exploitable"`
	AnalysisLogic string  `json:"analysis_logic"`
	PocType       string  `json:"poc_type"`
	PocContent    string  `json:"poc_content"`
	Confidence    float64 `json:"confidence"`
}

// AuditFinding 对单条 Finding 执行 AI 审计
// 这是原子级操作：一次调用只处理一条 Finding
func (a *Auditor) AuditFinding(findingID uuid.UUID) (*model.AiResult, error) {
	// 1. 从数据库读取 Finding
	var finding model.Finding
	if err := a.db.First(&finding, "id = ?", findingID).Error; err != nil {
		return nil, fmt.Errorf("finding not found: %w", err)
	}

	// 2. 检查是否已经审计过，避免重复消耗 token
	var existing model.AiResult
	if err := a.db.First(&existing, "finding_id = ?", findingID).Error; err == nil {
		logger.Warn("finding already audited, skipping",
			zap.String("finding_id", findingID.String()),
		)
		return &existing, nil
	}

	// 3. 更新 Finding 状态为 processing
	a.db.Model(&model.Finding{}).Where("id = ?", findingID).
		Update("audit_status", model.AuditStatusProcessing)

	logger.Info("starting AI audit",
		zap.String("finding_id", findingID.String()),
		zap.String("rule_id", finding.RuleID),
		zap.String("file", finding.FilePath),
		zap.Int("line", finding.StartLine),
	)

	// 4. 构建 Prompt
	userPrompt, err := BuildUserPrompt(finding)
	if err != nil {
		a.markFailed(findingID)
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// 5. 调用 AI（带超时控制）
	timeout := time.Duration(a.client.timeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rawResponse, promptTokens, completionTokens, err := a.client.Chat(ctx, GetSystemPrompt(), userPrompt)
	if err != nil {
		a.markFailed(findingID)
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	logger.Info("AI response received",
		zap.String("finding_id", findingID.String()),
		zap.Int("prompt_tokens", promptTokens),
		zap.Int("completion_tokens", completionTokens),
	)

	// 6. 解析 AI 返回的 JSON
	aiResp, err := parseAIResponse(rawResponse)
	if err != nil {
		a.markFailed(findingID)
		return nil, fmt.Errorf("failed to parse AI response: %w\nRaw: %s", err, rawResponse)
	}

	// 7. 构建 AiResult 并写入数据库
	result := model.AiResult{
		ID:               uuid.New(),
		FindingID:        findingID,
		IsExploitable:    aiResp.IsExploitable,
		AnalysisLogic:    aiResp.AnalysisLogic,
		PocType:          model.PocType(aiResp.PocType),
		PocContent:       aiResp.PocContent,
		Confidence:       aiResp.Confidence,
		ModelUsed:        a.client.model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}

	if err := a.db.Create(&result).Error; err != nil {
		a.markFailed(findingID)
		return nil, fmt.Errorf("failed to save ai_result: %w", err)
	}

	// 8. 更新 Finding 状态为 completed
	a.db.Model(&model.Finding{}).Where("id = ?", findingID).
		Update("audit_status", model.AuditStatusCompleted)

	logger.Info("AI audit completed",
		zap.String("finding_id", findingID.String()),
		zap.Bool("is_exploitable", result.IsExploitable),
		zap.Float64("confidence", result.Confidence),
	)

	// 9. 打印审计结论
	printAuditResult(finding, result)

	return &result, nil
}

// parseAIResponse 解析 AI 返回的 JSON，兼容带 markdown 代码块的情况
func parseAIResponse(raw string) (*aiResponse, error) {
	// 清理 AI 可能返回的 markdown 代码块包裹
	cleaned := strings.TrimSpace(raw)

	// 移除开头的 ```json 或 ```
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
	}

	// 移除结尾的 ```
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var resp aiResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	// 置信度范围校验
	if resp.Confidence < 0 {
		resp.Confidence = 0
	}
	if resp.Confidence > 1 {
		resp.Confidence = 1
	}

	return &resp, nil
}

// markFailed 将 Finding 状态回滚为 pending，方便用户重试
func (a *Auditor) markFailed(findingID uuid.UUID) {
	if err := a.db.Model(&model.Finding{}).Where("id = ?", findingID).
		Update("audit_status", model.AuditStatusPending).Error; err != nil {
		logger.Error("failed to mark finding as failed",
			zap.String("finding_id", findingID.String()),
			zap.Error(err),
		)
	}
}

// printAuditResult 在控制台打印格式化的审计结论
func printAuditResult(finding model.Finding, result model.AiResult) {
	exploitable := "❌ 不可利用"
	if result.IsExploitable {
		exploitable = "✅ 可利用"
	}

	// 预估输出长度，减少内存分配
	estimatedLen := 100 + len(finding.FilePath) + len(finding.RuleID) +
		len(result.AnalysisLogic) + len(result.PocContent)

	var sb strings.Builder
	sb.Grow(estimatedLen)

	sb.WriteString("\n--------------------------------------------------\n")
	sb.WriteString("[Finding] ")
	sb.WriteString(finding.FilePath)
	sb.WriteString(" (line ")
	sb.WriteString(fmt.Sprintf("%d", finding.StartLine))
	sb.WriteString(")\n[Rule]    ")
	sb.WriteString(finding.RuleID)
	sb.WriteString("\n[审计结论] ")
	sb.WriteString(exploitable)
	sb.WriteString("\n[可信度]  ")
	sb.WriteString(fmt.Sprintf("%.2f", result.Confidence))
	sb.WriteString("\n[PoC类型] ")
	sb.WriteString(string(result.PocType))
	sb.WriteString("\n--------------------------------------------------\n[AI 分析]\n")
	sb.WriteString(result.AnalysisLogic)
	sb.WriteString("\n--------------------------------------------------\n[PoC 内容]\n")
	sb.WriteString(result.PocContent)
	sb.WriteString("\n--------------------------------------------------\n")

	fmt.Print(sb.String())
}