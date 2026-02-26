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
	ID                   string              `json:"id"`
	ShortDescription     sarifMessage        `json:"shortDescription"`
	DefaultConfiguration sarifConfiguration  `json:"defaultConfiguration"`
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
	StartLine int `json:"startLine"`
	EndLine   int `json:"endLine"`
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

	// 读取并解析 SARIF JSON
	data, err := os.ReadFile(sarifPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sarif file: %w", err)
	}

	var report sarifReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse sarif json: %w", err)
	}

	// 构建 ruleID -> severity 映射表（从 tool.driver.rules 提取）
	severityMap := buildSeverityMap(report)

	var findings []model.Finding

	for _, run := range report.Runs {
		for _, result := range run.Results {
			if len(result.Locations) == 0 {
				continue // 没有位置信息的结果跳过
			}

			loc := result.Locations[0].PhysicalLocation
			filePath := loc.ArtifactLocation.URI
			startLine := loc.Region.StartLine
			endLine := loc.Region.EndLine

			// endLine 有时候不存在，默认等于 startLine
			if endLine == 0 {
				endLine = startLine
			}

			// 读取代码片段（含上下文）
			snippet, err := extractCodeSnippet(
				filepath.Join(sourceRoot, filePath),
				startLine,
				endLine,
				contextLines,
			)
			if err != nil {
				// 读取失败不中断整体解析，记录警告继续处理
				logger.Warn("failed to extract code snippet",
					zap.String("file", filePath),
					zap.Int("line", startLine),
					zap.Error(err),
				)
				snippet = "[code snippet unavailable]"
			}

			finding := model.Finding{
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
			}

			findings = append(findings, finding)
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

	var lines []string
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
		prefix := "    "
		if lineNum >= startLine && lineNum <= endLine {
			prefix = ">>> " // 漏洞所在行加箭头标记
		}

		lines = append(lines, fmt.Sprintf("%s%4d | %s", prefix, lineNum, scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	return strings.Join(lines, "\n"), nil
}

// buildSeverityMap 从 SARIF rules 中提取 ruleID -> level 映射
func buildSeverityMap(report sarifReport) map[string]string {
	m := make(map[string]string)
	for _, run := range report.Runs {
		for _, rule := range run.Tool.Driver.Rules {
			m[rule.ID] = rule.DefaultConfiguration.Level
		}
	}
	return m
}

// mapSeverity 将 SARIF level 映射为项目内部的 Severity 枚举
func mapSeverity(level string) model.Severity {
	switch strings.ToLower(level) {
	case "error":
		return model.SeverityHigh
	case "warning":
		return model.SeverityMedium
	case "note":
		return model.SeverityNote
	default:
		return model.SeverityLow
	}
}