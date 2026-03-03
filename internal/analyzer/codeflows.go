package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
)

// FlowNode 数据流中的一个节点
type FlowNode struct {
	Index    int    `json:"index"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
}

// FlowPath 一条完整的数据流路径（source → ... → sink）
type FlowPath struct {
	Nodes []FlowNode `json:"nodes"`
}

// FindingFlow 一个 finding 及其所有数据流路径
type FindingFlow struct {
	RuleID   string     `json:"rule_id"`
	Message  string     `json:"message"`
	Severity string     `json:"severity"`
	FilePath string     `json:"file_path"`
	Line     int        `json:"line"`
	Flows    []FlowPath `json:"flows"`
}

// ParseCodeFlows 从 SARIF 文件中解析所有带 codeFlows 的 finding
func ParseCodeFlows(sarifPath string) ([]FindingFlow, error) {
	data, err := os.ReadFile(sarifPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sarif file: %w", err)
	}

	// 使用带 codeFlows 扩展的 SARIF 结构体
	var sarif sarifReportFull
	if err := json.Unmarshal(data, &sarif); err != nil {
		return nil, fmt.Errorf("failed to parse sarif: %w", err)
	}

	var result []FindingFlow

	for _, run := range sarif.Runs {
		ruleMap := make(map[string]sarifRule, len(run.Tool.Driver.Rules))
		for _, r := range run.Tool.Driver.Rules {
			ruleMap[r.ID] = r
		}

		for _, res := range run.Results {
			if len(res.CodeFlows) == 0 {
				continue
			}

			filePath := ""
			line := 0
			if len(res.Locations) > 0 {
				loc := res.Locations[0]
				filePath = loc.PhysicalLocation.ArtifactLocation.URI
				line = loc.PhysicalLocation.Region.StartLine
			}

			meta := ruleMap[res.RuleID]
			ff := FindingFlow{
				RuleID:   res.RuleID,
				Message:  res.Message.Text,
				Severity: levelToSeverity(meta.DefaultConfiguration.Level),
				FilePath: filePath,
				Line:     line,
			}

			for _, cf := range res.CodeFlows {
				for _, tf := range cf.ThreadFlows {
					var path FlowPath
					for i, loc := range tf.Locations {
						ploc := loc.Location.PhysicalLocation
						node := FlowNode{
							Index:    i,
							FilePath: ploc.ArtifactLocation.URI,
							Line:     ploc.Region.StartLine,
							Column:   ploc.Region.StartColumn,
							Message:  loc.Location.Message.Text,
						}
						path.Nodes = append(path.Nodes, node)
					}
					if len(path.Nodes) >= 2 {
						ff.Flows = append(ff.Flows, path)
					}
				}
			}

			if len(ff.Flows) > 0 {
				result = append(result, ff)
			}
		}
	}

	return result, nil
}

func levelToSeverity(level string) string {
	switch level {
	case "error":
		return "high"
	case "warning":
		return "medium"
	default:
		return "low"
	}
}

