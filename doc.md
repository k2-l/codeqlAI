### 项目愿景
构建一个基于 Go 语言的自动化安全分析平台，集成 CodeQL 静态分析能力，并利用 LLM（大语言模型）对扫描结果进行二次审计和分析验证漏洞可用性生成。

### 技术栈定义 (Baseline)
- 后端语言: Go 1.21+
- Web 框架: Gin (高性能、轻量)
- 任务队列: Asynq (基于 Redis 的异步任务处理)
- 数据库: PostgreSQL (存储扫描记录与结果) / Redis (队列与缓存)
- 核心引擎: CodeQL CLI (需系统预装)
- AI 集成：Go-OpenAI SDK (支持 GPT-4 / Claude / DeepSeek)
- 前端：Vue3 + TypeScript

### 项目目录结构
```
codeqlAI/
├── cmd/
│   └── server/             # 程序入口 (main.go)
├── internal/               # 私有业务逻辑 (核心代码)
│   ├── analyzer/           # CodeQL 核心驱动 (Phase 1 & 3)
│   │   ├── executor.go     # 调用 CLI 执行命令
│   │   ├── git.go          # git 命令调用
│   │   └── parser.go       # SARIF 结果解析器
│   ├── auditor/            # AI 审计模块 (Phase 4)
│   │   ├── client.go       # OpenAI/Claude SDK 封装
│   │   └── auditor.go      # Prompt 模板管理
│   ├── queue/              # Asynq 任务调度逻辑 (Phase 2)
│   │   ├── processor.go    # 任务处理器 (Worker)
│   │   └── tasks.go        # 定义任务 Payload
│   ├── model/              # GORM 数据库模型定义
│   ├── database/           # 数据库操作
│   ├── api/                # Gin 路由与控制器 (Phase 4)
│   │   └── v1/             # API 版本控制
│   └── service/            # 业务逻辑层 (串联扫描、AI、数据库)
│   │   └── scan_service.go # 扫描逻辑
├── pkg/                    # 可导出的公共库 (如日志、工具类)
│   └── logger/             # 日志模块
├── scripts/                # 脚本 (如 CodeQL 安装、数据库迁移)
├── storage/                # 临时目录 (存放源码、CodeQL DB、SARIF)
├── configs/                # 配置文件 (YAML/Env)
├── go.mod                  # 项目依赖
└── docker-compose.yml      # 快速启动 Redis 和 PostgreSQL

```

### Phase 顺序
#### Phase 0(基础设施搭建)
- 设计并最终确定所有数据库 Schema
- 配置 docker-compose.yml 把 PostgreSQL 和 Redis 跑起来
- 初始化 Go 项目结构、引入 GORM 做数据库迁移
- 完成基础配置加载（读取 YAML/环境变量）
- 写好全局日志组件（pkg/logger）
#### Phase 1(核心引擎封装)
- 实现 executor.go，封装 database create 和 database analyze 的 CLI 调用
- 做好命令注入防护，Language 字段走白名单映射而不是直接拼字符串
- 实现 parser.go，解析 SARIF 并提取代码片段（向前后各扩展若干行）
- 直接将解析结果写入数据库
- 写单元测试验证解析逻辑
#### Phase 2(AI审计原子接口)
- 封装OpenAI/Claude SDK客户端
- 实现Prompt模板管理
- 实现速率限制(令牌桶，定义好并发上限)
- 单独用一个 Finding 测试 AI 审计全流程，验证 JSON 输出格式正确
- 把AI结果回写到数据库对应记录
#### Phase 3(Gin API + Asynq异步编排)
- 实现 POST /scan、 GET /task/:id、 GET /task/:id/results 接口
- 加上简单 API 鉴权(例如简单的API Key)
- 用 Asynq 把 CodeQL 和 AI 审计编排成异步任务链
- 实现状态流转和 Task Log 实时写入
- 加上超时 Kill 和重试策略
#### Phase 4(可靠性与安全加固)
- Git 拉取的 SSRF 防护
- 存储空间的生命周期管理（ZIP、克隆目录、临时 DB 的清理策略）
- 错误重试的退避逻辑细化
- 压测并发场景下的稳定性
#### Phase 5(前端可视化)
- 登录页
- 导航侧边栏
- 控制中台主页
- 用户详细信息页
- 系统设置页


### 详细功能模块

