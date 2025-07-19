package installer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Installer struct {
	Progress chan ProgressUpdate
	logs     []string
}

type ProgressUpdate struct {
	Step    string
	Message string
	Percent float64
	Error   error
}

func New() *Installer {
	return &Installer{
		Progress: make(chan ProgressUpdate, 100),
		logs:     make([]string, 0),
	}
}

func (i *Installer) Install() {
	defer close(i.Progress)

	steps := []struct {
		name   string
		fn     func() error
		weight float64
	}{
		{"检查系统环境", i.checkSystem, 5},
		{"检测 Node.js", i.checkNodeJS, 10},
		{"安装 Node.js", i.installNodeJS, 20},
		{"检测 Git", i.checkGit, 10},
		{"安装 Git", i.installGit, 20},
		{"安装 Claude Code", i.installClaudeCode, 20},
		{"配置 K2 API", func() error { return nil }, 10}, // K2 API 配置将在后续步骤中进行
		{"验证安装", i.verifyInstallation, 5},
	}

	totalWeight := 0.0
	for _, step := range steps {
		totalWeight += step.weight
	}

	currentProgress := 0.0

	for _, step := range steps {
		i.sendProgress(step.name, fmt.Sprintf("正在%s...", step.name), currentProgress/totalWeight)

		err := step.fn()
		if err != nil {
			i.sendProgress(step.name, fmt.Sprintf("%s失败: %v", step.name, err), currentProgress/totalWeight)
			i.sendError(fmt.Errorf("%s失败: %v", step.name, err))
			return
		}

		currentProgress += step.weight
		i.sendProgress(step.name, fmt.Sprintf("%s完成", step.name), currentProgress/totalWeight)
	}

	i.sendProgress("完成", "所有组件安装完成！", 1.0)
}

func (i *Installer) checkSystem() error {
	i.addLog(fmt.Sprintf("操作系统: %s", runtime.GOOS))
	i.addLog(fmt.Sprintf("架构: %s", runtime.GOARCH))

	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

	return nil
}

func (i *Installer) checkNodeJS() error {
	cmd := exec.Command("node", "--version")
	output, err := cmd.Output()

	if err == nil {
		version := strings.TrimSpace(string(output))
		i.addLog(fmt.Sprintf("检测到 Node.js: %s", version))

		// 检查版本是否满足要求 - 提取主版本号
		// 版本格式通常是 v16.14.0 或 v20.10.0
		if strings.HasPrefix(version, "v") {
			// 提取主版本号
			parts := strings.Split(version[1:], ".")
			if len(parts) >= 1 {
				majorVersion, err := strconv.Atoi(parts[0])
				if err == nil && majorVersion >= 16 {
					i.addLog(fmt.Sprintf("Node.js 版本满足要求 (v%d >= v16)", majorVersion))
					return nil
				}
			}
		}

		return fmt.Errorf("Node.js 版本过低，需要 v16 或更高版本")
	}

	i.addLog("未检测到 Node.js，需要安装")
	return fmt.Errorf("未安装 Node.js")
}

func (i *Installer) installNodeJS() error {
	// 检查是否需要安装
	if err := i.checkNodeJS(); err == nil {
		i.addLog("Node.js 已安装，跳过")
		return nil
	}

	switch runtime.GOOS {
	case "windows":
		return i.installNodeJSWindows()
	case "darwin":
		return i.installNodeJSMac()
	case "linux":
		return i.installNodeJSLinux()
	default:
		return fmt.Errorf("不支持的操作系统")
	}
}

