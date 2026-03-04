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

// ParseCodeFlows 从 SARIF 文件中解析所有带 codeFlows 的 finding。
// 使用流式解码，避免将整个文件一次性载入内存。
func ParseCodeFlows(sarifPath string) ([]FindingFlow, error) {
	f, err := os.Open(sarifPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sarif file: %w", err)
	}
	defer f.Close()

	var sarif sarifReportFull
	if err := json.NewDecoder(f).Decode(&sarif); err != nil {
		return nil, fmt.Errorf("failed to parse sarif: %w", err)
	}

	// 以 results 总数为上限预分配，避免 append 反复扩容
	total := 0
	for _, r := range sarif.Runs {
		total += len(r.Results)
	}
	result := make([]FindingFlow, 0, total)

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

			ff := FindingFlow{
				RuleID:   res.RuleID,
				Message:  res.Message.Text,
				Severity: levelToSeverity(ruleMap[res.RuleID].DefaultConfiguration.Level),
				FilePath: filePath,
				Line:     line,
				Flows:    make([]FlowPath, 0, len(res.CodeFlows)),
			}

			for _, cf := range res.CodeFlows {
				for _, tf := range cf.ThreadFlows {
					// 不足 2 个节点无法构成路径，提前跳过避免无效分配
					if len(tf.Locations) < 2 {
						continue
					}
					// 长度已知，直接分配定长切片，用下标赋值代替 append
					nodes := make([]FlowNode, len(tf.Locations))
					for i, loc := range tf.Locations {
						ploc := loc.Location.PhysicalLocation
						nodes[i] = FlowNode{
							Index:    i,
							FilePath: ploc.ArtifactLocation.URI,
							Line:     ploc.Region.StartLine,
							Column:   ploc.Region.StartColumn,
							Message:  loc.Location.Message.Text,
						}
					}
					ff.Flows = append(ff.Flows, FlowPath{Nodes: nodes})
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