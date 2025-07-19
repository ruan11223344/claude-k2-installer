package installer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Installer struct {
	Progress chan ProgressUpdate
	logs     []string
	closed   bool       // 标记channel是否已关闭
	mu       sync.Mutex // 保护closed字段
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
	defer func() {
		i.mu.Lock()
		i.closed = true
		i.mu.Unlock()
		close(i.Progress)
	}()

	steps := []struct {
		name         string
		fn           func() error
		weight       float64
		allowFailure bool // 允许失败并继续的标志
	}{
		{"检查系统环境", i.checkSystem, 5, false},
		{"检测 Node.js", i.checkNodeJS, 10, true}, // 允许检测失败，因为后面会安装
		{"安装 Node.js", i.installNodeJS, 20, false},
		{"检测 Git", i.checkGit, 10, true}, // 允许检测失败，因为后面会安装
		{"安装 Git", i.installGit, 20, false},
		{"安装 Claude Code", i.installClaudeCode, 20, false},
		{"验证安装", i.verifyInstallation, 5, false},
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
			if step.allowFailure {
				// 对于允许失败的步骤，记录但继续执行
				i.addLog(fmt.Sprintf("⚠️ %s失败，继续下一步: %v", step.name, err))
				i.sendProgress(step.name, fmt.Sprintf("%s未通过，继续安装", step.name), currentProgress/totalWeight)
			} else {
				// 对于不允许失败的步骤，停止安装
				i.sendProgress(step.name, fmt.Sprintf("%s失败: %v", step.name, err), currentProgress/totalWeight)
				i.sendError(fmt.Errorf("%s失败: %v", step.name, err))
				return
			}
		} else {
			i.sendProgress(step.name, fmt.Sprintf("%s完成", step.name), currentProgress/totalWeight)
		}

		currentProgress += step.weight
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

		// Windows 特殊处理：检查常见的安装位置
		if runtime.GOOS == "windows" {
			i.addLog("正在检查 Windows 常见的 Node.js 安装位置...")

			// 先检查PATH中的nodejs目录
			pathDirs := strings.Split(pathEnv, ";")
			for _, dir := range pathDirs {
				dir = strings.TrimSpace(dir)
				if strings.Contains(strings.ToLower(dir), "nodejs") {
					nodeExe := filepath.Join(dir, "node.exe")
					i.addLog(fmt.Sprintf("检查PATH中的目录: %s", dir))
					if _, err := os.Stat(nodeExe); err == nil {
						i.addLog(fmt.Sprintf("✅ 找到 node.exe: %s", nodeExe))
						// 尝试运行
						testCmd := exec.Command(nodeExe, "--version")
						if testOutput, testErr := testCmd.Output(); testErr == nil {
							version := strings.TrimSpace(string(testOutput))
							i.addLog(fmt.Sprintf("版本: %s", version))
							return i.validateNodeVersion(version)
						} else {
							i.addLog(fmt.Sprintf("⚠️ 无法执行 %s: %v", nodeExe, testErr))
						}
					} else {
						i.addLog(fmt.Sprintf("❌ 目录存在但找不到 node.exe: %s", dir))
						i.addLog("这可能是之前安装的残留环境变量")
					}
				}
			}

			// 再检查标准安装位置
			commonPaths := []string{
				`C:\Program Files\nodejs\node.exe`,
				`C:\Program Files (x86)\nodejs\node.exe`,
				filepath.Join(os.Getenv("ProgramFiles"), "nodejs", "node.exe"),
				filepath.Join(os.Getenv("ProgramFiles(x86)"), "nodejs", "node.exe"),
			}

			for _, path := range commonPaths {
				if _, err := os.Stat(path); err == nil {
					i.addLog(fmt.Sprintf("发现 Node.js 在: %s", path))
					// 尝试运行找到的 node
					testCmd := exec.Command(path, "--version")
					if testOutput, testErr := testCmd.Output(); testErr == nil {
						version := strings.TrimSpace(string(testOutput))
						i.addLog(fmt.Sprintf("版本: %s", version))
						return i.validateNodeVersion(version)
					}
				}
			}
		}

		// macOS 特殊处理：检查常见的安装位置
		if runtime.GOOS == "darwin" {
			i.addLog("正在检查 macOS 常见的 Node.js 安装位置...")
			commonPaths := []string{
				"/opt/homebrew/bin/node", // Apple Silicon Homebrew
				"/usr/local/bin/node",    // Intel Homebrew
				"/usr/bin/node",          // 系统默认
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
	i.addLog("开始 Node.js 安装流程...")

	tempDir := os.TempDir()
	scriptPath := filepath.Join(tempDir, "install_nodejs.bat")

	// 创建批处理脚本内容
	scriptContent := `@echo off
echo Starting Node.js installation...

set "NODE_URL1=https://mirrors.aliyun.com/nodejs-release/v20.10.0/node-v20.10.0-x64.msi"
set "NODE_URL2=https://cdn.npmmirror.com/binaries/node/v20.10.0/node-v20.10.0-x64.msi"
set "NODE_URL3=https://nodejs.org/dist/v20.10.0/node-v20.10.0-x64.msi"
set "INSTALLER_PATH=%TEMP%\node-installer.msi"

echo [STEP 1] Cleaning up old installations...
taskkill /F /IM node.exe >nul 2>&1
if exist "C:\Program Files\nodejs" (
    rmdir /s /q "C:\Program Files\nodejs" 2>nul
)

echo [STEP 2] Downloading Node.js...
echo Trying mirror 1...
powershell -Command "try { $ProgressPreference='SilentlyContinue'; Invoke-WebRequest -Uri '%NODE_URL1%' -OutFile '%INSTALLER_PATH%' -TimeoutSec 60 -UseBasicParsing } catch { exit 1 }"
if %ERRORLEVEL% EQU 0 (
    echo Download successful from mirror 1
    goto :install
)

echo Trying mirror 2...
powershell -Command "try { $ProgressPreference='SilentlyContinue'; Invoke-WebRequest -Uri '%NODE_URL2%' -OutFile '%INSTALLER_PATH%' -TimeoutSec 60 -UseBasicParsing } catch { exit 1 }"
if %ERRORLEVEL% EQU 0 (
    echo Download successful from mirror 2
    goto :install
)

echo Trying mirror 3...
powershell -Command "try { $ProgressPreference='SilentlyContinue'; Invoke-WebRequest -Uri '%NODE_URL3%' -OutFile '%INSTALLER_PATH%' -TimeoutSec 60 -UseBasicParsing } catch { exit 1 }"
if %ERRORLEVEL% EQU 0 (
    echo Download successful from mirror 3
    goto :install
)

echo ERROR: All download attempts failed
exit /b 1

:install
echo [STEP 3] Installing Node.js...
msiexec /i "%INSTALLER_PATH%" /qn /norestart ADDLOCAL=ALL ALLUSERS=1
set INSTALL_RESULT=%ERRORLEVEL%

if %INSTALL_RESULT% NEQ 0 (
    echo ERROR: Installation failed with code %INSTALL_RESULT%
    
    if %INSTALL_RESULT% EQU 1603 (
        echo.
        echo Error 1603 usually means:
        echo - Another installation is in progress
        echo - Need administrator permissions
        echo - Windows Installer service issues
        echo.
        echo Please try:
        echo 1. Run installer as Administrator
        echo 2. Restart computer and try again
        echo 3. Check Windows Update
    )
    
    if %INSTALL_RESULT% EQU 1638 (
        echo.
        echo Error 1638: Another version is already installed
        echo Please uninstall existing Node.js first
    )
    
    del /f /q "%INSTALLER_PATH%" 2>nul
    exit /b %INSTALL_RESULT%
)

echo Installation completed
del /f /q "%INSTALLER_PATH%" 2>nul

echo [STEP 4] Verifying installation...
ping 127.0.0.1 -n 3 >nul

where node >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    for /f "tokens=*" %%i in ('node --version 2^>nul') do echo Node.js installed successfully: %%i
    exit /b 0
)

if exist "C:\Program Files\nodejs\node.exe" (
    "C:\Program Files\nodejs\node.exe" --version >nul 2>&1
    if %ERRORLEVEL% EQU 0 (
        echo Node.js installed at: C:\Program Files\nodejs
        echo You may need to restart terminal to use 'node' command
        exit /b 0
    )
)

echo WARNING: Installation completed but Node.js not found in PATH
echo Please restart your terminal or computer
exit /b 0
`

	// 写入脚本文件（使用UTF-8编码）
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("创建安装脚本失败: %v", err)
	}
	defer os.Remove(scriptPath)

	i.addLog(fmt.Sprintf("执行安装脚本: %s", scriptPath))

	// 执行批处理脚本 - 使用流式输出避免UI卡住
	cmd := exec.Command("cmd", "/c", scriptPath)
	cmd.Dir = tempDir

	// 设置输出编码为UTF-8
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	// 使用流式执行避免UI卡住
	err = i.executeCommandWithStreaming(cmd)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			switch code {
			case 1603:
				return fmt.Errorf("Node.js 安装失败 (1603): 致命错误。可能需要管理员权限或重启系统")
			case 1638:
				return fmt.Errorf("Node.js 安装失败 (1638): 已安装其他版本。请先卸载现有版本")
			default:
				return fmt.Errorf("Node.js 安装失败，错误代码: %d", code)
			}
		}
		return fmt.Errorf("Node.js 安装失败: %v", err)
	}

	// 再次验证安装
	if err := i.checkNodeJS(); err == nil {
		i.addLog("✅ Node.js 安装并验证成功！")
		return nil
	}

	// 如果验证失败，但安装脚本成功，说明可能需要重启
	i.addLog("⚠️ Node.js 已安装，但可能需要重启终端或系统才能生效")

	// 尝试设置临时环境变量
	possiblePaths := []string{
		`C:\Program Files\nodejs`,
		`C:\Program Files (x86)\nodejs`,
		filepath.Join(os.Getenv("ProgramFiles"), "nodejs"),
	}

	for _, nodePath := range possiblePaths {
		nodeExe := filepath.Join(nodePath, "node.exe")
		if _, err := os.Stat(nodeExe); err == nil {
			os.Setenv("PATH", fmt.Sprintf("%s;%s", nodePath, os.Getenv("PATH")))
			i.addLog(fmt.Sprintf("已将 %s 添加到临时PATH", nodePath))
			break
		}
	}

	return nil
}