func (i *Installer) installNodeJSWindows() error {
	// 下载 Node.js 安装程序
	// 使用淘宝镜像源
	nodeURL := "https://cdn.npmmirror.com/binaries/node/v20.10.0/node-v20.10.0-x64.msi"

	tempDir := os.TempDir()
	installerPath := filepath.Join(tempDir, "node-installer.msi")

	i.addLog("下载 Node.js 安装程序...")
	err := i.downloadFile(nodeURL, installerPath)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}

	i.addLog("运行 Node.js 安装程序...")
	// 使用 /qn 而不是 /quiet 以确保静默安装
	// ADDLOCAL=ALL 确保安装所有组件包括 npm
	// ALLUSERS=1 为所有用户安装
	cmd := exec.Command("msiexec", "/i", installerPath, "/qn", "/norestart", "ADDLOCAL=ALL", "ALLUSERS=1")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("安装失败: %v", err)
	}

	// 清理安装文件
	os.Remove(installerPath)

	// Windows 下需要刷新环境变量
	i.addLog("刷新系统环境变量...")
	// 通知系统环境变量已更改
	cmd = exec.Command("setx", "NODE_REFRESH", "1")
	cmd.Run() // 忽略错误

	// 等待一下让系统处理
	time.Sleep(3 * time.Second)

	// 尝试直接使用完整路径验证安装
	possiblePaths := []string{
		`C:\Program Files\nodejs\node.exe`,
		`C:\Program Files (x86)\nodejs\node.exe`,
		filepath.Join(os.Getenv("ProgramFiles"), "nodejs", "node.exe"),
	}

	for _, nodePath := range possiblePaths {
		if _, err := os.Stat(nodePath); err == nil {
			i.addLog(fmt.Sprintf("Node.js 已安装到: %s", nodePath))
			// 设置临时环境变量供后续使用
			os.Setenv("PATH", fmt.Sprintf("%s;%s", filepath.Dir(nodePath), os.Getenv("PATH")))
			return nil
		}
	}

	// 如果找不到，但安装没报错，可能需要重启
	i.addLog("⚠️ Node.js 安装完成，但可能需要重启终端才能使用")
	return nil
}

func (i *Installer) installNodeJSMac() error {
	// 检查是否有 Homebrew
	cmd := exec.Command("brew", "--version")
	if err := cmd.Run(); err != nil {
		i.addLog("未检测到 Homebrew，尝试下载安装包...")
		return i.installNodeJSMacPkg()
	}

	i.addLog("使用 Homebrew 安装 Node.js...")
	cmd = exec.Command("brew", "install", "node")
	output, err := cmd.CombinedOutput()
	i.addLog(string(output))

	return err
}

func (i *Installer) installNodeJSMacPkg() error {
	// 使用淘宝镜像源
	nodeURL := "https://cdn.npmmirror.com/binaries/node/v20.10.0/node-v20.10.0.pkg"

	tempDir := os.TempDir()
	installerPath := filepath.Join(tempDir, "node-installer.pkg")

	i.addLog("下载 Node.js 安装程序...")
	err := i.downloadFile(nodeURL, installerPath)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}

	i.addLog("运行 Node.js 安装程序...")
	cmd := exec.Command("sudo", "installer", "-pkg", installerPath, "-target", "/")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("安装失败: %v", err)
	}

	// 清理安装文件
	os.Remove(installerPath)

	return nil
}

func (i *Installer) installNodeJSLinux() error {
	// 尝试使用包管理器
	if _, err := exec.LookPath("apt-get"); err == nil {
		i.addLog("使用 apt-get 安装 Node.js...")
		cmd := exec.Command("sudo", "apt-get", "update")
		cmd.Run()

		cmd = exec.Command("sudo", "apt-get", "install", "-y", "nodejs", "npm")
		output, err := cmd.CombinedOutput()
		i.addLog(string(output))
		return err
	}

	if _, err := exec.LookPath("yum"); err == nil {
		i.addLog("使用 yum 安装 Node.js...")
		cmd := exec.Command("sudo", "yum", "install", "-y", "nodejs", "npm")
		output, err := cmd.CombinedOutput()
		i.addLog(string(output))
		return err
	}

	return fmt.Errorf("无法自动安装 Node.js，请手动安装")
}

func (i *Installer) checkGit() error {
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()

	if err == nil {
		version := strings.TrimSpace(string(output))
		i.addLog(fmt.Sprintf("检测到 Git: %s", version))
		return nil
	}

	i.addLog("未检测到 Git，需要安装")
	return fmt.Errorf("未安装 Git")
}

