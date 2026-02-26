package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SourceType 代码来源类型
type SourceType string

const (
	SourceTypeZip SourceType = "zip"
	SourceTypeGit SourceType = "git"
)

// Language 支持的扫描语言白名单
type Language string

const (
	LanguageJava       Language = "java"
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageCPP        Language = "cpp"
)

// TaskStatus 任务状态流转
type TaskStatus string

const (
	TaskStatusPending     TaskStatus = "pending"
	TaskStatusCloning     TaskStatus = "cloning"
	TaskStatusBuilding    TaskStatus = "building"
	TaskStatusAnalyzing   TaskStatus = "analyzing"
	TaskStatusAIReviewing TaskStatus = "ai_reviewing"
	TaskStatusCompleted   TaskStatus = "completed"
	TaskStatusFailed      TaskStatus = "failed"
)

// Severity 漏洞严重程度
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityNote     Severity = "note"
)

// AuditStatus AI 审计状态
type AuditStatus string

const (
	AuditStatusPending    AuditStatus = "pending"
	AuditStatusProcessing AuditStatus = "processing"
	AuditStatusCompleted  AuditStatus = "completed"
	AuditStatusSkipped    AuditStatus = "skipped"
)

// PocType AI 生成的 PoC 类型
type PocType string

const (
	PocTypeNucleiYAML    PocType = "nuclei_yaml"
	PocTypeHTTPRequest   PocType = "http_request"
	PocTypePythonScript  PocType = "python_script"
	PocTypeCurl          PocType = "curl"
	PocTypeNotApplicable PocType = "not_applicable"
)

// ========== 表1: projects ==========

// Project 项目表，对应一个代码仓库
type Project struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"          json:"id"`
	Name       string     `gorm:"type:varchar(255);not null"    json:"name"`
	SourceType SourceType `gorm:"type:varchar(10);not null"     json:"source_type"`
	SourceURL  string     `gorm:"type:varchar(1024)"            json:"source_url"` // Git URL 或上传文件的存储路径
	CreatedAt  time.Time  `gorm:"autoCreateTime"                json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime"                json:"updated_at"`

	// 关联
	Tasks []Task `gorm:"foreignKey:ProjectID" json:"tasks,omitempty"`
}

func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ========== 表2: tasks ==========

// Task 扫描任务表，一个 Project 可以有多次扫描
type Task struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"              json:"id"`
	ProjectID    uuid.UUID  `gorm:"type:uuid;not null;index"          json:"project_id"`
	Language     Language   `gorm:"type:varchar(20);not null"         json:"language"`
	Status       TaskStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	ErrorLog     string     `gorm:"type:text"                         json:"error_log,omitempty"`  // 失败时记录原因
	CodeqlDBPath string     `gorm:"type:varchar(1024)"                json:"codeql_db_path"`       // 扫描完成后会清空
	SarifPath    string     `gorm:"type:varchar(1024)"                json:"sarif_path"`
	StartedAt    *time.Time `gorm:"default:null"                      json:"started_at,omitempty"`
	FinishedAt   *time.Time `gorm:"default:null"                      json:"finished_at,omitempty"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"                    json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"                    json:"updated_at"`

	// 关联
	Project  Project   `gorm:"foreignKey:ProjectID"  json:"project,omitempty"`
	Findings []Finding `gorm:"foreignKey:TaskID"     json:"findings,omitempty"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// ========== 表3: findings ==========

// Finding SARIF 解析出的单个漏洞点
type Finding struct {
	ID          uuid.UUID   `gorm:"type:uuid;primaryKey"                        json:"id"`
	TaskID      uuid.UUID   `gorm:"type:uuid;not null;index"                    json:"task_id"`
	RuleID      string      `gorm:"type:varchar(255);not null"                  json:"rule_id"`      // 如 java/sql-injection
	Severity    Severity    `gorm:"type:varchar(20);not null"                   json:"severity"`
	Message     string      `gorm:"type:text;not null"                          json:"message"`      // CodeQL 原始描述
	FilePath    string      `gorm:"type:varchar(1024);not null"                 json:"file_path"`    // 受影响文件相对路径
	StartLine   int         `gorm:"not null"                                    json:"start_line"`
	EndLine     int         `gorm:"not null"                                    json:"end_line"`
	CodeSnippet string      `gorm:"type:text"                                   json:"code_snippet"` // 前后扩展的代码上下文，供 AI 分析用
	AuditStatus AuditStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"audit_status"`
	IsIgnored   bool        `gorm:"not null;default:false"                      json:"is_ignored"`   // 用户手动标记忽略
	CreatedAt   time.Time   `gorm:"autoCreateTime"                              json:"created_at"`
	UpdatedAt   time.Time   `gorm:"autoUpdateTime"                              json:"updated_at"`

	// 关联
	Task     Task      `gorm:"foreignKey:TaskID"     json:"task,omitempty"`
	AiResult *AiResult `gorm:"foreignKey:FindingID"  json:"ai_result,omitempty"` // 指针：没审计过则为 nil
}

func (f *Finding) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// ========== 表4: ai_results ==========

// AiResult AI 审计结论表，与 Finding 一对一
type AiResult struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"                     json:"id"`
	FindingID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"           json:"finding_id"` // 唯一索引保证一对一
	IsExploitable  bool      `gorm:"not null"                                 json:"is_exploitable"`
	AnalysisLogic  string    `gorm:"type:text"                                json:"analysis_logic"`  // AI 的分析推理过程
	PocType        PocType   `gorm:"type:varchar(30)"                         json:"poc_type"`
	PocContent     string    `gorm:"type:text"                                json:"poc_content"`     // 具体 PoC 内容
	Confidence     float64   `gorm:"type:numeric(4,2)"                        json:"confidence"`      // 0.00 ~ 1.00
	ModelUsed      string    `gorm:"type:varchar(100)"                        json:"model_used"`      // 记录用了哪个模型，方便后期对比
	PromptTokens   int       `gorm:"default:0"                                json:"prompt_tokens"`   // 消耗的 token 数
	CompletionTokens int     `gorm:"default:0"                                json:"completion_tokens"`
	CreatedAt      time.Time `gorm:"autoCreateTime"                           json:"created_at"`

	// 关联
	Finding Finding `gorm:"foreignKey:FindingID" json:"finding,omitempty"`
}

func (a *AiResult) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}