func (i *Installer) installNodeJSMac() error {
	// 检查是否有 Homebrew
	cmd := exec.Command("brew", "--version")
	if err := cmd.Run(); err != nil {
		i.addLog("未检测到 Homebrew，开始自动安装...")
		
		// 自动安装 Homebrew
		if err := i.installHomebrewCN(); err != nil {
			i.addLog(fmt.Sprintf("Homebrew 安装失败: %v", err))
			i.addLog("将使用备用方案直接下载 Node.js 安装包")
			return i.installNodeJSMacPkg()
		}
		
		// 重新检查 Homebrew 是否安装成功
		cmd = exec.Command("brew", "--version")
		if err := cmd.Run(); err != nil {
			i.addLog("Homebrew 安装后仍无法使用，使用备用方案...")
			return i.installNodeJSMacPkg()
		}
		
		i.addLog("✅ Homebrew 安装成功！")
	}

	i.addLog("配置 Homebrew 使用中国镜像源并安装 Node.js...")
	
	// 创建配置脚本
	tempDir := os.TempDir()
	brewScriptPath := filepath.Join(tempDir, "brew_install_nodejs.sh")
	
	brewScript := `#!/bin/bash
# 保存用户原有的 Homebrew 配置
OLD_HOMEBREW_BREW_GIT_REMOTE="$HOMEBREW_BREW_GIT_REMOTE"
OLD_HOMEBREW_CORE_GIT_REMOTE="$HOMEBREW_CORE_GIT_REMOTE"
OLD_HOMEBREW_BOTTLE_DOMAIN="$HOMEBREW_BOTTLE_DOMAIN"

# 检查是否已经配置了镜像源
if [[ -n "$HOMEBREW_BOTTLE_DOMAIN" ]]; then
    echo "检测到已配置 Homebrew 镜像源: $HOMEBREW_BOTTLE_DOMAIN"
    echo "将使用现有配置..."
else
    # 只有在没有配置的情况下才设置镜像源
    echo "配置 Homebrew 使用中国科技大学镜像源..."
    export HOMEBREW_BREW_GIT_REMOTE="https://mirrors.ustc.edu.cn/brew.git"
    export HOMEBREW_CORE_GIT_REMOTE="https://mirrors.ustc.edu.cn/homebrew-core.git"
    export HOMEBREW_BOTTLE_DOMAIN="https://mirrors.ustc.edu.cn/homebrew-bottles"
    echo "HOMEBREW_BOTTLE_DOMAIN=$HOMEBREW_BOTTLE_DOMAIN"
fi

# 更新并安装 Node.js
echo "更新 Homebrew..."
brew update || echo "更新失败，继续尝试安装..."

echo "安装 Node.js..."
brew install node

# 验证安装
if node --version >/dev/null 2>&1; then
    NODE_VERSION=$(node --version)
    echo "Node.js installed successfully: $NODE_VERSION"
    exit 0
else
    echo "Node.js installation may have failed"
    exit 1
fi
`
	
	err := os.WriteFile(brewScriptPath, []byte(brewScript), 0755)
	if err != nil {
		return fmt.Errorf("创建 Homebrew 脚本失败: %v", err)
	}
	defer os.Remove(brewScriptPath)
	
	cmd = exec.Command("bash", brewScriptPath)
	cmd.Dir = tempDir
	
	// 使用流式执行避免UI卡住
	if err := i.executeCommandWithStreaming(cmd); err != nil {
		i.addLog("Homebrew 安装失败，尝试直接下载安装包...")
		return i.installNodeJSMacPkg()
	}
	
	return nil
}

