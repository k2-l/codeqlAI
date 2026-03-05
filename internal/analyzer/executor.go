package analyzer

import (
	"bytes"
	"codeqlAI/configs"
	"codeqlAI/pkg/logger"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// defaultLanguages 未配置时的兜底列表
var defaultLanguages = []string{"java", "go", "python", "javascript", "cpp"}

// Executor 封装 CodeQL CLI 调用
type Executor struct {
	binaryPath       string
	querySuite       string
	threads          int
	timeoutMinute    int
	storagePath      string
	threadsArg       string
	allowedLanguages map[string]bool // 从配置构建，运行时不变
	languages        []string        // 有序列表，供前端展示用
}

// NewExecutor 初始化 Executor，同时做环境检查
func NewExecutor(cfg configs.CodeQLConfig) (*Executor, error) {
	binary := "codeql"
	if cfg.BinaryPath != "" {
		binary = cfg.BinaryPath
	}

	if err := checkBinary(binary); err != nil {
		return nil, err
	}

	threads := cfg.Threads
	if threads <= 0 {
		threads = 4
	}

	timeoutMinute := cfg.TimeoutMinute
	if timeoutMinute <= 0 {
		timeoutMinute = 30
	}

	// 构建语言白名单，未配置则使用默认列表
	langs := cfg.Languages
	if len(langs) == 0 {
		langs = defaultLanguages
	}
	allowed := make(map[string]bool, len(langs))
	for _, l := range langs {
		allowed[l] = true
	}

	logger.Info("CodeQL executor initialized",
		zap.String("binary", binary),
		zap.String("query_suite", cfg.QuerySuite),
		zap.Int("threads", threads),
		zap.Int("timeout_minute", timeoutMinute),
		zap.Strings("languages", langs),
	)

	return &Executor{
		binaryPath:       binary,
		querySuite:       cfg.QuerySuite,
		threads:          threads,
		timeoutMinute:    timeoutMinute,
		storagePath:      cfg.StoragePath,
		threadsArg:       "--threads=" + strconv.Itoa(threads),
		allowedLanguages: allowed,
		languages:        langs,
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

// timeout 返回统一的超时 Duration，避免两处重复计算
func (e *Executor) timeout() time.Duration {
	return time.Duration(e.timeoutMinute) * time.Minute
}

// CreateDatabase 调用 codeql database create 为指定代码目录建库
func (e *Executor) CreateDatabase(taskID string, language string, sourceRoot string) error {
	if err := e.validateLanguage(language); err != nil {
		return err
	}

	dbPath := e.DBPath(taskID)
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()

	args := []string{
		"database", "create",
		dbPath,
		"--language=" + language,
		"--source-root=" + sourceRoot,
		e.threadsArg,
		"--overwrite",
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
// customQLPath 为空则使用官方查询套件
func (e *Executor) RunAnalysis(taskID string, language string, customQLPath string) error {
	if err := e.validateLanguage(language); err != nil {
		return err
	}

	dbPath := e.DBPath(taskID)
	sarifPath := e.SarifPath(taskID)

	var queryTarget string
	if customQLPath != "" {
		queryTarget = customQLPath
		logger.Info("using custom QL rule", zap.String("task_id", taskID), zap.String("ql_path", customQLPath))
	} else {
		queryTarget = fmt.Sprintf("codeql/%s-queries:codeql-suites/%s-%s.qls", language, language, e.querySuite)
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout())
	defer cancel()

	args := []string{
		"database", "analyze",
		dbPath,
		queryTarget,
		"--format=sarif-latest",
		"--output=" + sarifPath,
		e.threadsArg,
	}

	logger.Info("running CodeQL analysis",
		zap.String("task_id", taskID),
		zap.String("query_suite", queryTarget),
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
	out, err := cmd.CombinedOutput()

	// 优先判断超时：ctx 已超时时底层 err 是"signal: killed"，语义不清晰
	if ctx.Err() == context.DeadlineExceeded {
		return string(bytes.TrimSpace(out)), fmt.Errorf("command timed out after %d minutes", e.timeoutMinute)
	}

	return string(bytes.TrimSpace(out)), err
}

// Languages 返回当前支持的语言列表（供 API 层调用）
func (e *Executor) Languages() []string {
	return e.languages
}

// validateLanguage 白名单校验，防止语言参数被注入恶意命令
func (e *Executor) validateLanguage(language string) error {
	if !e.allowedLanguages[language] {
		return fmt.Errorf("unsupported language: %q, allowed: %v", language, e.languages)
	}
	return nil
}
