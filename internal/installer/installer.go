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

// Install 开始安装过程
func (i *Installer) Install() {
	// 安装完成后关闭 channel
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

// getHomebrewPrefix 获取 Homebrew 的安装前缀
func getHomebrewPrefix() string {
	// 尝试运行 brew --prefix
	cmd := exec.Command("brew", "--prefix")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}
	
	// 如果 brew 命令失败，检查常见位置
	if runtime.GOARCH == "arm64" {
		// Apple Silicon
		if _, err := os.Stat("/opt/homebrew"); err == nil {
			return "/opt/homebrew"
		}
	} else {
		// Intel Mac
		if _, err := os.Stat("/usr/local"); err == nil {
			return "/usr/local"
		}
	}
	
	return ""
}

func (i *Installer) checkNodeJS() error {
	// 首先尝试使用 which/where 命令查找 node
	var lookupCmd string
	var lookupArgs []string
	
	if runtime.GOOS == "windows" {
		lookupCmd = "where"
		lookupArgs = []string{"node"}
	} else {
		lookupCmd = "which"
		lookupArgs = []string{"node"}
	}
	
	// 使用 which/where 查找 node
	cmd := exec.Command(lookupCmd, lookupArgs...)
	lookupOutput, lookupErr := cmd.Output()
	
	if lookupErr == nil {
		// 找到了 node 命令的路径
		nodePath := strings.TrimSpace(string(lookupOutput))
		if nodePath != "" {
			// Windows 的 where 命令可能返回多行，取第一行
			lines := strings.Split(nodePath, "\n")
			if len(lines) > 0 {
				nodePath = strings.TrimSpace(lines[0])
			}
			i.addLog(fmt.Sprintf("通过 %s 找到 Node.js: %s", lookupCmd, nodePath))
		}
	}
	
	// 尝试直接执行 node 命令
	cmd = exec.Command("node", "--version")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		// 如果失败，显示更详细的错误信息
		i.addLog(fmt.Sprintf("执行 'node --version' 失败: %v", err))
		if len(output) > 0 {
			i.addLog(fmt.Sprintf("错误输出: %s", string(output)))
		}
		
		// 检查 PATH 环境变量
		pathEnv := os.Getenv("PATH")
		i.addLog(fmt.Sprintf("当前 PATH: %s", pathEnv))
		
		// macOS 特殊处理：检查常见的安装位置
		if runtime.GOOS == "darwin" {
			i.addLog("正在检查 macOS 常见的 Node.js 安装位置...")
			commonPaths := []string{
				"/opt/homebrew/bin/node",     // Apple Silicon Homebrew
				"/usr/local/bin/node",         // Intel Homebrew
				"/usr/bin/node",               // 系统默认
			}
			
			for _, path := range commonPaths {
				if _, err := os.Stat(path); err == nil {
					i.addLog(fmt.Sprintf("发现 Node.js 在: %s", path))
					// 尝试运行找到的 node
					testCmd := exec.Command(path, "--version")
					if testOutput, testErr := testCmd.Output(); testErr == nil {
						version := strings.TrimSpace(string(testOutput))
						i.addLog(fmt.Sprintf("版本: %s", version))
						
						// 将目录添加到当前进程的 PATH 中
						nodeDir := filepath.Dir(path)
						currentPath := os.Getenv("PATH")
						newPath := nodeDir + ":" + currentPath
						os.Setenv("PATH", newPath)
						i.addLog(fmt.Sprintf("已将 %s 添加到 PATH 环境变量", nodeDir))
						
						// 重新检查版本
						if checkErr := i.validateNodeVersion(version); checkErr == nil {
							i.addLog("✅ Node.js 检测成功")
							return nil
						}
					}
				}
			}
		}
		
		i.addLog("未检测到 Node.js，需要安装")
		return fmt.Errorf("未安装 Node.js")
	}
	
	version := strings.TrimSpace(string(output))
	i.addLog(fmt.Sprintf("检测到 Node.js: %s", version))

	return i.validateNodeVersion(version)
}

