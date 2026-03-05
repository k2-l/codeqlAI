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

var defaultLanguages = []string{"java", "go", "python", "javascript", "cpp"}

type Executor struct {
	binaryPath       string
	querySuite       string
	threads          int
	timeoutMinute    int
	storagePath      string
	threadsArg       string
	ramArg           string          // "--ram=N"，空表示不限制
	allowedLanguages map[string]bool
	languages        []string
}

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

	// 构建语言白名单
	langs := cfg.Languages
	if len(langs) == 0 {
		langs = defaultLanguages
	}
	allowed := make(map[string]bool, len(langs))
	for _, l := range langs {
		allowed[l] = true
	}

	// --ram 参数，0 表示不限制
	ramArg := ""
	if cfg.RAM > 0 {
		ramArg = "--ram=" + strconv.Itoa(cfg.RAM)
	}

	logger.Info("CodeQL executor initialized",
		zap.String("binary", binary),
		zap.String("query_suite", cfg.QuerySuite),
		zap.Int("threads", threads),
		zap.Int("timeout_minute", timeoutMinute),
		zap.Int("ram_mb", cfg.RAM),
		zap.Strings("languages", langs),
	)

	return &Executor{
		binaryPath:       binary,
		querySuite:       cfg.QuerySuite,
		threads:          threads,
		timeoutMinute:    timeoutMinute,
		storagePath:      cfg.StoragePath,
		threadsArg:       "--threads=" + strconv.Itoa(threads),
		ramArg:           ramArg,
		allowedLanguages: allowed,
		languages:        langs,
	}, nil
}

func checkBinary(binary string) error {
	path, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("codeql binary not found in PATH: %w\n请确认已安装 CodeQL CLI 并加入系统 PATH", err)
	}
	logger.Info("codeql binary found", zap.String("path", path))
	return nil
}

func (e *Executor) DBPath(taskID string) string {
	return filepath.Join(e.storagePath, taskID, "codeql-db")
}

func (e *Executor) SarifPath(taskID string) string {
	return filepath.Join(e.storagePath, taskID, "results.sarif")
}

func (e *Executor) timeout() time.Duration {
	return time.Duration(e.timeoutMinute) * time.Minute
}

func (e *Executor) Languages() []string {
	return e.languages
}

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
	if e.ramArg != "" {
		args = append(args, e.ramArg)
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
	if e.ramArg != "" {
		args = append(args, e.ramArg)
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

func (e *Executor) runCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(bytes.TrimSpace(out)), fmt.Errorf("command timed out after %d minutes", e.timeoutMinute)
	}

	return string(bytes.TrimSpace(out)), err
}

func (e *Executor) validateLanguage(language string) error {
	if !e.allowedLanguages[language] {
		return fmt.Errorf("unsupported language: %q, allowed: %v", language, e.languages)
	}
	return nil
}