func (i *Installer) installGit() error {
	// 检查是否需要安装
	if err := i.checkGit(); err == nil {
		i.addLog("Git 已安装，跳过")
		return nil
	}

	switch runtime.GOOS {
	case "windows":
		return i.installGitWindows()
	case "darwin":
		return i.installGitMac()
	case "linux":
		return i.installGitLinux()
	default:
		return fmt.Errorf("不支持的操作系统")
	}
}

func (i *Installer) installGitWindows() error {
	// 使用清华大学镜像源（Git for Windows）
	// 如果清华镜像不可用，可切换到其他镜像
	gitURL := "https://mirrors.tuna.tsinghua.edu.cn/github-release/git-for-windows/git/v2.43.0.windows.1/Git-2.43.0-64-bit.exe"
	// 备选：直接使用 GitHub 的 CDN（通常在国内也比较快）
	// gitURL := "https://github.com/git-for-windows/git/releases/download/v2.43.0.windows.1/Git-2.43.0-64-bit.exe"

	tempDir := os.TempDir()
	installerPath := filepath.Join(tempDir, "git-installer.exe")

	i.addLog("下载 Git 安装程序...")
	err := i.downloadFile(gitURL, installerPath)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}

	i.addLog("运行 Git 安装程序...")
	// /VERYSILENT 静默安装
	// /NORESTART 不重启
	// /COMPONENTS="icons,ext\reg\shellhere,assoc,assoc_sh" 安装基本组件
	cmd := exec.Command(installerPath, "/VERYSILENT", "/NORESTART", "/NOCANCEL", "/SP-", "/CLOSEAPPLICATIONS", "/RESTARTAPPLICATIONS")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("安装失败: %v", err)
	}

	// 清理安装文件
	os.Remove(installerPath)

	// Windows 下需要刷新环境变量
	i.addLog("刷新 Git 环境变量...")
	time.Sleep(2 * time.Second)

	// 尝试直接使用完整路径验证安装
	possibleGitPaths := []string{
		`C:\Program Files\Git\bin\git.exe`,
		`C:\Program Files (x86)\Git\bin\git.exe`,
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "git.exe"),
	}

	for _, gitPath := range possibleGitPaths {
		if _, err := os.Stat(gitPath); err == nil {
			i.addLog(fmt.Sprintf("Git 已安装到: %s", gitPath))
			// 设置临时环境变量供后续使用
			os.Setenv("PATH", fmt.Sprintf("%s;%s", filepath.Dir(gitPath), os.Getenv("PATH")))
			return nil
		}
	}

	i.addLog("⚠️ Git 安装完成，但可能需要重启终端才能使用")
	return nil
}

func (i *Installer) installGitMac() error {
	// macOS 通常自带 Git，如果没有，使用 Homebrew
	cmd := exec.Command("brew", "install", "git")
	output, err := cmd.CombinedOutput()
	i.addLog(string(output))

	return err
}

func (i *Installer) installGitLinux() error {
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.Command("sudo", "apt-get", "install", "-y", "git")
		output, err := cmd.CombinedOutput()
		i.addLog(string(output))
		return err
	}

	if _, err := exec.LookPath("yum"); err == nil {
		cmd := exec.Command("sudo", "yum", "install", "-y", "git")
		output, err := cmd.CombinedOutput()
		i.addLog(string(output))
		return err
	}

	return fmt.Errorf("无法自动安装 Git，请手动安装")
}

func (i *Installer) installClaudeCode() error {
	i.addLog("安装 Claude Code...")

	// 使用淘宝 npm 镜像
	cmd := exec.Command("npm", "install", "-g", "@anthropic-ai/claude-code", "--registry=https://registry.npmmirror.com")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	i.addLog(out.String())

	if err != nil {
		return fmt.Errorf("安装 Claude Code 失败: %v", err)
	}

	// 验证安装
	cmd = exec.Command("claude", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Claude Code 安装验证失败: %v", err)
	}

	i.addLog(fmt.Sprintf("Claude Code 安装成功: %s", string(output)))
	return nil
}

