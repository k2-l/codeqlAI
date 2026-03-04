package auditor

import (
	"bytes"
	"codeqlAI/internal/model"
	"sync"
	"text/template"
)

// promptData 注入 Prompt 模板的数据结构
type promptData struct {
	RuleID      string
	Severity    string
	FilePath    string
	Line        int
	CodeSnippet string
}

// systemPrompt AI 角色定义，固定不变
const systemPrompt = `你是一个专业的渗透测试专家和代码安全审计员。
你的任务是对 CodeQL 静态分析发现的潜在漏洞进行二次验证。
你需要仔细分析代码的数据流，判断漏洞在实际环境中是否真的可以被触发和利用。
你必须严格按照指定的 JSON 格式输出，不得包含任何额外的文字、注释或 markdown 代码块。`

// userPromptTemplate 用户侧 Prompt 模板
const userPromptTemplate = `CodeQL 静态分析发现了一个潜在漏洞，请进行深度审计：

---
漏洞规则: {{.RuleID}}
严重程度: {{.Severity}}
受影响文件: {{.FilePath}} (第 {{.Line}} 行)
源码上下文 (>>> 标记为漏洞核心行):
` + "```" + `
{{.CodeSnippet}}
` + "```" + `
---

审计任务：
1. 仔细分析源码数据流，判断该漏洞在实际环境中是否可以被触发
2. 给出明确的可利用性判断
3. 如果可利用，生成对应的 PoC：
   - Web 漏洞：优先生成原始 HTTP 请求包或 curl 命令
   - 系统类漏洞：提供 Python 验证脚本

严格以如下 JSON 格式输出，不要包含任何其他内容：
{
  "is_exploitable": true或false,
  "analysis_logic": "详细的分析推理过程，说明数据流路径和触发条件",
  "poc_type": "http_request 或 python_script 或 curl 或 not_applicable",
  "poc_content": "具体的 PoC 内容，如果不可利用则填写 N/A",
  "confidence": 0.0到1.0之间的小数
}`

// 预编译模板，避免每次调用时重复解析
var (
	userPromptTmpl     *template.Template
	userPromptTmplOnce sync.Once
	tmplParseErr       error
)

// getUserPromptTmpl 获取预编译的模板（延迟初始化）
func getUserPromptTmpl() (*template.Template, error) {
	userPromptTmplOnce.Do(func() {
		userPromptTmpl, tmplParseErr = template.New("audit").Parse(userPromptTemplate)
	})
	return userPromptTmpl, tmplParseErr
}

// BuildUserPrompt 根据 Finding 渲染用户侧 Prompt
func BuildUserPrompt(finding model.Finding) (string, error) {
	tmpl, err := getUserPromptTmpl()
	if err != nil {
		return "", err
	}

	data := promptData{
		RuleID:      finding.RuleID,
		Severity:    string(finding.Severity),
		FilePath:    finding.FilePath,
		Line:        finding.StartLine,
		CodeSnippet: finding.CodeSnippet,
	}

	// 预估输出长度以优化 buffer 扩容
	estimatedLen := len(userPromptTemplate) + len(finding.CodeSnippet) + 100
	var buf bytes.Buffer
	buf.Grow(estimatedLen)

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetSystemPrompt 返回系统侧 Prompt
func GetSystemPrompt() string {
	return systemPrompt
}