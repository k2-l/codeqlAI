package queue

import (
	"codeqlAI/internal/analyzer"
	"codeqlAI/internal/auditor"
	"codeqlAI/internal/model"
	"codeqlAI/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Processor 任务处理器，持有所有依赖
type Processor struct {
	db          *gorm.DB
	executor    *analyzer.Executor
	auditor     *auditor.Auditor
	ruleService ruleServiceIface
}

// ruleServiceIface 用接口解耦，避免循环依赖
type ruleServiceIface interface {
	GetRuleFilePath(id uuid.UUID) (string, error)
}

// NewProcessor 初始化任务处理器
func NewProcessor(db *gorm.DB, executor *analyzer.Executor, auditor *auditor.Auditor, ruleService ruleServiceIface) *Processor {
	return &Processor{
		db:          db,
		executor:    executor,
		auditor:     auditor,
		ruleService: ruleService,
	}
}

// HandleCodeQLScan 处理 CodeQL 扫描任务
func (p *Processor) HandleCodeQLScan(ctx context.Context, t *asynq.Task) error {
	var payload CodeQLScanPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	taskID, err := uuid.Parse(payload.TaskID)
	if err != nil {
		return fmt.Errorf("invalid task_id: %w", err)
	}

	log := logger.With(zap.String("task_id", payload.TaskID))
	log.Info("worker picked up CodeQL scan task")

	// 任务存在性检查：可能在排队期间已被用户删除
	var existingTask model.Task
	if err := p.db.First(&existingTask, "id = ?", taskID).Error; err != nil {
		log.Warn("task no longer exists in database, skipping", zap.String("task_id", payload.TaskID))
		return nil // 返回 nil 不触发重试
	}

	// 确保 storage 目录存在
	storageDir := fmt.Sprintf("storage/%s", payload.TaskID)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return p.failTask(taskID, fmt.Errorf("failed to create storage dir: %w", err))
	}

	// 确定源码目录：本地路径直接用，Git 来源则先克隆
	sourceRoot := payload.SourcePath
	if payload.IsGitSource() {
		p.updateTaskStatus(taskID, model.TaskStatusCloning, "")
		log.Info("step 0/3: cloning git repository", zap.String("url", payload.GitURL))

		cloneDir := fmt.Sprintf("%s/source", storageDir)
		err := analyzer.CloneRepository(analyzer.GitCloneOptions{
			URL:      payload.GitURL,
			Branch:   payload.GitBranch,
			Token:    payload.GitToken,
			SSHKey:   payload.GitSSHKey,
			DestPath: cloneDir,
		})
		if err != nil {
			return p.failTask(taskID, fmt.Errorf("git clone failed: %w", err))
		}
		sourceRoot = cloneDir
		log.Info("repository cloned ✓", zap.String("dest", cloneDir))
	}

	// Step 1: Building
	p.updateTaskStatus(taskID, model.TaskStatusBuilding, "")
	log.Info("step 1/3: creating CodeQL database")

	if err := p.executor.CreateDatabase(payload.TaskID, payload.Language, sourceRoot); err != nil {
		return p.failTask(taskID, fmt.Errorf("CreateDatabase failed: %w", err))
	}
	p.db.Model(&model.Task{}).Where("id = ?", taskID).
		Update("codeql_db_path", p.executor.DBPath(payload.TaskID))
	log.Info("CodeQL database created ✓")

	// Step 2: Analyzing
	p.updateTaskStatus(taskID, model.TaskStatusAnalyzing, "")
	log.Info("step 2/3: running CodeQL analysis")

	// 解析自定义规则路径（留空则使用官方套件）
	customQLPath := ""
	if payload.CustomRuleID != "" {
		ruleID, err := uuid.Parse(payload.CustomRuleID)
		if err != nil {
			return p.failTask(taskID, fmt.Errorf("invalid custom_rule_id: %w", err))
		}
		customQLPath, err = p.ruleService.GetRuleFilePath(ruleID)
		if err != nil {
			return p.failTask(taskID, fmt.Errorf("failed to get custom rule file: %w", err))
		}
	}

	if err := p.executor.RunAnalysis(payload.TaskID, payload.Language, customQLPath); err != nil {
		return p.failTask(taskID, fmt.Errorf("RunAnalysis failed: %w", err))
	}
	sarifPath := p.executor.SarifPath(payload.TaskID)
	p.db.Model(&model.Task{}).Where("id = ?", taskID).Update("sarif_path", sarifPath)
	log.Info("CodeQL analysis completed ✓")

	// Step 3: 解析 SARIF 写入数据库
	log.Info("step 3/3: parsing SARIF")
	findings, err := analyzer.ParseSarif(taskID, sarifPath, sourceRoot)
	if err != nil {
		return p.failTask(taskID, fmt.Errorf("ParseSarif failed: %w", err))
	}

	if len(findings) > 0 {
		if err := p.db.Create(&findings).Error; err != nil {
			return p.failTask(taskID, fmt.Errorf("failed to save findings: %w", err))
		}
	}

	// 完成
	finishedAt := time.Now()
	p.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":      model.TaskStatusCompleted,
		"finished_at": finishedAt,
	})

	log.Info("scan task completed", zap.Int("findings_count", len(findings)))
	return nil
}

// HandleAIAudit 处理 AI 审计任务
func (p *Processor) HandleAIAudit(ctx context.Context, t *asynq.Task) error {
	var payload AIAuditPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	findingID, err := uuid.Parse(payload.FindingID)
	if err != nil {
		return fmt.Errorf("invalid finding_id: %w", err)
	}

	logger.Info("worker picked up AI audit task",
		zap.String("finding_id", payload.FindingID),
	)

	_, err = p.auditor.AuditFinding(findingID)
	return err
}

// updateTaskStatus 更新任务状态
func (p *Processor) updateTaskStatus(taskID uuid.UUID, status model.TaskStatus, errLog string) {
	updates := map[string]interface{}{"status": status}
	if errLog != "" {
		updates["error_log"] = errLog
	}
	p.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(updates)
}

// failTask 标记任务失败并返回错误
func (p *Processor) failTask(taskID uuid.UUID, err error) error {
	p.updateTaskStatus(taskID, model.TaskStatusFailed, err.Error())
	logger.Error("task failed", zap.String("task_id", taskID.String()), zap.Error(err))
	return err
}
