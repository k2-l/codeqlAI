package analyzer

import (
	"bufio"
	"codeqlAI/internal/model"
	"codeqlAI/pkg/logger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ===== SARIF 数据结构定义 =====
// 只提取我们需要的字段，忽略其他无关内容

type sarifReport struct {
	Runs []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID                   string             `json:"id"`
	ShortDescription     sarifMessage       `json:"shortDescription"`
	DefaultConfiguration sarifConfiguration `json:"defaultConfiguration"`
}

type sarifConfiguration struct {
	Level string `json:"level"` // error | warning | note
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"` // error | warning | note
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"` // 相对文件路径
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	EndLine     int `json:"endLine"`
	StartColumn int `json:"startColumn"`
}

// ===== codeFlows 扩展结构（用于漏洞地图） =====

// sarifReportFull 扩展版本，包含 codeFlows
type sarifReportFull struct {
	Runs []sarifRunFull `json:"runs"`
}

type sarifRunFull struct {
	Tool    sarifTool         `json:"tool"`
	Results []sarifResultFull `json:"results"`
}

type sarifResultFull struct {
	RuleID    string             `json:"ruleId"`
	Level     string             `json:"level"`
	Message   sarifMessage       `json:"message"`
	Locations []sarifLocation    `json:"locations"`
	CodeFlows []sarifCodeFlow    `json:"codeFlows"`
}

type sarifCodeFlow struct {
	ThreadFlows []sarifThreadFlow `json:"threadFlows"`
}

type sarifThreadFlow struct {
	Locations []sarifThreadFlowLocation `json:"locations"`
}

type sarifThreadFlowLocation struct {
	Location sarifFlowLocation `json:"location"`
}

type sarifFlowLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
	Message          sarifMessage          `json:"message"`
}

// ===== 解析器 =====

// contextLines 代码片段前后各扩展的行数
const contextLines = 15

// ParseSarif 解析 SARIF 文件，提取漏洞点并补充代码片段
// taskID: 关联的任务 ID
// sarifPath: SARIF 文件路径
// sourceRoot: 源代码根目录（用于读取代码片段）
func ParseSarif(taskID uuid.UUID, sarifPath string, sourceRoot string) ([]model.Finding, error) {
	logger.Info("parsing SARIF file",
		zap.String("task_id", taskID.String()),
		zap.String("sarif_path", sarifPath),
	)

	data, err := os.ReadFile(sarifPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sarif file: %w", err)
	}

	var report sarifReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse sarif json: %w", err)
	}

	severityMap := buildSeverityMap(report)

	// 预估 findings 数量以预分配切片容量
	estimatedFindings := 0
	for _, run := range report.Runs {
		estimatedFindings += len(run.Results)
	}
	findings := make([]model.Finding, 0, estimatedFindings)

	for _, run := range report.Runs {
		for _, result := range run.Results {
			if len(result.Locations) == 0 {
				continue
			}

			loc := result.Locations[0].PhysicalLocation
			filePath := loc.ArtifactLocation.URI
			startLine := loc.Region.StartLine
			endLine := loc.Region.EndLine

			if endLine == 0 {
				endLine = startLine
			}

			snippet, err := extractCodeSnippet(
				filepath.Join(sourceRoot, filePath),
				startLine,
				endLine,
				contextLines,
			)
			if err != nil {
				logger.Warn("failed to extract code snippet",
					zap.String("file", filePath),
					zap.Int("line", startLine),
					zap.Error(err),
				)
				snippet = "[code snippet unavailable]"
			}

			findings = append(findings, model.Finding{
				ID:          uuid.New(),
				TaskID:      taskID,
				RuleID:      result.RuleID,
				Severity:    mapSeverity(severityMap[result.RuleID]),
				Message:     result.Message.Text,
				FilePath:    filePath,
				StartLine:   startLine,
				EndLine:     endLine,
				CodeSnippet: snippet,
				AuditStatus: model.AuditStatusPending,
				IsIgnored:   false,
			})
		}
	}

	logger.Info("SARIF parsed successfully",
		zap.String("task_id", taskID.String()),
		zap.Int("findings_count", len(findings)),
	)

	return findings, nil
}

// extractCodeSnippet 从源文件中读取指定行范围，并向前后各扩展 contextLines 行
func extractCodeSnippet(filePath string, startLine, endLine, context int) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("cannot open file %s: %w", filePath, err)
	}
	defer f.Close()

	// 计算实际读取范围
	from := startLine - context
	if from < 1 {
		from = 1
	}
	to := endLine + context

	// 预计算行数，预分配 strings.Builder 容量
	lineCount := to - from + 1
	var builder strings.Builder
	builder.Grow(lineCount * 80) // 假设平均每行约 80 字符

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum < from {
			continue
		}
		if lineNum > to {
			break
		}

		// 标记漏洞核心行，方便 AI 识别重点
		if lineNum >= startLine && lineNum <= endLine {
			builder.WriteString(">>> ")
		} else {
			builder.WriteString("    ")
		}

		// 手动格式化行号，避免 fmt.Sprintf 开销
		writePaddedLineNum(&builder, lineNum)
		builder.WriteString(" | ")
		builder.WriteString(scanner.Text())
		builder.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	result := builder.String()
	// 移除末尾多余的换行符
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// writePaddedLineNum 写入带前导空格填充的行号（4字符宽度）
func writePaddedLineNum(b *strings.Builder, n int) {
	switch {
	case n >= 10000:
		b.WriteString(fmt.Sprintf("%4d", n))
	case n >= 1000:
		b.WriteByte(' ')
		b.WriteString(fmt.Sprintf("%d", n))
	case n >= 100:
		b.WriteString("  ")
		b.WriteString(fmt.Sprintf("%d", n))
	case n >= 10:
		b.WriteString("   ")
		b.WriteByte(byte('0' + n/10))
		b.WriteByte(byte('0' + n%10))
	default:
		b.WriteString("    ")
		b.WriteByte(byte('0' + n))
	}
}

// buildSeverityMap 从 SARIF rules 中提取 ruleID -> level 映射
func buildSeverityMap(report sarifReport) map[string]string {
	// 预估容量
	totalRules := 0
	for _, run := range report.Runs {
		totalRules += len(run.Tool.Driver.Rules)
	}
	m := make(map[string]string, totalRules)

	for _, run := range report.Runs {
		for _, rule := range run.Tool.Driver.Rules {
			m[rule.ID] = rule.DefaultConfiguration.Level
		}
	}
	return m
}

// mapSeverity 将 SARIF level 映射为项目内部的 Severity 枚举
func mapSeverity(level string) model.Severity {
	// 直接比较，避免 strings.ToLower 开销
	switch level {
	case "error", "Error", "ERROR":
		return model.SeverityHigh
	case "warning", "Warning", "WARNING":
		return model.SeverityMedium
	case "note", "Note", "NOTE":
		return model.SeverityNote
	default:
		return model.SeverityLow
	}
}