#### 1、任务管理模块
- 代码获取:
- 支持本地 ZIP 文件上传
- 支持 Git Pull 拉取
- 异步调度:
- 使用 Asynq 分配 CreateDB、Analyze、AI_Audit 三个阶段任务。
- 状态流转：Pending -> Cloning -> Building -> Analyzing -> AI_Reviewing -> Completed/Failed
#### 2、CodeQL 核心驱动 (Analyzer)
- 环境检查: 启动时自动检测 codeql 二进制文件是否在系统 PATH 中。
- 数据库创建: codeql database create [db_path] --language=[lang] --source-root=[code_path]。
- 规则库管理:
- 内置官方默认查询包（Standard Queries）。
- 支持自定义 .ql 文件上传或目录挂载。
- 结果解析:
- 运行查询生成 result.sarif (JSON 格式)。
- 编写 Go Parser 提取关键信息：RuleID, Severity, Message, File Path, Start Line, Code Snippet。
#### 3、AI 辅助审计模块 (AI Auditor)
- Context 注入: 提取 SARIF 结果中的代码片段(关键点)
- 因为 AI 需要代码片段，而 SARIF 文件本身通常不包含完整的源码片段，只包含行列号。
- 难点实现：解析器需要根据 SARIF 提供的 physicalLocation，去原始代码文件中读取对应的行，并向前后各扩展 10-20 行作为 AI 的上下文。
- Prompt 模板:
```你是一个专业的渗透测试专家。CodeQL 静态分析发现了一个潜在漏洞：
---
漏洞规则: {{.RuleID}}
严重程度: {{.Severity}}
受影响文件: {{.FilePath}} (第 {{.Line}} 行)
源码上下文: {{.CodeSnippet}}
---
任务要求：
1. 验证逻辑：仔细分析源码数据流，判断该漏洞在实际环境中是否可以被触发。
2. 漏洞可行性：给出明确的 [Yes/No] 判断。
 1. 生成 PoC：
   - 如果是 Web 漏洞，优先生成 Nuclei YAML 模板或原始 HTTP 请求包。
   - 如果是系统类漏洞，提供 Python 验证脚本或 cURL 命令。
4. 严格以 JSON 格式输出，包含字段：is_exploitable, analysis_logic, poc_type, poc_content, confidence。 
```
- 限流处理: 实现速率限制，防止大量漏洞同时触发时耗尽 AI Token。
- AI输出 模板：
```--------------------------------------------------
[审计结论]: %t // true或flase
[漏洞类型]: %s 
[可信度评分]: %.2f // 0.0~1.0
[POC 框架]: %s // 
--------------------------------------------------
[AI 分析]: %s
--------------------------------------------------
[自动化验证 PoC]: %s
```
#### 4、数据存储与 API (Backend)
- Schema 设计:
- Projects: 项目名称、源码路径。
- Tasks: 关联项目、语言、当前状态、Log 记录。
- Vulnerabilities: 关联任务、CodeQL 原始结果、AI 审计结论、是否忽略。
- 接口设计:
- POST /api/v1/scan: 提交扫描任务。
- GET /api/v1/task/:id: 获取进度。
- GET /api/v1/task/:id/results: 获取发现的漏洞列表。
#### 5、核心实体模型 (Go Structs)
```type ScanTask struct {
 ID string `json:"id"`
 ProjectName string `json:"project_name"`
 Language string `json:"language"` // java, go, python, javascript
 Status string `json:"status"` // running, success, failed
 CreatedAt time.Time `json:"created_at"`
}

type Finding struct {
 ID int `json:"id"`
 TaskID string `json:"task_id"`
 RuleID string `json:"rule_id"`
 Severity string `json:"severity"`
 FilePath string `json:"file_path"`
 Line int `json:"line"`
 CodeSnippet string `json:"code_snippet"`
 AiAnalysis AiResult `json:"ai_verification"` // AI 分析结果结构化
}

type AiResult struct {
 IsExploitable bool `json:"is_exploitable"` // AI 判断是否可利用
 AnalysisLogic string `json:"analysis_logic"` // AI 的分析逻辑/证明过程
 PocType string `json:"poc_type"` // HTTP_Request, Nuclei_Yaml, Python_Script, cURL
 PocContent string `json:"poc_content"` // 具体的 PoC 代码或模板内容
 Confidence float64 `json:"confidence"` // AI 置信度 (0.0 - 1.0)
```