package analyzer

import (
	"codeqlAI/pkg/logger"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

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
	// 1. SSRF 防护：校验 URL 不指向内网
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
		zap.String("url", opts.URL), // 注意：打印原始 URL，不含 Token
		zap.String("branch", opts.Branch),
		zap.String("dest", opts.DestPath),
	)

	// 5. 执行克隆
	var cmd *exec.Cmd
	if opts.SSHKey != "" {
		// SSH Key 认证：通过环境变量注入 GIT_SSH_COMMAND
		cmd = exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", opts.SSHKey),
		)
	} else {
		cmd = exec.Command("git", args...)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, strings.TrimSpace(string(out)))
	}

	logger.Info("repository cloned successfully", zap.String("dest", opts.DestPath))
	return nil
}

// buildCloneArgs 构建 git clone 参数列表
func buildCloneArgs(cloneURL, branch, destPath string) []string {
	args := []string{"clone", "--depth=1"} // depth=1 只克隆最新一个提交，节省时间和空间
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, cloneURL, destPath)
	return args
}

// ValidateGitURL 校验 Git URL，防止 SSRF 攻击（公开函数，供 service 层提前校验）
func ValidateGitURL(rawURL string) error {
	return validateGitURL(rawURL)
}

// validateGitURL 内部实现
func validateGitURL(rawURL string) error {
	// 只允许 HTTPS 和 SSH 协议
	if !isHTTPS(rawURL) && !isSSH(rawURL) {
		return fmt.Errorf("unsupported URL scheme, only https:// and git@/ssh:// are allowed")
	}

	// SSH 格式（git@github.com:user/repo.git）单独处理
	if isSSH(rawURL) {
		return validateSSHURL(rawURL)
	}

	// 解析 HTTPS URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	hostname := parsed.Hostname()

	// 拒绝 localhost
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return fmt.Errorf("SSRF protection: localhost is not allowed")
	}

	// 解析 IP，检查是否为内网地址
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve host %s: %w", hostname, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("SSRF protection: private/internal IP address is not allowed: %s", ipStr)
		}
	}

	return nil
}

// validateSSHURL 校验 SSH 格式的 Git URL
// 格式：git@github.com:user/repo.git
func validateSSHURL(sshURL string) error {
	// 提取 host 部分
	// git@github.com:user/repo.git -> github.com
	parts := strings.SplitN(sshURL, "@", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid SSH URL format")
	}
	hostAndPath := parts[1]
	host := strings.SplitN(hostAndPath, ":", 2)[0]

	if host == "localhost" || host == "127.0.0.1" {
		return fmt.Errorf("SSRF protection: localhost is not allowed")
	}

	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host %s: %w", host, err)
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip != nil && isPrivateIP(ip) {
			return fmt.Errorf("SSRF protection: private IP is not allowed: %s", ipStr)
		}
	}
	return nil
}

// isPrivateIP 判断是否为内网/保留 IP 地址
func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // 云服务商元数据接口（如 AWS 169.254.169.254）
		"fc00::/7",
		"fe80::/10",
		"::1/128",
	}
	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// injectToken 将 Token 注入 HTTPS URL 用于认证
// https://github.com/user/repo.git -> https://token@github.com/user/repo.git
func injectToken(rawURL, token string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = url.UserPassword("oauth2", token)
	return parsed.String()
}

func isHTTPS(u string) bool {
	return strings.HasPrefix(u, "https://")
}

func isSSH(u string) bool {
	return strings.HasPrefix(u, "git@") || strings.HasPrefix(u, "ssh://")
}