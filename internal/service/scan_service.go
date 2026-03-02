package service

import (
	"codeqlAI/internal/analyzer"
	"codeqlAI/internal/model"
	"codeqlAI/internal/queue"
	"codeqlAI/pkg/logger"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ScanService 扫描业务逻辑层
type ScanService struct {
	db          *gorm.DB
	asynqClient *asynq.Client
	inspector   *asynq.Inspector
}

// NewScanService 初始化扫描服务
func NewScanService(db *gorm.DB, asynqClient *asynq.Client, inspector *asynq.Inspector) *ScanService {
	return &ScanService{
		db:          db,
		asynqClient: asynqClient,
		inspector:   inspector,
	}
}

// SubmitScanRequest 提交扫描任务的请求参数
type SubmitScanRequest struct {
	ProjectName string `json:"project_name" binding:"required"`
	TaskName    string `json:"task_name"`
	Language    string `json:"language" binding:"required"`

	// 本地路径（与 GitURL 二选一）
	SourcePath string `json:"source_path"`

	// Git 来源
	GitURL    string `json:"git_url"`
	GitBranch string `json:"git_branch"` // 留空则克隆默认分支
	GitToken  string `json:"git_token"`  // 私有仓库 HTTPS Token
	GitSSHKey string `json:"git_ssh_key"` // 私有仓库 SSH Key 路径
}

// SubmitScan 创建 Project + Task，推入扫描队列
func (s *ScanService) SubmitScan(req SubmitScanRequest) (*model.Task, error) {
	// 语言白名单校验
	allowed := map[string]bool{
		"java": true, "go": true, "python": true,
		"javascript": true, "cpp": true,
	}
	if !allowed[req.Language] {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}

	// 来源校验
	if req.SourcePath == "" && req.GitURL == "" {
		return nil, fmt.Errorf("either source_path or git_url is required")
	}
	if req.SourcePath != "" && req.GitURL != "" {
		return nil, fmt.Errorf("source_path and git_url cannot be set at the same time")
	}

	// Git URL 提前做 SSRF 校验，不合法直接拒绝，不进队列
	if req.GitURL != "" {
		if err := analyzer.ValidateGitURL(req.GitURL); err != nil {
			return nil, fmt.Errorf("invalid git_url: %w", err)
		}
	}

	// 任务名自动生成
	displayName := req.TaskName
	if displayName == "" {
		displayName = fmt.Sprintf("%s-%s-%s", req.ProjectName, req.Language, time.Now().Format("20060102-150405"))
	}

	// 确定来源类型
	sourceType := model.SourceTypeZip
	sourceURL := req.SourcePath
	if req.GitURL != "" {
		sourceType = model.SourceTypeGit
		sourceURL = req.GitURL
	}

	// 创建 Project
	project := model.Project{
		Name:       req.ProjectName,
		SourceType: sourceType,
		SourceURL:  sourceURL,
	}
	if err := s.db.Create(&project).Error; err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// 创建 Task
	now := time.Now()
	task := model.Task{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		DisplayName: displayName,
		Language:    model.Language(req.Language),
		Status:      model.TaskStatusPending,
		StartedAt:   &now,
	}
	if err := s.db.Create(&task).Error; err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 推入 Asynq 队列
	taskType, payload, err := queue.NewCodeQLScanTask(queue.CodeQLScanPayload{
		TaskID:     task.ID.String(),
		ProjectID:  project.ID.String(),
		Language:   req.Language,
		SourcePath: req.SourcePath,
		GitURL:     req.GitURL,
		GitBranch:  req.GitBranch,
		GitToken:   req.GitToken,
		GitSSHKey:  req.GitSSHKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create task payload: %w", err)
	}

	if _, err := s.asynqClient.Enqueue(asynq.NewTask(taskType, payload)); err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	logger.Info("scan task submitted",
		zap.String("task_id", task.ID.String()),
		zap.String("display_name", displayName),
		zap.String("source_type", string(sourceType)),
	)

	return &task, nil
}

// GetTask 按 UUID 查询任务
func (s *ScanService) GetTask(taskID uuid.UUID) (*model.Task, error) {
	var task model.Task
	if err := s.db.Preload("Project").First(&task, "id = ?", taskID).Error; err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	return &task, nil
}

// GetTaskByName 按自定义任务名查询（模糊匹配，返回最近一条）
func (s *ScanService) GetTaskByName(name string) (*model.Task, error) {
	var task model.Task
	if err := s.db.Preload("Project").
		Where("display_name LIKE ?", "%"+name+"%").
		Order("created_at DESC").
		First(&task).Error; err != nil {
		return nil, fmt.Errorf("task not found with name '%s': %w", name, err)
	}
	return &task, nil
}

// GetFindings 查询任务下的所有漏洞
func (s *ScanService) GetFindings(taskID uuid.UUID) ([]model.Finding, error) {
	var findings []model.Finding
	if err := s.db.Preload("AiResult").
		Where("task_id = ?", taskID).
		Order("severity, start_line").
		Find(&findings).Error; err != nil {
		return nil, fmt.Errorf("failed to query findings: %w", err)
	}
	return findings, nil
}

// DeleteTask 彻底删除任务：取消队列任务 + 清理数据库所有关联记录
func (s *ScanService) DeleteTask(taskID uuid.UUID) error {
	// 1. 查询任务是否存在
	var task model.Task
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// 2. 尝试从 Asynq 队列里取消（pending 状态的任务才能取消，运行中的忽略错误）
	// Asynq 用 task_id 格式为 "asynq task id"，需要遍历 pending 队列查找
	pendingTasks, err := s.inspector.ListPendingTasks("default", asynq.PageSize(100))
	if err == nil {
		for _, t := range pendingTasks {
			// 从 payload 里匹配 task_id
			if containsTaskID(t.Payload, taskID.String()) {
				if err := s.inspector.DeleteTask("default", t.ID); err != nil {
					logger.Warn("failed to delete asynq task",
						zap.String("asynq_task_id", t.ID),
						zap.Error(err),
					)
				} else {
					logger.Info("asynq task cancelled",
						zap.String("asynq_task_id", t.ID),
						zap.String("task_id", taskID.String()),
					)
				}
			}
		}
	}

	// 3. 级联删除数据库记录（ai_results -> findings -> tasks -> projects）
	// 先查出所有 finding ID
	var findingIDs []uuid.UUID
	s.db.Model(&model.Finding{}).Where("task_id = ?", taskID).Pluck("id", &findingIDs)

	// 删除 ai_results
	if len(findingIDs) > 0 {
		if err := s.db.Where("finding_id IN ?", findingIDs).Delete(&model.AiResult{}).Error; err != nil {
			return fmt.Errorf("failed to delete ai_results: %w", err)
		}
	}

	// 删除 findings
	if err := s.db.Where("task_id = ?", taskID).Delete(&model.Finding{}).Error; err != nil {
		return fmt.Errorf("failed to delete findings: %w", err)
	}

	// 删除 task
	projectID := task.ProjectID
	if err := s.db.Delete(&model.Task{}, "id = ?", taskID).Error; err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	// 删除 project（如果该 project 下没有其他 task 了）
	var taskCount int64
	s.db.Model(&model.Task{}).Where("project_id = ?", projectID).Count(&taskCount)
	if taskCount == 0 {
		s.db.Delete(&model.Project{}, "id = ?", projectID)
		logger.Info("project deleted (no remaining tasks)",
			zap.String("project_id", projectID.String()),
		)
	}

	logger.Info("task deleted successfully", zap.String("task_id", taskID.String()))
	return nil
}

// containsTaskID 检查 asynq 任务的 payload 里是否包含指定 task_id
func containsTaskID(payload []byte, taskID string) bool {
	return strings.Contains(string(payload), taskID)
}

// TriggerAIAudit 触发单条 Finding 的 AI 审计
func (s *ScanService) TriggerAIAudit(findingID uuid.UUID) error {
	var finding model.Finding
	if err := s.db.First(&finding, "id = ?", findingID).Error; err != nil {
		return fmt.Errorf("finding not found: %w", err)
	}

	if finding.AuditStatus == model.AuditStatusCompleted {
		return fmt.Errorf("finding already audited")
	}

	taskType, payload, err := queue.NewAIAuditTask(queue.AIAuditPayload{
		FindingID: findingID.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to create audit payload: %w", err)
	}

	if _, err := s.asynqClient.Enqueue(asynq.NewTask(taskType, payload)); err != nil {
		return fmt.Errorf("failed to enqueue audit task: %w", err)
	}

	logger.Info("AI audit task enqueued", zap.String("finding_id", findingID.String()))
	return nil
}