// installHomebrewCN 使用国内镜像安装 Homebrew
func (i *Installer) installHomebrewCN() error {
	i.addLog("准备安装 Homebrew（使用国内镜像）...")
	i.addLog("⚠️  安装需要管理员权限，系统将弹出密码输入框")
	
	tempDir := os.TempDir()
	scriptPath := filepath.Join(tempDir, "install_homebrew.sh")
	
	// 创建安装脚本
	scriptContent := `#!/bin/bash
echo "开始安装 Homebrew..."

# 检查是否已经安装
if command -v brew >/dev/null 2>&1; then
    echo "Homebrew 已经安装"
    brew --version
    exit 0
fi

# 使用国内镜像安装
/bin/zsh -c "$(curl -fsSL https://gitee.com/cunkai/HomebrewCN/raw/master/Homebrew.sh)"

# 检查安装结果
if command -v brew >/dev/null 2>&1; then
    echo "Homebrew 安装成功！"
    brew --version
    exit 0
else
    # 尝试为不同的安装路径设置 PATH
    if [ -f "/opt/homebrew/bin/brew" ]; then
        eval "$(/opt/homebrew/bin/brew shellenv)"
    elif [ -f "/usr/local/bin/brew" ]; then
        eval "$(/usr/local/bin/brew shellenv)"
    fi
    
    # 再次检查
    if command -v brew >/dev/null 2>&1; then
        echo "Homebrew 安装成功！"
        brew --version
        exit 0
    else
        echo "Homebrew 安装失败或需要重启终端"
        exit 1
    fi
fi
`

	// 写入脚本文件
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("创建安装脚本失败: %v", err)
	}
	defer os.Remove(scriptPath)

	// 使用 osascript 以管理员权限执行
	// 这会弹出系统的密码输入对话框
	executeScript := fmt.Sprintf(`do shell script "bash %s 2>&1" with administrator privileges`, scriptPath)
	cmd := exec.Command("osascript", "-e", executeScript)
	
	// 执行并获取输出
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		// 将输出按行分割并添加到日志
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				i.addLog(line)
			}
		}
	}
	
	if err != nil {
		// 如果用户取消了密码输入，会返回错误
		if strings.Contains(err.Error(), "User canceled") {
			return fmt.Errorf("用户取消了密码输入")
		}
		return fmt.Errorf("安装失败: %v", err)
	}

	// 设置 PATH 环境变量
	if _, err := os.Stat("/opt/homebrew/bin/brew"); err == nil {
		os.Setenv("PATH", fmt.Sprintf("/opt/homebrew/bin:%s", os.Getenv("PATH")))
		i.addLog("已添加 /opt/homebrew/bin 到 PATH")
	} else if _, err := os.Stat("/usr/local/bin/brew"); err == nil {
		os.Setenv("PATH", fmt.Sprintf("/usr/local/bin:%s", os.Getenv("PATH")))
		i.addLog("已添加 /usr/local/bin 到 PATH")
	}

	return nil
}

