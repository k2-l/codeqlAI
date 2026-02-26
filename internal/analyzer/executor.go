package analyzer

import (
	"codeqlAI/configs"
	"codeqlAI/pkg/logger"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Executor 封装 CodeQL CLI 调用
type Executor struct {
	binaryPath    string
	querySuite    string
	threads       int
	timeoutMinute int
	storagePath   string
}

// NewExecutor 初始化 Executor，同时做环境检查
func NewExecutor(cfg configs.CodeQLConfig) (*Executor, error) {
	binary := "codeql" // 默认从 PATH 查找
	if cfg.BinaryPath != "" {
		binary = cfg.BinaryPath
	}

	// 启动时立即检查 codeql 是否可用
	if err := checkBinary(binary); err != nil {
		return nil, err
	}

	threads := cfg.Threads
	if threads <= 0 {
		threads = 4 // 默认 4 线程
	}

	timeoutMinute := cfg.TimeoutMinute
	if timeoutMinute <= 0 {
		timeoutMinute = 30 // 默认 30 分钟超时
	}

	logger.Info("CodeQL executor initialized",
		zap.String("binary", binary),
		zap.String("query_suite", cfg.QuerySuite),
		zap.Int("threads", threads),
		zap.Int("timeout_minute", timeoutMinute),
	)

	return &Executor{
		binaryPath:    binary,
		querySuite:    cfg.QuerySuite,
		threads:       threads,
		timeoutMinute: timeoutMinute,
		storagePath:   cfg.StoragePath,
	}, nil
}

// checkBinary 检查 codeql 二进制是否存在且可执行
func checkBinary(binary string) error {
	path, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("codeql binary not found in PATH: %w\n请确认已安装 CodeQL CLI 并加入系统 PATH", err)
	}
	logger.Info("codeql binary found", zap.String("path", path))
	return nil
}

// DBPath 返回某个任务的 CodeQL 数据库存放路径
func (e *Executor) DBPath(taskID string) string {
	return filepath.Join(e.storagePath, taskID, "codeql-db")
}

// SarifPath 返回某个任务的 SARIF 结果存放路径
func (e *Executor) SarifPath(taskID string) string {
	return filepath.Join(e.storagePath, taskID, "results.sarif")
}

// CreateDatabase 调用 codeql database create 为指定代码目录建库
// taskID: 任务 ID，用于隔离不同任务的文件
// language: 扫描语言（java/go/python/javascript/cpp）
// sourceRoot: 源代码根目录
func (e *Executor) CreateDatabase(taskID string, language string, sourceRoot string) error {
	// 语言白名单校验，防止命令注入
	if err := validateLanguage(language); err != nil {
		return err
	}

	dbPath := e.DBPath(taskID)
	timeout := time.Duration(e.timeoutMinute) * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{
		"database", "create",
		dbPath,
		"--language=" + language,
		"--source-root=" + sourceRoot,
		"--threads=" + fmt.Sprintf("%d", e.threads),
		"--overwrite", // 允许覆盖已有数据库（重跑任务时用）
	}

	logger.Info("creating CodeQL database",
		zap.String("task_id", taskID),
		zap.String("language", language),
		zap.String("source_root", sourceRoot),
		zap.String("db_path", dbPath),
	)

	output, err := e.runCommand(ctx, args...)
	if err != nil {
		logger.Error("codeql database create failed",
			zap.String("task_id", taskID),
			zap.String("output", output),
			zap.Error(err),
		)
		return fmt.Errorf("codeql database create failed: %w\nOutput: %s", err, output)
	}

	logger.Info("CodeQL database created successfully",
		zap.String("task_id", taskID),
		zap.String("db_path", dbPath),
	)
	return nil
}

// RunAnalysis 调用 codeql database analyze 运行查询并输出 SARIF
// taskID: 任务 ID
// language: 扫描语言（用于选择对应的查询套件）
func (e *Executor) RunAnalysis(taskID string, language string) error {
	if err := validateLanguage(language); err != nil {
		return err
	}

	dbPath := e.DBPath(taskID)
	sarifPath := e.SarifPath(taskID)

	// 根据语言选择对应的官方查询套件
	// 格式：codeql/{language}-queries:codeql-suites/{language}-{suite}.qls
	querySuite := fmt.Sprintf("codeql/%s-queries:codeql-suites/%s-%s.qls", language, language, e.querySuite)

	timeout := time.Duration(e.timeoutMinute) * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{
		"database", "analyze",
		dbPath,
		querySuite,
		"--format=sarif-latest",
		"--output=" + sarifPath,
		"--threads=" + fmt.Sprintf("%d", e.threads),
	}

	logger.Info("running CodeQL analysis",
		zap.String("task_id", taskID),
		zap.String("query_suite", querySuite),
		zap.String("sarif_path", sarifPath),
	)

	output, err := e.runCommand(ctx, args...)
	if err != nil {
		logger.Error("codeql database analyze failed",
			zap.String("task_id", taskID),
			zap.String("output", output),
			zap.Error(err),
		)
		return fmt.Errorf("codeql database analyze failed: %w\nOutput: %s", err, output)
	}

	logger.Info("CodeQL analysis completed",
		zap.String("task_id", taskID),
		zap.String("sarif_path", sarifPath),
	)
	return nil
}

// runCommand 执行 codeql 命令，返回合并后的输出和错误
func (e *Executor) runCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)

	// 合并 stdout 和 stderr，方便记录完整日志
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after %d minutes", e.timeoutMinute)
	}

	return output, err
}

// validateLanguage 白名单校验，防止语言参数被注入恶意命令
func validateLanguage(language string) error {
	allowed := map[string]bool{
		"java":       true,
		"go":         true,
		"python":     true,
		"javascript": true,
		"cpp":        true,
	}
	if !allowed[language] {
		return fmt.Errorf("unsupported language: %q, allowed: java/go/python/javascript/cpp", language)
	}
	return nil
}