// ===== 枚举 =====
export type TaskStatus =
  | 'pending' | 'cloning' | 'building'
  | 'analyzing' | 'ai_reviewing' | 'completed' | 'failed'

export type Language = 'java' | 'go' | 'python' | 'javascript' | 'cpp'
export type Severity  = 'critical' | 'high' | 'medium' | 'low' | 'note'
export type AuditStatus = 'pending' | 'processing' | 'completed' | 'skipped'
export type PocType = 'http_request' | 'python_script' | 'curl' | 'not_applicable'
export type SourceType = 'zip' | 'git'

// ===== 实体 =====
export interface Project {
  id:          string
  name:        string
  source_type: SourceType
  source_url:  string
  created_at:  string
  updated_at:  string
}

export interface Task {
  id:             string
  project_id:     string
  display_name:   string
  language:       Language
  status:         TaskStatus
  error_log?:     string
  codeql_db_path: string
  sarif_path:     string
  started_at?:    string
  finished_at?:   string
  created_at:     string
  updated_at:     string
  project?:       Project
}

export interface AiResult {
  id:                string
  finding_id:        string
  is_exploitable:    boolean
  analysis_logic:    string
  poc_type:          PocType
  poc_content:       string
  confidence:        number
  model_used:        string
  prompt_tokens:     number
  completion_tokens: number
  created_at:        string
}

export interface Finding {
  id:           string
  task_id:      string
  rule_id:      string
  severity:     Severity
  message:      string
  file_path:    string
  start_line:   number
  end_line:     number
  code_snippet: string
  audit_status: AuditStatus
  is_ignored:   boolean
  created_at:   string
  updated_at:   string
  ai_result?:   AiResult
}

// ===== API 请求/响应 =====
export interface SubmitScanRequest {
  project_name: string
  task_name?:   string
  language:     Language
  source_path?: string
  git_url?:     string
  git_branch?:  string
  git_token?:   string
  git_ssh_key?: string
}

export interface SubmitScanResponse {
  message:      string
  task_id:      string
  display_name: string
  status:       TaskStatus
}

export interface FindingsResponse {
  task_id: string
  total:   number
  items:   Finding[]
}

// ===== 仪表盘统计 =====
export interface DashboardStats {
  total_tasks:     number
  completed_tasks: number
  failed_tasks:    number
  running_tasks:   number
  total_findings:  number
  high_findings:   number
  audited_findings: number
  exploitable_findings: number
}