func (i *Installer) installNodeJSMacPkg() error {
	i.addLog("准备下载并安装 Node.js...")

	tempDir := os.TempDir()
	installerPath := filepath.Join(tempDir, "node-installer.pkg")
	scriptPath := filepath.Join(tempDir, "install_nodejs.sh")

	// 创建下载脚本，支持多个镜像源
	scriptContent := fmt.Sprintf(`#!/bin/bash
set -e

INSTALLER_PATH="%s"

echo "[STEP 1] Starting Node.js download..."

# Mirror URLs
MIRRORS=(
    "https://cdn.npmmirror.com/binaries/node/v20.10.0/node-v20.10.0.pkg"
    "https://nodejs.org/dist/v20.10.0/node-v20.10.0.pkg"
)

# Try each mirror
for i in "${!MIRRORS[@]}"; do
    MIRROR="${MIRRORS[$i]}"
    echo "[STEP 2] Trying mirror $((i+1)): ${MIRROR}"
    
    if curl -L --connect-timeout 10 --max-time 300 -o "$INSTALLER_PATH" "$MIRROR" 2>&1; then
        echo "[STEP 3] Download successful from mirror $((i+1))"
        break
    else
        echo "Mirror $((i+1)) failed, trying next..."
        rm -f "$INSTALLER_PATH"
        if [ $i -eq $((${#MIRRORS[@]}-1)) ]; then
            echo "ERROR: All mirrors failed"
            exit 1
        fi
    fi
done

# Verify download
if [ ! -f "$INSTALLER_PATH" ]; then
    echo "ERROR: Download failed - file not found"
    exit 1
fi

FILE_SIZE=$(stat -f%%z "$INSTALLER_PATH" 2>/dev/null || stat -c%%s "$INSTALLER_PATH" 2>/dev/null || echo 0)
echo "[STEP 4] Downloaded file size: $((FILE_SIZE / 1024 / 1024)) MB"

if [ "$FILE_SIZE" -lt 1000000 ]; then
    echo "ERROR: Downloaded file too small, possibly corrupted"
    exit 1
fi

echo "[STEP 5] Node.js installation ready"
echo "Installation will be performed with administrator privileges"

# 保存安装器路径到临时文件，供 osascript 使用
echo "$INSTALLER_PATH" > /tmp/nodejs_installer_path.txt
exit 0
`, installerPath)

	// 写入脚本文件
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("创建安装脚本失败: %v", err)
	}
	defer os.Remove(scriptPath)
	defer os.Remove(installerPath)

	i.addLog(fmt.Sprintf("执行安装脚本: %s", scriptPath))

	// 使用流式执行，支持实时输出
	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = tempDir

	// 使用流式执行下载
	err = i.executeCommandWithStreaming(cmd)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("Node.js 下载失败，退出代码: %d", exitErr.ExitCode())
		}
		return fmt.Errorf("Node.js 下载失败: %v", err)
	}
	
	// 读取安装器路径
	installerPathBytes, err := os.ReadFile("/tmp/nodejs_installer_path.txt")
	if err == nil {
		installerPath = strings.TrimSpace(string(installerPathBytes))
		os.Remove("/tmp/nodejs_installer_path.txt")
	}
	
	// 检查安装包是否存在
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("安装包不存在: %s", installerPath)
	}
	
	i.addLog("正在安装 Node.js...")
	i.addLog("⚠️  系统将弹出密码输入框，请输入您的密码")
	
	// 使用 osascript 以管理员权限安装
	installScript := fmt.Sprintf(`do shell script "installer -pkg '%s' -target /" with administrator privileges`, installerPath)
	installCmd := exec.Command("osascript", "-e", installScript)
	
	output, err := installCmd.CombinedOutput()
	if err != nil {
		// 如果用户取消了密码输入
		if strings.Contains(err.Error(), "User canceled") {
			return fmt.Errorf("用户取消了密码输入")
		}
		return fmt.Errorf("Node.js 安装失败: %v\n输出: %s", err, string(output))
	}
	
	i.addLog("✅ Node.js 安装完成！")

	// 再次验证安装
	if err := i.checkNodeJS(); err == nil {
		i.addLog("✅ Node.js 安装并验证成功！")
		return nil
	}

	// 如果验证失败，但安装脚本成功，说明可能需要重启终端
	i.addLog("⚠️ Node.js 已安装，但可能需要重启终端才能生效")
	
	// 尝试添加到当前进程的PATH
	os.Setenv("PATH", fmt.Sprintf("/usr/local/bin:%s", os.Getenv("PATH")))
	
	return nil
}

func (i *Installer) installNodeJSLinux() error {
	// 尝试使用包管理器
	if _, err := exec.LookPath("apt-get"); err == nil {
		i.addLog("使用 apt-get 安装 Node.js...")
		cmd := exec.Command("sudo", "apt-get", "update")
		cmd.Run()

		cmd = exec.Command("sudo", "apt-get", "install", "-y", "nodejs", "npm")
		return i.executeCommandWithStreaming(cmd)
	}

	if _, err := exec.LookPath("yum"); err == nil {
		i.addLog("使用 yum 安装 Node.js...")
		cmd := exec.Command("sudo", "yum", "install", "-y", "nodejs", "npm")
		return i.executeCommandWithStreaming(cmd)
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
			"/opt/homebrew/bin/git", // Apple Silicon Homebrew
			"/usr/local/bin/git",    // Intel Homebrew
			"/usr/bin/git",          // 系统默认
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
	// 使用批处理脚本下载和安装
	i.addLog("创建Git安装脚本...")

	tempDir := os.TempDir()
	scriptPath := filepath.Join(tempDir, "install_git.bat")

	// 创建批处理脚本内容
	scriptContent := `@echo off
chcp 65001 >nul
echo Starting Git installation...

set "GIT_URL1=https://cdn.npmmirror.com/binaries/git-for-windows/v2.50.1.windows.1/Git-2.50.1-64-bit.exe"
set "GIT_URL2=https://github.com/git-for-windows/git/releases/download/v2.50.1.windows.1/Git-2.50.1-64-bit.exe"
set "GIT_URL3=https://mirrors.tuna.tsinghua.edu.cn/github-release/git-for-windows/git/v2.50.1.windows.1/Git-2.50.1-64-bit.exe"
set "INSTALLER_PATH=%TEMP%\git-installer.exe"

echo Downloading Git from mirror 1...
powershell -Command "try { Invoke-WebRequest -Uri '%GIT_URL1%' -OutFile '%INSTALLER_PATH%' -TimeoutSec 30 -UseBasicParsing } catch { exit 1 }"
if %ERRORLEVEL% EQU 0 (
    echo Download successful from mirror 1
    goto :install
)

echo Download failed from mirror 1, trying mirror 2...
powershell -Command "try { Invoke-WebRequest -Uri '%GIT_URL2%' -OutFile '%INSTALLER_PATH%' -TimeoutSec 30 -UseBasicParsing } catch { exit 1 }"
if %ERRORLEVEL% EQU 0 (
    echo Download successful from mirror 2
    goto :install
)

echo Download failed from mirror 2, trying mirror 3...
powershell -Command "try { Invoke-WebRequest -Uri '%GIT_URL3%' -OutFile '%INSTALLER_PATH%' -TimeoutSec 30 -UseBasicParsing } catch { exit 1 }"
if %ERRORLEVEL% EQU 0 (
    echo Download successful from mirror 3
    goto :install
)

echo ERROR: All download sources failed
exit /b 1

:install
echo Installing Git...
"%INSTALLER_PATH%" /VERYSILENT /NORESTART /NOCANCEL /SP- /CLOSEAPPLICATIONS /RESTARTAPPLICATIONS
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Git installation failed with code %ERRORLEVEL%
    del /f /q "%INSTALLER_PATH%" 2>nul
    exit /b %ERRORLEVEL%
)

echo Git installation completed
del /f /q "%INSTALLER_PATH%" 2>nul

echo Refreshing environment variables...
for /f "tokens=2*" %%A in ('reg query "HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment" /v Path 2^>nul') do set "SystemPath=%%B"
for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v Path 2^>nul') do set "UserPath=%%B"
set "PATH=%SystemPath%;%UserPath%"

echo Verifying Git installation...
git --version >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    for /f "tokens=*" %%i in ('git --version') do echo Git installed successfully: %%i
) else (
    if exist "C:\Program Files\Git\bin\git.exe" (
        "C:\Program Files\Git\bin\git.exe" --version >nul 2>&1
        if %ERRORLEVEL% EQU 0 (
            for /f "tokens=*" %%i in ('"C:\Program Files\Git\bin\git.exe" --version') do echo Git installed at: C:\Program Files\Git\bin\git.exe [%%i]
            echo You may need to restart terminal to use 'git' command
        )
    ) else (
        echo WARNING: Git installed but not found in PATH
    )
)

echo Installation script completed
exit /b 0
`

	// 写入脚本文件（使用UTF-8编码）
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("创建安装脚本失败: %v", err)
	}
	defer os.Remove(scriptPath)

	i.addLog(fmt.Sprintf("执行安装脚本: %s", scriptPath))

	// 执行批处理脚本 - 使用流式输出避免UI卡住
	cmd := exec.Command("cmd", "/c", scriptPath)
	cmd.Dir = tempDir

	// 设置输出编码为UTF-8
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	// 使用流式执行避免UI卡住
	err = i.executeCommandWithStreaming(cmd)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("Git 安装失败，退出代码: %d", exitErr.ExitCode())
		}
		return fmt.Errorf("Git 安装失败: %v", err)
	}

	// 再次验证安装
	if err := i.checkGit(); err == nil {
		i.addLog("✅ Git 安装验证成功")
		return nil
	}

	// 如果验证失败，但安装脚本成功，说明可能需要重启
	i.addLog("⚠️ Git 已安装，但可能需要重启终端或系统才能生效")

	// 尝试设置临时环境变量
	possiblePaths := []string{
		`C:\Program Files\Git\bin`,
		`C:\Program Files (x86)\Git\bin`,
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin"),
	}

	for _, gitPath := range possiblePaths {
		gitExe := filepath.Join(gitPath, "git.exe")
		if _, err := os.Stat(gitExe); err == nil {
			os.Setenv("PATH", fmt.Sprintf("%s;%s", gitPath, os.Getenv("PATH")))
			i.addLog(fmt.Sprintf("已将 %s 添加到临时PATH", gitPath))
			break
		}
	}

	return nil
}

