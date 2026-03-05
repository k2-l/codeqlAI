// configs/config.go
package configs

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Auth     AuthConfig     `yaml:"auth"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	CodeQL   CodeQLConfig   `yaml:"codeql"`
	AI       AIConfig       `yaml:"ai"`
}

type AuthConfig struct {
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	JWTSecret string `yaml:"jwt_secret"`
	TokenTTLH int    `yaml:"token_ttl_hours"` // Token 有效期（小时），默认 24
}

type AppConfig struct {
	Name  string `yaml:"name"`
	Port  int    `yaml:"port"`
	Debug bool   `yaml:"debug"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
	TimeZone string `yaml:"timezone"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type CodeQLConfig struct {
	BinaryPath    string   `yaml:"binary_path"`
	QuerySuite    string   `yaml:"query_suite"`
	Threads       int      `yaml:"threads"`
	TimeoutMinute int      `yaml:"timeout_minute"`
	StoragePath   string   `yaml:"storage_path"`
	Languages     []string `yaml:"languages"` // 支持的扫描语言列表，留空则使用默认值
}

type AIConfig struct {
	Provider   string  `yaml:"provider"`
	BaseURL    string  `yaml:"base_url"`
	APIKey     string  `yaml:"api_key"`
	Model      string  `yaml:"model"`
	MaxTokens  int     `yaml:"max_tokens"`
	TimeoutSec int     `yaml:"timeout_sec"`
	RateLimit  int     `yaml:"rate_limit"`
}

// Load 从指定路径加载 YAML 配置文件
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