func (i *Installer) configureK2API(apiKey string) error {
	return i.configureK2APIWithOptions(apiKey, "30", false)
}

func (i *Installer) configureK2APIWithOptions(apiKey string, rpm string, useSystemConfig bool) error {
	if apiKey == "" {
		i.addLog("跳过 K2 API 配置（未提供 API Key）")
		return nil
	}

	i.addLog(fmt.Sprintf("配置 K2 API（速率限制: %s RPM）...", rpm))

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	// 计算请求延迟（毫秒）
	rpmInt, _ := strconv.Atoi(rpm)
	requestDelay := 60000 / rpmInt // 60秒转毫秒除以RPM

	// 配置内容
	configContent := fmt.Sprintf(`{
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.moonshot.cn/anthropic/",
    "ANTHROPIC_API_KEY": "%s",
    "CLAUDE_REQUEST_DELAY_MS": "%d",
    "CLAUDE_MAX_CONCURRENT_REQUESTS": "1"
  }
}`, apiKey, requestDelay)

	// 尝试写入系统级配置（需要管理员权限）
	systemConfigWritten := false
	if useSystemConfig {
		i.addLog("尝试写入系统级配置...")
		if runtime.GOOS == "windows" {
			systemPath := `C:\ProgramData\ClaudeCode\managed-settings.json`
			systemDir := `C:\ProgramData\ClaudeCode`

			// 尝试创建目录和写入文件
			if err := os.MkdirAll(systemDir, 0755); err == nil {
				if err := os.WriteFile(systemPath, []byte(configContent), 0644); err == nil {
					i.addLog("✅ 已写入系统级配置（最高优先级）")
					systemConfigWritten = true
				}
			}
		} else if runtime.GOOS == "darwin" {
			systemPath := "/Library/Application Support/ClaudeCode/managed-settings.json"
			systemDir := "/Library/Application Support/ClaudeCode"

			// macOS 需要 sudo 权限
			if err := os.MkdirAll(systemDir, 0755); err == nil {
				if err := os.WriteFile(systemPath, []byte(configContent), 0644); err == nil {
					i.addLog("✅ 已写入系统级配置（最高优先级）")
					systemConfigWritten = true
				}
			}
		} else { // Linux
			systemPath := "/etc/claude-code/managed-settings.json"
			systemDir := "/etc/claude-code"

			// Linux 需要 sudo 权限
			if err := os.MkdirAll(systemDir, 0755); err == nil {
				if err := os.WriteFile(systemPath, []byte(configContent), 0644); err == nil {
					i.addLog("✅ 已写入系统级配置（最高优先级）")
					systemConfigWritten = true
				}
			}
		}

		if !systemConfigWritten {
			i.addLog("⚠️ 无法写入系统级配置（需要管理员权限），将使用用户级配置")
		}
	}

	// 根据操作系统设置配置
	if runtime.GOOS == "windows" {
		// Windows: 使用 Claude Code 标准配置目录
		claudeDir := filepath.Join(home, ".claude")
		os.MkdirAll(claudeDir, 0755)

		configPath := filepath.Join(claudeDir, "settings.json")
		config := fmt.Sprintf(`{
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.moonshot.cn/anthropic/",
    "ANTHROPIC_API_KEY": "%s",
    "CLAUDE_REQUEST_DELAY_MS": "%d",
    "CLAUDE_MAX_CONCURRENT_REQUESTS": "1"
  }
}`, apiKey, requestDelay)

		err = os.WriteFile(configPath, []byte(config), 0644)
		if err != nil {
			return fmt.Errorf("写入配置文件失败: %v", err)
		}

		i.addLog(fmt.Sprintf("已配置 Claude Code settings.json，请求延迟: %d毫秒", requestDelay))
	} else {
		// Mac/Linux: 使用 Claude Code 标准配置目录 + 环境变量
		claudeDir := filepath.Join(home, ".claude")
		os.MkdirAll(claudeDir, 0755)

		// 1. 创建 settings.json 配置文件
		configPath := filepath.Join(claudeDir, "settings.json")
		config := fmt.Sprintf(`{
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.moonshot.cn/anthropic/",
    "ANTHROPIC_API_KEY": "%s",
    "CLAUDE_REQUEST_DELAY_MS": "%d",
    "CLAUDE_MAX_CONCURRENT_REQUESTS": "1"
  }
}`, apiKey, requestDelay)

		err = os.WriteFile(configPath, []byte(config), 0644)
		if err != nil {
			return fmt.Errorf("写入配置文件失败: %v", err)
		}

		// 2. 同时设置环境变量作为备用
		shellConfig := ""
		shell := os.Getenv("SHELL")

		if strings.Contains(shell, "zsh") {
			shellConfig = filepath.Join(home, ".zshrc")
		} else if strings.Contains(shell, "bash") {
			shellConfig = filepath.Join(home, ".bashrc")
		} else {
			shellConfig = filepath.Join(home, ".profile")
		}

		envConfig := fmt.Sprintf(`
# Claude Code K2 Configuration
export ANTHROPIC_BASE_URL="https://api.moonshot.cn/anthropic/"
export ANTHROPIC_API_KEY="%s"
export CLAUDE_REQUEST_DELAY_MS="%d"
export CLAUDE_MAX_CONCURRENT_REQUESTS="1"
`, apiKey, requestDelay)

		// 追加到配置文件
		f, err := os.OpenFile(shellConfig, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("打开配置文件失败: %v", err)
		}
		defer f.Close()

		_, err = f.WriteString(envConfig)
		if err != nil {
			return fmt.Errorf("写入配置失败: %v", err)
		}

		i.addLog(fmt.Sprintf("已配置 Claude Code settings.json 和环境变量，请求延迟: %d毫秒", requestDelay))
		i.addLog(fmt.Sprintf("环境变量已添加到 %s", shellConfig))
	}

	i.addLog("K2 API 配置完成")
	return nil
}