func (i *Installer) installGitMac() error {
	// 首先检查是否已经安装了 Git（通过 Xcode Command Line Tools）
	if err := i.checkGit(); err == nil {
		i.addLog("Git 已通过 Xcode Command Line Tools 安装")
		return nil
	}

	// 检查是否有 Homebrew
	cmd := exec.Command("brew", "--version")
	if err := cmd.Run(); err == nil {
		// 使用 Homebrew 安装，配置中国镜像源
		i.addLog("配置 Homebrew 使用中国镜像源...")
		
		// 创建配置脚本
		tempDir := os.TempDir()
		brewScriptPath := filepath.Join(tempDir, "brew_install_git.sh")
		
		brewScript := `#!/bin/bash
# 保存用户原有的 Homebrew 配置
OLD_HOMEBREW_BREW_GIT_REMOTE="$HOMEBREW_BREW_GIT_REMOTE"
OLD_HOMEBREW_CORE_GIT_REMOTE="$HOMEBREW_CORE_GIT_REMOTE"
OLD_HOMEBREW_BOTTLE_DOMAIN="$HOMEBREW_BOTTLE_DOMAIN"

# 检查是否已经配置了镜像源
if [[ -n "$HOMEBREW_BOTTLE_DOMAIN" ]]; then
    echo "检测到已配置 Homebrew 镜像源: $HOMEBREW_BOTTLE_DOMAIN"
    echo "将使用现有配置..."
else
    # 只有在没有配置的情况下才设置镜像源
    echo "配置 Homebrew 使用中国科技大学镜像源..."
    export HOMEBREW_BREW_GIT_REMOTE="https://mirrors.ustc.edu.cn/brew.git"
    export HOMEBREW_CORE_GIT_REMOTE="https://mirrors.ustc.edu.cn/homebrew-core.git"
    export HOMEBREW_BOTTLE_DOMAIN="https://mirrors.ustc.edu.cn/homebrew-bottles"
    echo "HOMEBREW_BOTTLE_DOMAIN=$HOMEBREW_BOTTLE_DOMAIN"
fi

# 更新并安装 Git
echo "更新 Homebrew..."
brew update || echo "更新失败，继续尝试安装..."

echo "安装 Git..."
brew install git
`
		
		if err := os.WriteFile(brewScriptPath, []byte(brewScript), 0755); err == nil {
			defer os.Remove(brewScriptPath)
			
			cmd = exec.Command("bash", brewScriptPath)
			cmd.Dir = tempDir
			
			// 使用流式执行避免UI卡住
			if err := i.executeCommandWithStreaming(cmd); err == nil {
				return nil
			}
			i.addLog("Homebrew 安装 Git 失败，尝试其他方法...")
		}
	}

	// 如果没有 Homebrew 或 Homebrew 安装失败，尝试安装 Xcode Command Line Tools
	i.addLog("尝试安装 Xcode Command Line Tools (包含 Git)...")
	
	// 创建安装脚本
	tempDir := os.TempDir()
	scriptPath := filepath.Join(tempDir, "install_git.sh")
	
	scriptContent := `#!/bin/bash
set -e

echo "[STEP 1] Checking for Xcode Command Line Tools..."

# Check if git is already available through Xcode CLT
if /usr/bin/git --version >/dev/null 2>&1; then
    GIT_VERSION=$(/usr/bin/git --version)
    echo "Git is already installed: $GIT_VERSION"
    exit 0
fi

echo "[STEP 2] Installing Xcode Command Line Tools..."
echo "This will open a dialog window. Please click 'Install' when prompted."
echo "This process may take 10-15 minutes depending on your internet speed."

# Trigger the installation dialog
xcode-select --install 2>&1 || true

# Wait for user to start installation
echo ""
echo "Waiting for installation to begin..."
sleep 5

# Check if installation is in progress
while true; do
    if pgrep -x "Install Command Line Developer Tools" >/dev/null 2>&1; then
        echo "Installation in progress..."
        sleep 10
    else
        # Check if installation completed
        if /usr/bin/git --version >/dev/null 2>&1; then
            GIT_VERSION=$(/usr/bin/git --version)
            echo "[STEP 3] Git installed successfully: $GIT_VERSION"
            exit 0
        else
            # Check if user cancelled
            if xcode-select -p >/dev/null 2>&1; then
                echo "Xcode Command Line Tools detected, checking Git..."
                if /usr/bin/git --version >/dev/null 2>&1; then
                    GIT_VERSION=$(/usr/bin/git --version)
                    echo "Git installed successfully: $GIT_VERSION"
                    exit 0
                fi
            fi
            
            echo "Installation may have been cancelled or failed."
            echo "Please try running 'xcode-select --install' manually."
            exit 1
        fi
    fi
done
`

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("创建安装脚本失败: %v", err)
	}
	defer os.Remove(scriptPath)

	i.addLog(fmt.Sprintf("执行安装脚本: %s", scriptPath))

	// 使用流式执行
	cmd = exec.Command("bash", scriptPath)
	cmd.Dir = tempDir

	err = i.executeCommandWithStreaming(cmd)
	if err != nil {
		return fmt.Errorf("Git 安装失败: %v. 请手动运行 'xcode-select --install' 安装 Xcode Command Line Tools", err)
	}

	// 验证安装
	if err := i.checkGit(); err == nil {
		i.addLog("✅ Git 安装成功！")
		return nil
	}

	return fmt.Errorf("Git 安装失败，请手动安装 Xcode Command Line Tools 或使用 Homebrew")
}