// validateNodeVersion 验证Node.js版本是否满足要求
func (i *Installer) validateNodeVersion(version string) error {
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
	// 首先尝试使用 which/where 命令查找 git
	var lookupCmd string
	var lookupArgs []string
	
	if runtime.GOOS == "windows" {
		lookupCmd = "where"
		lookupArgs = []string{"git"}
	} else {
		lookupCmd = "which"
		lookupArgs = []string{"git"}
	}
	
	// 尝试查找 git 命令
	if lookupOutput, lookupErr := exec.Command(lookupCmd, lookupArgs...).Output(); lookupErr == nil {
		gitPath := strings.TrimSpace(string(lookupOutput))
		if gitPath != "" {
			i.addLog(fmt.Sprintf("找到 Git 在: %s", gitPath))
		}
	}

	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()

	if err == nil {
		version := strings.TrimSpace(string(output))
		i.addLog(fmt.Sprintf("检测到 Git: %s", version))
		return nil
	}

	// macOS 特殊处理：检查常见的安装位置
	if runtime.GOOS == "darwin" {
		i.addLog("正在检查 macOS 常见的 Git 安装位置...")
		commonPaths := []string{
			"/opt/homebrew/bin/git",      // Apple Silicon Homebrew
			"/usr/local/bin/git",         // Intel Homebrew
			"/usr/bin/git",               // 系统默认
		}
		
		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				i.addLog(fmt.Sprintf("发现 Git 在: %s", path))
				// 尝试运行找到的 git
				testCmd := exec.Command(path, "--version")
				if testOutput, testErr := testCmd.Output(); testErr == nil {
					version := strings.TrimSpace(string(testOutput))
					i.addLog(fmt.Sprintf("版本: %s", version))
					
					// 将目录添加到当前进程的 PATH 中
					gitDir := filepath.Dir(path)
					currentPath := os.Getenv("PATH")
					newPath := gitDir + ":" + currentPath
					os.Setenv("PATH", newPath)
					i.addLog(fmt.Sprintf("已将 %s 添加到 PATH 环境变量", gitDir))
					i.addLog("✅ Git 检测成功")
					return nil
				}
			}
		}
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

	// 配置内容 - 只使用 API KEY，避免认证冲突
	// useSystemConfig 参数现在用于决定是否设置永久环境变量
	// true: 设置永久环境变量（写入配置文件/注册表）
	// false: 仅显示临时设置命令

	// 根据操作系统设置配置
	if runtime.GOOS == "windows" {
		if useSystemConfig {
			// Windows: 设置永久环境变量
			i.addLog("设置 Windows 永久环境变量...")
			envVars := map[string]string{
				"ANTHROPIC_BASE_URL": "https://api.moonshot.cn/anthropic/",
				"ANTHROPIC_API_KEY": apiKey,
				"CLAUDE_REQUEST_DELAY_MS": fmt.Sprintf("%d", requestDelay),
				"CLAUDE_MAX_CONCURRENT_REQUESTS": "1",
			}
			
			for envVar, value := range envVars {
				// 设置用户级环境变量（使用 setx）
				cmd := exec.Command("setx", envVar, value)
				err := cmd.Run()
				if err != nil {
					i.addLog(fmt.Sprintf("⚠️ 设置环境变量 %s 失败: %v", envVar, err))
				} else {
					i.addLog(fmt.Sprintf("✅ 已设置用户环境变量: %s", envVar))
				}
			}
			
			i.addLog(fmt.Sprintf("永久环境变量已设置（请求延迟: %d毫秒），可能需要重启终端才能生效", requestDelay))
		} else {
			// 创建临时批处理脚本设置环境变量
			i.addLog("正在创建临时环境变量脚本...")
			
			// 获取临时目录
			tempDir := os.TempDir()
			scriptPath := filepath.Join(tempDir, "claude_k2_setup.bat")
			// 改用PowerShell脚本，避免set命令卡死问题
			scriptPath = filepath.Join(tempDir, "claude_k2_setup.ps1")
			scriptContent := fmt.Sprintf(`# Claude Code K2 临时环境变量设置脚本
$env:ANTHROPIC_BASE_URL="https://api.moonshot.cn/anthropic/"
$env:ANTHROPIC_API_KEY="%s"
$env:CLAUDE_REQUEST_DELAY_MS="%d"
$env:CLAUDE_MAX_CONCURRENT_REQUESTS="1"
$env:ANTHROPIC_AUTH_TOKEN=""

Write-Host "✅ K2环境变量已设置：" -ForegroundColor Green
Write-Host "  - API Key: %s..." -ForegroundColor Yellow
Write-Host "  - Base URL: https://api.moonshot.cn/anthropic/" -ForegroundColor Yellow
Write-Host "  - 请求延迟: %d毫秒" -ForegroundColor Yellow
Write-Host ""
Write-Host "现在可以运行 'claude' 命令使用K2 API" -ForegroundColor Cyan
`, apiKey, requestDelay, apiKey[:10], requestDelay)

			err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
			if err != nil {
				i.addLog(fmt.Sprintf("⚠️ 创建临时脚本失败: %v", err))
			} else {
				i.addLog(fmt.Sprintf("✅ 临时环境变量脚本已创建: %s", scriptPath))
				i.addLog("  打开Claude Code时将自动运行此脚本设置环境变量")
			}
		}
	} else {
		// Mac/Linux: 只设置环境变量，不写入 settings.json
		if useSystemConfig {
			// 设置永久环境变量
			shell := os.Getenv("SHELL")
			shellConfigs := []string{}
			
			// 根据 shell 类型确定配置文件
			if strings.Contains(shell, "zsh") {
				shellConfigs = append(shellConfigs, filepath.Join(home, ".zshrc"))
			} else if strings.Contains(shell, "bash") {
				// bash 在 macOS 上通常使用 .bash_profile，在 Linux 上使用 .bashrc
				if runtime.GOOS == "darwin" {
					shellConfigs = append(shellConfigs, filepath.Join(home, ".bash_profile"))
				} else {
					shellConfigs = append(shellConfigs, filepath.Join(home, ".bashrc"))
				}
			} else if strings.Contains(shell, "fish") {
				shellConfigs = append(shellConfigs, filepath.Join(home, ".config/fish/config.fish"))
			} else {
				// 默认使用 .profile
				shellConfigs = append(shellConfigs, filepath.Join(home, ".profile"))
			}
			
			// 对每个配置文件进行处理
			for _, shellConfig := range shellConfigs {
				envConfig := fmt.Sprintf(`
# Claude Code K2 Configuration
export ANTHROPIC_BASE_URL="https://api.moonshot.cn/anthropic/"
export ANTHROPIC_API_KEY="%s"
export CLAUDE_REQUEST_DELAY_MS="%d"
export CLAUDE_MAX_CONCURRENT_REQUESTS="1"
unset ANTHROPIC_AUTH_TOKEN
`, apiKey, requestDelay)

				// 检查文件是否存在
				if _, err := os.Stat(shellConfig); os.IsNotExist(err) {
					// 文件不存在，跳过
					continue
				}
				
				// 检查配置是否已存在
				existingData, err := os.ReadFile(shellConfig)
				if err != nil {
					i.addLog(fmt.Sprintf("⚠️ 读取 %s 失败: %v", shellConfig, err))
					continue
				}
				
				if strings.Contains(string(existingData), "# Claude Code K2 Configuration") {
					i.addLog(fmt.Sprintf("⚠️ %s 中已存在配置，跳过", shellConfig))
					continue
				}

				// 追加到配置文件
				f, err := os.OpenFile(shellConfig, os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					i.addLog(fmt.Sprintf("⚠️ 无法打开 %s: %v", shellConfig, err))
					continue
				}
				
				_, err = f.WriteString(envConfig)
				f.Close()
				
				if err != nil {
					i.addLog(fmt.Sprintf("⚠️ 写入 %s 失败: %v", shellConfig, err))
				} else {
					i.addLog(fmt.Sprintf("✅ 永久环境变量已添加到 %s", shellConfig))
				}
			}
			
			i.addLog(fmt.Sprintf("永久环境变量已设置（请求延迟: %d毫秒），请重新打开终端或运行 source 命令生效", requestDelay))
		} else {
			// 创建临时脚本设置环境变量
			i.addLog("正在创建临时环境变量脚本...")
			
			// 创建临时脚本文件
			scriptPath := "/tmp/claude_k2_setup.sh"
			scriptContent := fmt.Sprintf(`#!/bin/bash
# Claude Code K2 临时环境变量设置脚本
export ANTHROPIC_BASE_URL="https://api.moonshot.cn/anthropic/"
export ANTHROPIC_API_KEY="%s"
export CLAUDE_REQUEST_DELAY_MS="%d"
export CLAUDE_MAX_CONCURRENT_REQUESTS="1"
unset ANTHROPIC_AUTH_TOKEN

echo "✅ K2环境变量已设置："
echo "  - API Key: %s..."
echo "  - Base URL: https://api.moonshot.cn/anthropic/"
echo "  - 请求延迟: %d毫秒"
echo ""
echo "现在可以运行 'claude' 命令使用K2 API"
`, apiKey, requestDelay, apiKey[:10], requestDelay)

			err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
			if err != nil {
				i.addLog(fmt.Sprintf("⚠️ 创建临时脚本失败: %v", err))
			} else {
				i.addLog(fmt.Sprintf("✅ 临时环境变量脚本已创建: %s", scriptPath))
				i.addLog("  打开Claude Code时将自动运行此脚本设置环境变量")
			}
		}
	}

	// 创建 .claude.json 文件以跳过登录
	claudeJsonPath := filepath.Join(home, ".claude.json")
	claudeJson := `{
  "hasCompletedOnboarding": true
}`
	err = os.WriteFile(claudeJsonPath, []byte(claudeJson), 0644)
	if err != nil {
		i.addLog(fmt.Sprintf("⚠️ 创建 .claude.json 失败: %v", err))
	} else {
		i.addLog("✅ 已创建 .claude.json 以跳过登录流程")
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
	
	// 删除 .claude.json 文件
	claudeJsonPath := filepath.Join(home, ".claude.json")
	if _, err := os.Stat(claudeJsonPath); err == nil {
		err = os.Remove(claudeJsonPath)
		if err != nil {
			i.addLog(fmt.Sprintf("⚠️ 删除 .claude.json 失败: %v", err))
		} else {
			i.addLog("✅ 已删除 .claude.json")
		}
	}
	
	// 删除 ~/.claude/settings.json 文件
	claudeDir := filepath.Join(home, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		err = os.Remove(settingsPath)
		if err != nil {
			i.addLog(fmt.Sprintf("⚠️ 删除 settings.json 失败: %v", err))
		} else {
			i.addLog("✅ 已删除 ~/.claude/settings.json")
		}
	}

	// 清理环境变量配置
	if runtime.GOOS == "windows" {
		// Windows: 清除永久环境变量
		i.addLog("清除 Windows 环境变量...")
		envVars := []string{
			"ANTHROPIC_BASE_URL",
			"ANTHROPIC_API_KEY", 
			"ANTHROPIC_AUTH_TOKEN",
			"CLAUDE_REQUEST_DELAY_MS",
			"CLAUDE_MAX_CONCURRENT_REQUESTS",
		}
		
		for _, envVar := range envVars {
			// 清除用户级环境变量
			cmd := exec.Command("reg", "delete", "HKCU\\Environment", "/v", envVar, "/f")
			err := cmd.Run()
			if err == nil {
				i.addLog(fmt.Sprintf("✅ 已清除用户环境变量: %s", envVar))
			}
			
			// 尝试清除系统级环境变量（需要管理员权限）
			cmd = exec.Command("reg", "delete", "HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment", "/v", envVar, "/f")
			err = cmd.Run()
			if err == nil {
				i.addLog(fmt.Sprintf("✅ 已清除系统环境变量: %s", envVar))
			}
		}
		
		// 广播环境变量更改消息
		i.addLog("刷新 Windows 环境变量...")
		cmd := exec.Command("setx", "CLAUDE_REFRESH", "1")
		cmd.Run()
		// 删除临时变量
		cmd = exec.Command("reg", "delete", "HKCU\\Environment", "/v", "CLAUDE_REFRESH", "/f")
		cmd.Run()
		
	} else {
		// Mac/Linux: 清除永久环境变量
		// Mac/Linux: 删除环境变量配置
		shell := os.Getenv("SHELL")
		shellConfigs := []string{}
		
		// 根据 shell 类型确定配置文件
		if strings.Contains(shell, "zsh") {
			shellConfigs = append(shellConfigs, filepath.Join(home, ".zshrc"))
		} else if strings.Contains(shell, "bash") {
			// bash 可能使用多个配置文件
			shellConfigs = append(shellConfigs, 
				filepath.Join(home, ".bashrc"),
				filepath.Join(home, ".bash_profile"),
			)
		} else if strings.Contains(shell, "fish") {
			shellConfigs = append(shellConfigs, filepath.Join(home, ".config/fish/config.fish"))
		}
		
		// 总是检查 .profile 作为后备
		shellConfigs = append(shellConfigs, filepath.Join(home, ".profile"))
		
		// 清理所有找到的配置文件
		for _, shellConfig := range shellConfigs {
			if _, err := os.Stat(shellConfig); err != nil {
				continue // 文件不存在，跳过
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
					i.addLog(fmt.Sprintf("⚠️ 恢复 %s 失败: %v", shellConfig, err))
				} else {
					i.addLog(fmt.Sprintf("✅ 已清理 %s 中的配置", shellConfig))
				}
			}
		}
	}

	i.addLog("Claude Code 配置已恢复到初始状态")
	return nil
}