func (i *Installer) verifyInstallation() error {
	i.addLog("验证安装...")

	// 验证 Node.js
	cmd := exec.Command("node", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Node.js 验证失败")
	}

	// 验证 Git
	cmd = exec.Command("git", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Git 验证失败")
	}

	// 验证 Claude Code
	cmd = exec.Command("claude", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Claude Code 验证失败")
	}

	i.addLog("所有组件验证通过！")
	return nil
}

func (i *Installer) downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (i *Installer) sendProgress(step, message string, percent float64) {
	i.Progress <- ProgressUpdate{
		Step:    step,
		Message: message,
		Percent: percent,
	}
}

func (i *Installer) sendError(err error) {
	i.Progress <- ProgressUpdate{
		Error: err,
	}
}

func (i *Installer) addLog(message string) {
	i.logs = append(i.logs, message)
}

func (i *Installer) GetLogs() []string {
	return i.logs
}

// ConfigureK2API 公开方法用于配置 API
func (i *Installer) ConfigureK2API(apiKey string) error {
	return i.configureK2API(apiKey)
}

// ConfigureK2APIWithRateLimit 配置 API 和速率限制
func (i *Installer) ConfigureK2APIWithRateLimit(apiKey string, rpm string) error {
	return i.configureK2APIWithOptions(apiKey, rpm, false)
}

// ConfigureK2APIWithOptions 配置 API 和速率限制，带系统级配置选项
func (i *Installer) ConfigureK2APIWithOptions(apiKey string, rpm string, useSystemConfig bool) error {
	return i.configureK2APIWithOptions(apiKey, rpm, useSystemConfig)
}

