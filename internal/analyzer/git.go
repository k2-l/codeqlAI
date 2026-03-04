package analyzer

import (
	"bytes"
	"codeqlAI/pkg/logger"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// privateNetworks 内网/保留 IP 段，包级变量只解析一次
var privateNetworks []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // 云服务商元数据接口（如 AWS 169.254.169.254）
		"fc00::/7",
		"fe80::/10",
		"::1/128",
	}
	privateNetworks = make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			privateNetworks = append(privateNetworks, network)
		}
	}
}

// GitCloneOptions Git 克隆选项
type GitCloneOptions struct {
	URL      string // 仓库地址
	Branch   string // 分支或 Tag，留空则克隆默认分支
	Token    string // 私有仓库 Token（HTTPS 认证）
	SSHKey   string // 私有仓库 SSH Key 文件路径
	DestPath string // 克隆目标目录
}

// CloneRepository 克隆 Git 仓库到指定目录
func CloneRepository(opts GitCloneOptions) error {
	// 1. SSRF 防护
	if err := validateGitURL(opts.URL); err != nil {
		return err
	}

	// 2. 目标目录已存在则先清理
	if _, err := os.Stat(opts.DestPath); err == nil {
		if err := os.RemoveAll(opts.DestPath); err != nil {
			return fmt.Errorf("failed to clean dest path: %w", err)
		}
	}

	// 3. 构建克隆 URL（注入 Token 用于 HTTPS 私有仓库认证）
	cloneURL := opts.URL
	if opts.Token != "" && isHTTPS(opts.URL) {
		cloneURL = injectToken(opts.URL, opts.Token)
	}

	// 4. 构建 git clone 命令
	args := buildCloneArgs(cloneURL, opts.Branch, opts.DestPath)

	logger.Info("cloning repository",
		zap.String("url", opts.URL), // 打印原始 URL，不含 Token
		zap.String("branch", opts.Branch),
		zap.String("dest", opts.DestPath),
	)

	// 5. 执行克隆；SSH Key 认证通过环境变量注入
	cmd := exec.Command("git", args...)
	if opts.SSHKey != "" {
		cmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i "+opts.SSHKey+" -o StrictHostKeyChecking=no",
		)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, bytes.TrimSpace(out))
	}

	logger.Info("repository cloned successfully", zap.String("dest", opts.DestPath))
	return nil
}

// buildCloneArgs 构建 git clone 参数列表，容量预分配避免扩容
func buildCloneArgs(cloneURL, branch, destPath string) []string {
	// 基础参数 3 个，有 branch 则 +2
	capacity := 3
	if branch != "" {
		capacity = 5
	}
	args := make([]string, 0, capacity)
	args = append(args, "clone", "--depth=1")
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	return append(args, cloneURL, destPath)
}

// ValidateGitURL 校验 Git URL，防止 SSRF 攻击（公开函数，供 service 层提前校验）
func ValidateGitURL(rawURL string) error {
	return validateGitURL(rawURL)
}

func validateGitURL(rawURL string) error {
	if !isHTTPS(rawURL) && !isSSH(rawURL) {
		return fmt.Errorf("unsupported URL scheme, only https:// and git@/ssh:// are allowed")
	}
	if isSSH(rawURL) {
		return validateSSHURL(rawURL)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	return checkHost(parsed.Hostname())
}

// validateSSHURL 校验 SSH 格式 git URL（git@github.com:user/repo.git）
func validateSSHURL(sshURL string) error {
	// 提取 "@" 后的 host 部分
	_, after, found := strings.Cut(sshURL, "@")
	if !found {
		return fmt.Errorf("invalid SSH URL format")
	}
	// host 在第一个 ":" 之前
	host, _, _ := strings.Cut(after, ":")
	return checkHost(host)
}

// checkHost 统一执行 localhost 检查 + 内网 IP 检查
func checkHost(hostname string) error {
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return fmt.Errorf("SSRF protection: localhost is not allowed")
	}

	ips, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve host %s: %w", hostname, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip != nil && isPrivateIP(ip) {
			return fmt.Errorf("SSRF protection: private/internal IP address is not allowed: %s", ipStr)
		}
	}
	return nil
}

// isPrivateIP 判断是否为内网/保留 IP 地址
func isPrivateIP(ip net.IP) bool {
	for _, network := range privateNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// injectToken 将 Token 注入 HTTPS URL 用于认证
func injectToken(rawURL, token string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = url.UserPassword("oauth2", token)
	return parsed.String()
}

func isHTTPS(u string) bool { return strings.HasPrefix(u, "https://") }
func isSSH(u string) bool   { return strings.HasPrefix(u, "git@") || strings.HasPrefix(u, "ssh://") }