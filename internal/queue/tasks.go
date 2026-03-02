package queue

import (
	"encoding/json"
	"fmt"
)

const (
	TypeCodeQLScan = "codeql:scan"
	TypeAIAudit    = "ai:audit"
)

// CodeQLScanPayload 扫描任务完整参数
type CodeQLScanPayload struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
	Language  string `json:"language"`

	// 本地路径（与 Git 字段二选一）
	SourcePath string `json:"source_path"`

	// Git 来源
	GitURL    string `json:"git_url"`
	GitBranch string `json:"git_branch"`
	GitToken  string `json:"git_token"`
	GitSSHKey string `json:"git_ssh_key"`
}

// IsGitSource 判断是否为 Git 来源
func (p *CodeQLScanPayload) IsGitSource() bool {
	return p.GitURL != ""
}

type AIAuditPayload struct {
	FindingID string `json:"finding_id"`
}

func NewCodeQLScanTask(payload CodeQLScanPayload) (string, []byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal CodeQLScanPayload: %w", err)
	}
	return TypeCodeQLScan, data, nil
}

func NewAIAuditTask(payload AIAuditPayload) (string, []byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal AIAuditPayload: %w", err)
	}
	return TypeAIAudit, data, nil
}