// RestoreOriginalClaudeConfig 恢复 Claude Code 的原始配置
func (i *Installer) RestoreOriginalClaudeConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %v", err)
	}

	i.addLog("开始恢复 Claude Code 原始配置...")

	// 1. 删除用户级配置
	claudeDir := filepath.Join(home, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		err = os.Remove(settingsPath)
		if err != nil {
			i.addLog(fmt.Sprintf("⚠️ 删除 settings.json 失败: %v", err))
		} else {
			i.addLog("✅ 已删除用户级配置 settings.json")
		}
	}

	// 2. 删除旧的 .claude.json (Windows兼容)
	oldConfigPath := filepath.Join(home, ".claude.json")
	if _, err := os.Stat(oldConfigPath); err == nil {
		err = os.Remove(oldConfigPath)
		if err != nil {
			i.addLog(fmt.Sprintf("⚠️ 删除 .claude.json 失败: %v", err))
		} else {
			i.addLog("✅ 已删除旧配置 .claude.json")
		}
	}

	// 3. 尝试删除系统级配置（需要管理员权限）
	if runtime.GOOS == "windows" {
		systemPath := `C:\ProgramData\ClaudeCode\managed-settings.json`
		if _, err := os.Stat(systemPath); err == nil {
			err = os.Remove(systemPath)
			if err != nil {
				i.addLog("⚠️ 无法删除系统级配置（需要管理员权限）")
			} else {
				i.addLog("✅ 已删除系统级配置")
			}
		}
	} else if runtime.GOOS == "darwin" {
		systemPath := "/Library/Application Support/ClaudeCode/managed-settings.json"
		if _, err := os.Stat(systemPath); err == nil {
			err = os.Remove(systemPath)
			if err != nil {
				i.addLog("⚠️ 无法删除系统级配置（需要管理员权限）")
			} else {
				i.addLog("✅ 已删除系统级配置")
			}
		}
	} else { // Linux
		systemPath := "/etc/claude-code/managed-settings.json"
		if _, err := os.Stat(systemPath); err == nil {
			err = os.Remove(systemPath)
			if err != nil {
				i.addLog("⚠️ 无法删除系统级配置（需要管理员权限）")
			} else {
				i.addLog("✅ 已删除系统级配置")
			}
		}
	}

	// 4. 清理环境变量（Mac/Linux）
	if runtime.GOOS != "windows" {
		// Mac/Linux: 删除环境变量配置
		shellConfig := ""
		shell := os.Getenv("SHELL")

		if strings.Contains(shell, "zsh") {
			shellConfig = filepath.Join(home, ".zshrc")
		} else if strings.Contains(shell, "bash") {
			shellConfig = filepath.Join(home, ".bashrc")
		} else {
			shellConfig = filepath.Join(home, ".profile")
		}

		// 读取文件内容
		if data, err := os.ReadFile(shellConfig); err == nil {
			content := string(data)

			// 移除 Claude Code K2 Configuration 部分
			lines := strings.Split(content, "\n")
			var newLines []string
			skipSection := false

			for _, line := range lines {
				if strings.Contains(line, "# Claude Code K2 Configuration") {
					skipSection = true
					continue
				}

				if skipSection {
					// 跳过以 export ANTHROPIC_ 或 export CLAUDE_ 开头的行
					if strings.HasPrefix(strings.TrimSpace(line), "export ANTHROPIC_") ||
						strings.HasPrefix(strings.TrimSpace(line), "export CLAUDE_") {
						continue
					}
					// 如果遇到空行或其他注释，结束跳过
					if strings.TrimSpace(line) == "" || (!strings.HasPrefix(strings.TrimSpace(line), "export") && strings.HasPrefix(strings.TrimSpace(line), "#")) {
						skipSection = false
					}
				}

				if !skipSection {
					newLines = append(newLines, line)
				}
			}

			// 写回文件
			newContent := strings.Join(newLines, "\n")
			err = os.WriteFile(shellConfig, []byte(newContent), 0644)
			if err != nil {
				return fmt.Errorf("恢复配置文件失败: %v", err)
			}
		}
	}

	i.addLog("Claude Code 配置已恢复到初始状态")
	return nil
}