func (i *Installer) installGitLinux() error {
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.Command("sudo", "apt-get", "install", "-y", "git")
		return i.executeCommandWithStreaming(cmd)
	}

	if _, err := exec.LookPath("yum"); err == nil {
		cmd := exec.Command("sudo", "yum", "install", "-y", "git")
		return i.executeCommandWithStreaming(cmd)
	}

	return fmt.Errorf("无法自动安装 Git，请手动安装")
}

func (i *Installer) installClaudeCode() error {
	i.addLog("安装 Claude Code...")

	// 使用淘宝 npm 镜像
	cmd := exec.Command("npm", "install", "-g", "@anthropic-ai/claude-code", "--registry=https://registry.npmmirror.com")

	// 使用流式执行避免UI卡住
	err := i.executeCommandWithStreaming(cmd)

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
				"ANTHROPIC_BASE_URL":             "https://api.moonshot.cn/anthropic/",
				"ANTHROPIC_API_KEY":              apiKey,
				"CLAUDE_REQUEST_DELAY_MS":        fmt.Sprintf("%d", requestDelay),
				"CLAUDE_MAX_CONCURRENT_REQUESTS": "1",
			}

			for envVar, value := range envVars {
				// 设置用户级环境变量（使用 setx）
				i.addLog(fmt.Sprintf("🔧 执行命令: setx %s \"%s\"", envVar, value))
				cmd := exec.Command("setx", envVar, value)
				output, err := cmd.CombinedOutput()
				if err != nil {
					i.addLog(fmt.Sprintf("⚠️ 设置环境变量 %s 失败: %v", envVar, err))
					if len(output) > 0 {
						i.addLog(fmt.Sprintf("   错误输出: %s", string(output)))
					}
				} else {
					i.addLog(fmt.Sprintf("✅ 已设置用户环境变量: %s = %s", envVar, value))
					if len(output) > 0 {
						i.addLog(fmt.Sprintf("   命令输出: %s", string(output)))
					}
				}
			}

			i.addLog(fmt.Sprintf("永久环境变量已设置（请求延迟: %d毫秒），可能需要重启终端才能生效", requestDelay))
		} else {
			// 创建临时批处理脚本设置环境变量
			i.addLog("正在创建临时环境变量脚本...")

			// 获取临时目录
			tempDir := os.TempDir()
			// 使用批处理脚本，更稳定可靠
			scriptPath := filepath.Join(tempDir, "claude_k2_setup.bat")
			scriptContent := fmt.Sprintf(`@echo off
REM Claude Code K2 Environment Variables Setup Script
set "ANTHROPIC_BASE_URL=https://api.moonshot.cn/anthropic/"
set "ANTHROPIC_API_KEY=%s"
set "CLAUDE_REQUEST_DELAY_MS=%d"
set "CLAUDE_MAX_CONCURRENT_REQUESTS=1"
set "ANTHROPIC_AUTH_TOKEN="

echo K2 Environment Variables Set:
echo   - API Key: %s...
echo   - Base URL: https://api.moonshot.cn/anthropic/
echo   - Request Delay: %d ms
echo.
echo You can now run 'claude' command with K2 API
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

	// 处理 .claude.json 文件
	claudeJsonPath := filepath.Join(home, ".claude.json")
	backupPath := claudeJsonPath + ".backup"

	i.addLog(fmt.Sprintf("🔍 处理配置文件: %s", claudeJsonPath))

	// 读取或创建 .claude.json 配置
	config := make(map[string]interface{})

	// 尝试读取现有配置
	if data, err := os.ReadFile(claudeJsonPath); err == nil {
		i.addLog("📖 读取现有配置文件...")
		if err := json.Unmarshal(data, &config); err != nil {
			i.addLog(fmt.Sprintf("⚠️ 解析配置文件失败: %v", err))
			config = make(map[string]interface{})
		}
	} else if _, backupErr := os.Stat(backupPath); backupErr == nil {
		i.addLog("📋 从备份文件读取配置...")
		if backupData, readErr := os.ReadFile(backupPath); readErr == nil {
			if err := json.Unmarshal(backupData, &config); err != nil {
				i.addLog(fmt.Sprintf("⚠️ 解析备份文件失败: %v", err))
				config = make(map[string]interface{})
			}
		}
	} else {
		i.addLog("📄 创建新的配置文件...")
	}

	// 添加/更新K2配置
	config["hasCompletedOnboarding"] = true
	config["apiKey"] = apiKey
	config["apiBaseUrl"] = "https://api.moonshot.cn/anthropic/"
	config["requestDelayMs"] = requestDelay
	config["maxConcurrentRequests"] = 1

	// 写回配置文件
	if jsonData, err := json.MarshalIndent(config, "", "  "); err != nil {
		i.addLog(fmt.Sprintf("⚠️ 序列化配置失败: %v", err))
	} else {
		if err := os.WriteFile(claudeJsonPath, jsonData, 0644); err != nil {
			i.addLog(fmt.Sprintf("⚠️ 写入配置文件失败: %v", err))
			i.forceCreateClaudeConfig(claudeJsonPath, string(jsonData))
		} else {
			i.addLog("✅ 已更新 .claude.json 配置文件")
		}
	}

	i.addLog("K2 API 配置完成")
	return nil
}

// forceCreateClaudeConfig 强制创建Claude配置文件
func (i *Installer) forceCreateClaudeConfig(filePath, content string) {
	i.addLog("💪 尝试强制创建配置文件...")

	// 方法1: 直接写入
	if err := os.WriteFile(filePath, []byte(content), 0644); err == nil {
		i.addLog("✅ 方法1成功: 直接写入")
		return
	} else {
		i.addLog(fmt.Sprintf("⚠️ 方法1失败: %v", err))
	}

	// 方法2: 尝试更宽松的权限
	if err := os.WriteFile(filePath, []byte(content), 0666); err == nil {
		i.addLog("✅ 方法2成功: 宽松权限写入")
		return
	} else {
		i.addLog(fmt.Sprintf("⚠️ 方法2失败: %v", err))
	}

	// 方法3: 创建文件后写入
	if file, err := os.Create(filePath); err == nil {
		defer file.Close()
		if _, writeErr := file.WriteString(content); writeErr == nil {
			i.addLog("✅ 方法3成功: 创建文件后写入")
			return
		} else {
			i.addLog(fmt.Sprintf("⚠️ 方法3写入失败: %v", writeErr))
		}
	} else {
		i.addLog(fmt.Sprintf("⚠️ 方法3创建失败: %v", err))
	}

	i.addLog("❌ 所有方法都失败了，配置文件创建失败")
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
	// 创建带超时的 HTTP 客户端
	// 注意：这是总体超时时间，包括连接和下载
	client := &http.Client{
		Timeout: 5 * time.Minute, // 5分钟总超时（大文件需要更长时间）
		Transport: &http.Transport{
			// 连接超时设置
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second, // 连接超时10秒
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			// 空闲连接设置
			IdleConnTimeout:     90 * time.Second,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
		},
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// 设置用户代理，避免被某些服务器拒绝
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	i.addLog(fmt.Sprintf("开始下载: %s", url))
	i.addLog("连接服务器...")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			return fmt.Errorf("连接超时，请检查网络或稍后重试")
		}
		return fmt.Errorf("连接失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，HTTP状态码: %d", resp.StatusCode)
	}

	// 获取文件大小
	contentLength := resp.ContentLength
	if contentLength > 0 {
		i.addLog(fmt.Sprintf("文件大小: %.2f MB", float64(contentLength)/1024/1024))
	} else {
		i.addLog("文件大小: 未知")
	}

	// 创建输出文件
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 创建带超时的进度读取器
	progressReader := &progressReader{
		Reader:      resp.Body,
		Total:       contentLength,
		Current:     0,
		LastLog:     time.Now(),
		LastRead:    time.Now(),
		Installer:   i,
		ReadTimeout: 30 * time.Second, // 30秒内必须有数据传输
	}

	// 使用缓冲复制，提高性能
	buf := make([]byte, 64*1024) // 64KB 缓冲区（增大缓冲区）
	_, err = io.CopyBuffer(out, progressReader, buf)

	if err != nil {
		if err == io.ErrUnexpectedEOF {
			return fmt.Errorf("下载中断，文件不完整")
		}
		return fmt.Errorf("下载失败: %v", err)
	}

	i.addLog("✅ 下载完成")
	return nil
}

// progressReader 包装 io.Reader 以报告下载进度
type progressReader struct {
	io.Reader
	Total       int64
	Current     int64
	LastLog     time.Time
	LastRead    time.Time
	LastBytes   int64     // 上次记录时的字节数
	StartTime   time.Time // 下载开始时间
	Installer   *Installer
	ReadTimeout time.Duration
}

func (pr *progressReader) Read(p []byte) (int, error) {
	// 初始化开始时间
	if pr.StartTime.IsZero() {
		pr.StartTime = time.Now()
		pr.LastBytes = 0
	}

	// 检查读取超时
	if time.Since(pr.LastRead) > pr.ReadTimeout && pr.Current > 0 {
		return 0, fmt.Errorf("下载停滞：超过%d秒没有新数据", int(pr.ReadTimeout.Seconds()))
	}

	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.Current += int64(n)
		pr.LastRead = time.Now() // 更新最后读取时间
	}

	// 每秒更新一次进度
	if time.Since(pr.LastLog) >= time.Second {
		if pr.Total > 0 {
			percent := float64(pr.Current) * 100 / float64(pr.Total)

			// 计算瞬时速度（最近1秒的速度）
			bytesInLastSecond := pr.Current - pr.LastBytes
			instantSpeed := float64(bytesInLastSecond) / 1024 / 1024 // MB/s

			// 计算平均速度
			totalElapsed := time.Since(pr.StartTime).Seconds()
			avgSpeed := float64(pr.Current) / totalElapsed / 1024 / 1024 // MB/s

			// 使用平均速度预估剩余时间（更稳定）
			remaining := pr.Total - pr.Current
			var etaStr string
			if avgSpeed > 0 {
				etaSeconds := float64(remaining) / (avgSpeed * 1024 * 1024)
				if etaSeconds < 60 {
					etaStr = fmt.Sprintf("%.0f秒", etaSeconds)
				} else if etaSeconds < 3600 {
					etaStr = fmt.Sprintf("%.0f分钟", etaSeconds/60)
				} else {
					etaStr = fmt.Sprintf("%.1f小时", etaSeconds/3600)
				}
			} else {
				etaStr = "计算中..."
			}

			pr.Installer.addLog(fmt.Sprintf("下载进度: %.1f%% (%.2f/%.2f MB) 速度: %.2f MB/s 剩余: %s",
				percent,
				float64(pr.Current)/1024/1024,
				float64(pr.Total)/1024/1024,
				instantSpeed,
				etaStr))
		} else {
			pr.Installer.addLog(fmt.Sprintf("已下载: %.2f MB", float64(pr.Current)/1024/1024))
		}
		pr.LastBytes = pr.Current
		pr.LastLog = time.Now()
	}

	return n, err
}

func (i *Installer) sendProgress(step, message string, percent float64) {
	i.mu.Lock()
	closed := i.closed
	i.mu.Unlock()

	if !closed {
		select {
		case i.Progress <- ProgressUpdate{
			Step:    step,
			Message: message,
			Percent: percent,
		}:
			// 成功发送
		default:
			// channel满了，忽略
		}
	}
}

func (i *Installer) sendError(err error) {
	i.mu.Lock()
	closed := i.closed
	i.mu.Unlock()

	if !closed {
		select {
		case i.Progress <- ProgressUpdate{
			Error: err,
		}:
			// 成功发送
		default:
			// channel满了，忽略
		}
	}
}

func (i *Installer) addLog(message string) {
	i.logs = append(i.logs, message)
	// 检查channel是否已关闭
	i.mu.Lock()
	closed := i.closed
	i.mu.Unlock()

	if !closed {
		// 同步发送到UI，确保实时显示
		select {
		case i.Progress <- ProgressUpdate{
			Step:    "日志",
			Message: message,
			Percent: -1, // -1 表示只更新日志，不更新进度条
		}:
			// 成功发送
		default:
			// channel满了，忽略
		}
	}
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
	// 创建新的 Progress channel 用于配置阶段
	i.mu.Lock()
	if i.closed {
		// 如果原channel已关闭，创建新的
		i.Progress = make(chan ProgressUpdate, 100)
		i.closed = false
	}
	i.mu.Unlock()

	// 配置完成后关闭新的channel
	defer func() {
		i.mu.Lock()
		if !i.closed {
			i.closed = true
			close(i.Progress)
		}
		i.mu.Unlock()
	}()

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
		// Windows: 使用PowerShell脚本清除环境变量，避免卡死
		i.addLog("使用PowerShell清除 Windows 环境变量...")
		i.createWindowsRestoreScript()
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

// executeCommandWithStreaming 执行命令并实时输出日志，避免UI卡住
func (i *Installer) executeCommandWithStreaming(cmd *exec.Cmd) error {
	// 创建管道以实时获取输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建输出管道失败: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建错误管道失败: %v", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动命令失败: %v", err)
	}

	// 并发读取输出
	var wg sync.WaitGroup
	wg.Add(2)

	// 读取标准输出
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				i.addLog(line)
			}
		}
	}()

	// 读取错误输出
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				i.addLog(line)
			}
		}
	}()

	// 等待输出读取完成
	wg.Wait()

	// 等待命令执行完成
	return cmd.Wait()
}

// createWindowsRestoreScript 创建Windows恢复脚本
func (i *Installer) createWindowsRestoreScript() {
	tempDir := os.TempDir()
	scriptPath := filepath.Join(tempDir, "claude_restore.ps1")

	scriptContent := `# Claude Code 环境变量清理脚本
$envVars = @(
    "ANTHROPIC_BASE_URL",
    "ANTHROPIC_API_KEY", 
    "ANTHROPIC_AUTH_TOKEN",
    "CLAUDE_REQUEST_DELAY_MS",
    "CLAUDE_MAX_CONCURRENT_REQUESTS"
)

Write-Host "开始清理 Claude Code 环境变量..." -ForegroundColor Yellow

foreach ($envVar in $envVars) {
    # 清除用户级环境变量
    try {
        [System.Environment]::SetEnvironmentVariable($envVar, $null, [System.EnvironmentVariableTarget]::User)
        Write-Host "✅ 已清除用户环境变量: $envVar" -ForegroundColor Green
    } catch {
        Write-Host "⚠️ 清除用户环境变量失败: $envVar" -ForegroundColor Yellow
    }
    
    # 清除进程级环境变量
    try {
        [System.Environment]::SetEnvironmentVariable($envVar, $null, [System.EnvironmentVariableTarget]::Process)
    } catch {}
}

# 清理临时脚本
$tempScripts = @(
    "$env:TEMP\claude_k2_setup.ps1",
    "$env:TEMP\claude_k2_setup.bat"
)

foreach ($script in $tempScripts) {
    if (Test-Path $script) {
        try {
            Remove-Item $script -Force
            Write-Host "🗑️ 已删除临时脚本: $script" -ForegroundColor Cyan
        } catch {
            Write-Host "⚠️ 删除临时脚本失败: $script" -ForegroundColor Yellow
        }
    }
}

Write-Host "✅ Claude Code 环境变量清理完成！" -ForegroundColor Green
Write-Host "请重启命令行窗口以确保环境变量生效" -ForegroundColor Cyan
`

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		i.addLog(fmt.Sprintf("⚠️ 创建恢复脚本失败: %v", err))
		return
	}

	i.addLog(fmt.Sprintf("📝 已创建恢复脚本: %s", scriptPath))

	// 执行PowerShell脚本
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		i.addLog(fmt.Sprintf("⚠️ 执行恢复脚本失败: %v", err))
	} else {
		i.addLog("✅ PowerShell恢复脚本执行完成")
		// 输出脚本执行结果
		if len(output) > 0 {
			i.addLog(fmt.Sprintf("脚本输出: %s", string(output)))
		}
	}

	// 清理脚本文件
	os.Remove(scriptPath)
}
