package ui

import (
	"claude-k2-installer/internal/installer"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type Manager struct {
	window    fyne.Window
	installer *installer.Installer

	// UI 组件
	progressBar       *widget.ProgressBar
	statusLabel       *widget.Label
	logsDisplay       *widget.Entry
	installButton     *widget.Button
	apiKeyEntry       *widget.Entry
	rpmEntry          *widget.Entry
	tutorialButton    *widget.Button
	openButton        *widget.Button
	systemConfigCheck *widget.Check
}

func NewManager(window fyne.Window, inst *installer.Installer) *Manager {
	return &Manager{
		window:    window,
		installer: inst,
	}
}

// loadSavedConfig 加载已保存的配置
func (m *Manager) loadSavedConfig() {
	if config, err := LoadConfig(); err == nil {
		if m.apiKeyEntry != nil && config.APIKey != "" {
			m.apiKeyEntry.SetText(config.APIKey)
		}
		if m.rpmEntry != nil && config.RPM != "" {
			m.rpmEntry.SetText(config.RPM)
		}
	}
}

// saveCurrentConfig 保存当前配置
func (m *Manager) saveCurrentConfig() {
	if m.apiKeyEntry != nil && m.rpmEntry != nil {
		SaveConfig(m.apiKeyEntry.Text, m.rpmEntry.Text)
	}
}

func (m *Manager) CreateMainContent() fyne.CanvasObject {
	// 创建标题 - 使用更鲜艳的颜色
	title := canvas.NewText("Claude Code + K2 环境集成工具", color.RGBA{R: 30, G: 41, B: 59, A: 255})
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("一键安装配置 Claude Code 和 Kimi K2 开发环境", color.RGBA{R: 59, G: 130, B: 246, A: 255})
	subtitle.TextSize = 14
	subtitle.TextStyle = fyne.TextStyle{Bold: true}
	subtitle.Alignment = fyne.TextAlignCenter

	// 添加作者信息 - 可点击复制的微信号
	wechatBtn := widget.NewButton("🤖 加微信: ruan11223344 进群分享最新AI知识，一起学习进步 (点击复制)", func() {
		m.window.Clipboard().SetContent("ruan11223344")
		m.showQRCodeDialog()
	})
	wechatBtn.Importance = widget.HighImportance

	// 直接显示安装界面
	mainContent := m.createInstallerContent()

	// 组装完整界面
	content := container.NewVBox(
		container.NewPadded(container.NewVBox(title, subtitle)),
		container.NewPadded(wechatBtn),
		widget.NewSeparator(),
		mainContent,
	)

	return container.NewScroll(content)
}

// createInstallerContent 创建安装界面
func (m *Manager) createInstallerContent() fyne.CanvasObject {
	// 创建进度条
	m.progressBar = widget.NewProgressBar()
	m.statusLabel = widget.NewLabel("准备就绪")

	// 创建日志显示区
	m.logsDisplay = widget.NewMultiLineEntry()
	m.logsDisplay.Disable()
	m.logsDisplay.SetPlaceHolder("安装日志将显示在这里...")

	logScroll := container.NewScroll(m.logsDisplay)
	logScroll.SetMinSize(fyne.NewSize(0, 500))

	// API Key 输入
	m.apiKeyEntry = widget.NewPasswordEntry()
	m.apiKeyEntry.SetPlaceHolder("请输入API Key")
	m.apiKeyEntry.Resize(fyne.NewSize(300, 36)) // 固定尺寸

	// API Key 获取链接 - 可点击
	apiKeyBtn := widget.NewButton("🔑 点击获取 API Key", func() {
		urlStr := "https://platform.moonshot.cn/console/api-keys"
		m.openURL(urlStr)
	})
	apiKeyBtn.Importance = widget.MediumImportance

	// 恢复按钮
	restoreBtn := widget.NewButton("🔄 恢复Claude配置", func() {
		m.restoreClaudeConfig()
	})
	restoreBtn.Importance = widget.LowImportance

	apiKeyContainer := container.NewVBox(
		container.NewBorder(
			nil, nil,
			widget.NewLabel("API Key:"),
			container.NewHBox(apiKeyBtn, restoreBtn),
			m.apiKeyEntry,
		),
	)

	// 速率限制输入
	m.rpmEntry = widget.NewEntry()
	m.rpmEntry.SetPlaceHolder("3")
	m.rpmEntry.SetText("3")                  // 默认值（免费用户）
	m.rpmEntry.Resize(fyne.NewSize(100, 36)) // 固定尺寸，比较小

	// 速率限制说明
	rpmInfo := widget.NewLabel("免费: 3 | ¥50: 200 | ¥100: 500 | ¥500+: 5000")
	rpmInfo.TextStyle = fyne.TextStyle{Italic: true}

	rpmDesc := widget.NewLabel("* 速率限制基于Kimi充值额度，实测最少充值50元才不会影响使用")
	rpmDesc.TextStyle = fyne.TextStyle{Italic: true, Bold: true}
	rpmDesc.Alignment = fyne.TextAlignLeading

	// 充值链接 - 可点击
	chargeBtn := widget.NewButton("💳 打开Kimi充值链接", func() {
		urlStr := "https://platform.moonshot.cn/console/pay"
		m.openURL(urlStr)
	})
	chargeBtn.Importance = widget.MediumImportance

	rpmContainer := container.NewVBox(
		container.NewBorder(
			nil, nil,
			widget.NewLabel("速率限制 (RPM):"),
			chargeBtn,
			m.rpmEntry,
		),
		rpmInfo,
		rpmDesc,
	)

	// 自动设置勾选框
	m.systemConfigCheck = widget.NewCheck("永久设置K2环境变量（推荐 - 写入.bashrc/.zshrc/Windows环境变量）", nil)
	m.systemConfigCheck.SetChecked(true) // 默认勾选，永久设置

	// 添加说明文字
	envVarHelp := widget.NewLabel("✓ 勾选：永久设置（写入配置文件）  ✗ 不勾选：仅当前进程")
	envVarHelp.TextStyle = fyne.TextStyle{Italic: true}
	envVarHelp.Alignment = fyne.TextAlignLeading

	// 创建按钮
	m.installButton = widget.NewButton("开始安装", m.onInstallClick)
	m.installButton.Importance = widget.HighImportance

	m.tutorialButton = widget.NewButton("查看教程", m.showTutorial)

	// 创建打开按钮（初始隐藏）
	m.openButton = widget.NewButton("打开 Claude Code", m.openClaudeCode)
	m.openButton.Importance = widget.HighImportance
	m.openButton.Hide()

	buttonContainer := container.NewHBox(
		layout.NewSpacer(),
		m.tutorialButton,
		m.installButton,
		m.openButton,
		layout.NewSpacer(),
	)

	// 创建步骤说明
	stepsCard := m.createStepsCard()

	// 组装安装界面 - 改为左右布局
	leftPanel := container.NewVBox(
		stepsCard,
		widget.NewSeparator(),
		container.NewVBox(
			widget.NewLabel("配置信息"),
			apiKeyContainer,
			widget.NewSeparator(),
			rpmContainer,
			widget.NewSeparator(),
			m.systemConfigCheck,
			envVarHelp,
		),
		buttonContainer,
	)

	// 加载已保存的配置
	m.loadSavedConfig()

	rightPanel := container.NewVBox(
		container.NewVBox(
			widget.NewLabel("安装进度"),
			m.progressBar,
			m.statusLabel,
		),
		widget.NewSeparator(),
		container.NewVBox(
			widget.NewLabel("安装日志"),
			logScroll,
		),
	)

	// 左右分栏布局 - 左边60%，右边40%
	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(0.65) // 左侧60%，右侧40%
	return split
}

func (m *Manager) createStepsCard() fyne.CanvasObject {
	steps := []string{
		"1. 检查系统环境",
		"2. 自动安装 Node.js (如未安装)",
		"3. 自动安装 Git (如未安装)",
		"4. 安装 Claude Code CLI 工具",
		"5. 配置 Kimi K2 API",
		"6. 验证环境配置",
	}

	var labels []fyne.CanvasObject
	for _, step := range steps {
		label := widget.NewLabel(step)
		labels = append(labels, label)
	}

	stepsContainer := container.NewVBox(labels...)

	card := widget.NewCard("安装步骤", "本工具将自动完成以下步骤：", stepsContainer)

	return card
}

func (m *Manager) onInstallClick() {
	// 检查 API Key
	apiKey := m.apiKeyEntry.Text
	if apiKey == "" {
		dialog.ShowError(fmt.Errorf("请输入 Kimi K2 API Key"), m.window)
		return
	}

	// 获取速率限制
	rpm := m.rpmEntry.Text
	if rpm == "" {
		rpm = "3" // 默认值改为3
	}
	// 验证是否为数字
	if _, err := strconv.Atoi(rpm); err != nil {
		dialog.ShowError(fmt.Errorf("速率限制必须是数字"), m.window)
		return
	}

	// 保存当前配置
	m.saveCurrentConfig()

	// 禁用安装按钮
	m.installButton.Disable()
	m.logsDisplay.SetText("")

	// 启动安装
	go m.installer.Install()

	// 启动进度监控协程
	go func() {
		// 添加 panic 恢复机制
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("安装过程中发生错误: %v", r)
				fmt.Println(errMsg)
				if m.statusLabel != nil {
					m.statusLabel.SetText("安装失败")
				}
				if m.installButton != nil {
					m.installButton.Enable()
				}
				// 延迟显示错误对话框
				time.AfterFunc(100*time.Millisecond, func() {
					if m.window != nil {
						dialog.ShowError(fmt.Errorf(errMsg), m.window)
					}
				})
			}
		}()

		// Install() 方法会关闭 channel，这里不需要再关闭

		// 监控安装进度
		for update := range m.installer.Progress {
			if update.Error != nil {
				// 更新 UI
				if m.statusLabel != nil {
					m.statusLabel.SetText(fmt.Sprintf("错误: %v", update.Error))
				}
				if m.installButton != nil {
					m.installButton.Enable()
				}
				// 延迟显示错误对话框
				time.AfterFunc(100*time.Millisecond, func() {
					if m.window != nil {
						dialog.ShowError(update.Error, m.window)
					}
				})
				return
			}

			// 更新进度
			if m.progressBar != nil {
				m.progressBar.SetValue(update.Percent)
			}
			if m.statusLabel != nil {
				m.statusLabel.SetText(update.Message)
			}

			// 更新日志
			if m.logsDisplay != nil {
				logs := m.installer.GetLogs()
				logText := strings.Join(logs, "\n")
				m.logsDisplay.SetText(logText)
				// 滚动到底部
				m.logsDisplay.CursorRow = len(logs)
			}
		}

		// channel 已关闭，现在配置 API
		// 先显示完成状态
		m.handleInstallComplete()

		// 然后配置 API
		go func() {
			// 配置 API Key 和速率限制
			if m.statusLabel != nil {
				m.statusLabel.SetText("配置 K2 API...")
			}

			// 更新日志显示
			if m.logsDisplay != nil {
				m.logsDisplay.SetText(m.logsDisplay.Text + "\n配置 K2 API...")
			}

			// 传递系统级配置选项
			useSystemConfig := m.systemConfigCheck != nil && m.systemConfigCheck.Checked
			err := m.installer.ConfigureK2APIWithOptions(apiKey, rpm, useSystemConfig)
			if err != nil {
				// 不影响主流程，只是配置失败
				fyne.Do(func() {
					if m.statusLabel != nil {
						m.statusLabel.SetText("⚠️ 安装完成，但 API 配置失败")
					}
				})
				return
			}

			// 显示最终日志
			fyne.Do(func() {
				if m.logsDisplay != nil {
					logs := m.installer.GetLogs()
					logText := strings.Join(logs, "\n")
					m.logsDisplay.SetText(logText)
				}
				if m.statusLabel != nil {
					m.statusLabel.SetText("✅ 安装和配置全部完成！")
				}
			})
		}()
	}()
}

// handleInstallComplete 处理安装完成
func (m *Manager) handleInstallComplete() {
	// 确保 UI 更新在主线程中执行
	fyne.Do(func() {
		if m.installButton != nil {
			m.installButton.Hide()
		}
		if m.openButton != nil {
			m.openButton.Show()
		}
		if m.statusLabel != nil {
			m.statusLabel.SetText("✅ 安装完成！")
		}

		// 延迟一点显示对话框，确保 UI 更新完成
		time.AfterFunc(100*time.Millisecond, func() {
			if m.window != nil {
				completeDialog := dialog.NewInformation("安装完成",
					"Claude Code + K2 环境已成功安装！\n\n"+
						"点击「打开 Claude Code」按钮开始使用。",
					m.window)
				completeDialog.Show()
			}
		})
	})
}

func (m *Manager) showTutorial() {
	tutorial := NewTutorialWithImages(m.window)
	tutorial.Show()
}

// addLog 添加日志（线程安全）
func (m *Manager) addLog(message string) {
	// 将日志添加到日志显示区
	m.updateUI(func() {
		currentText := m.logsDisplay.Text
		if currentText != "" {
			currentText += "\n"
		}
		m.logsDisplay.SetText(currentText + message)
	})
}

func (m *Manager) updateUI(fn func()) {
	if fn == nil {
		return
	}

	// 确保所有 UI 操作都在主线程中执行
	if m.window == nil {
		return
	}

	// 直接执行，让 Fyne 自己处理线程问题
	// 因为我们已经在 goroutine 中了，所以直接调用即可
	fn()
}

// openURL 打开网址
func (m *Manager) openURL(urlStr string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	case "darwin":
		cmd = exec.Command("open", urlStr)
	default: // linux
		cmd = exec.Command("xdg-open", urlStr)
	}

	if cmd != nil {
		err := cmd.Start()
		if err != nil {
			// 如果打开失败，显示链接让用户手动复制
			m.window.Clipboard().SetContent(urlStr)
			dialog.ShowInformation("链接已复制", fmt.Sprintf("无法自动打开浏览器，链接已复制到剪贴板:\n%s", urlStr), m.window)
		}
	}
}

// restoreClaudeConfig 恢复Claude Code原始配置
func (m *Manager) restoreClaudeConfig() {
	err := m.installer.RestoreOriginalClaudeConfig()
	if err != nil {
		dialog.ShowError(fmt.Errorf("恢复配置失败: %v", err), m.window)
		return
	}
	dialog.ShowInformation("成功", "✅ Claude Code 配置已恢复到初始状态！", m.window)
}

// openClaudeCode 打开 Claude Code
func (m *Manager) openClaudeCode() {
	// 根据操作系统和永久设置选项启动 Claude Code
	var setupScript string
	var cmd *exec.Cmd

	// 检查是否勾选了永久设置
	useSystemConfig := m.systemConfigCheck != nil && m.systemConfigCheck.Checked

	switch runtime.GOOS {
	case "windows":
		// Windows: 根据永久设置决定启动方式
		tempDir := os.TempDir()
		setupScript = filepath.Join(tempDir, "claude_k2_setup.ps1")

		if useSystemConfig {
			// 勾选了永久设置：删除临时脚本，使用PowerShell重新加载环境变量
			os.Remove(setupScript)
			// 创建一个批处理脚本来启动Claude，避免PowerShell执行策略问题
			refreshScript := filepath.Join(tempDir, "claude_start.bat")
			refreshContent := `@echo off
echo 正在启动 Claude Code (永久环境变量模式)...
echo.
rem 通过重新打开注册表来刷新环境变量
for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v ANTHROPIC_API_KEY 2^>nul') do set "ANTHROPIC_API_KEY=%%B"
for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v ANTHROPIC_BASE_URL 2^>nul') do set "ANTHROPIC_BASE_URL=%%B"
for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v CLAUDE_REQUEST_DELAY_MS 2^>nul') do set "CLAUDE_REQUEST_DELAY_MS=%%B"
for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v CLAUDE_MAX_CONCURRENT_REQUESTS 2^>nul') do set "CLAUDE_MAX_CONCURRENT_REQUESTS=%%B"
set "ANTHROPIC_AUTH_TOKEN="

if defined ANTHROPIC_API_KEY (
    echo ✅ 检测到K2环境变量配置
    echo    API Key: %ANTHROPIC_API_KEY:~0,10%...
    echo    Base URL: %ANTHROPIC_BASE_URL%
) else (
    echo ⚠️ 未检测到K2环境变量，请检查安装
)
echo.
echo 启动 Claude Code...
claude
`
			os.WriteFile(refreshScript, []byte(refreshContent), 0755)
			cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", fmt.Sprintf("\"%s\"", refreshScript))
		} else {
			// 未勾选永久设置：使用临时脚本（如果存在）
			if _, err := os.Stat(setupScript); err == nil {
				cmd = exec.Command("cmd", "/c", "start", "powershell", "-ExecutionPolicy", "Bypass", "-NoExit", "-Command", fmt.Sprintf("& '%s'; claude", setupScript))
			} else {
				cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", "claude")
			}
		}
	case "darwin":
		// macOS: 根据永久设置决定启动方式
		setupScript = "/tmp/claude_k2_setup.sh"

		var script string
		if useSystemConfig {
			// 勾选了永久设置：删除临时脚本，使用永久环境变量
			os.Remove(setupScript)
			script = `tell application "Terminal"
				do script "claude"
				activate
			end tell`
		} else {
			// 未勾选永久设置：使用临时脚本（如果存在）
			if _, err := os.Stat(setupScript); err == nil {
				script = fmt.Sprintf(`tell application "Terminal"
				do script "source %s && claude"
				activate
			end tell`, setupScript)
			} else {
				script = `tell application "Terminal"
				do script "claude"
				activate
			end tell`
			}
		}
		cmd = exec.Command("osascript", "-e", script)
	}

	if cmd != nil {
		err := cmd.Start()
		if err != nil {
			dialog.ShowError(fmt.Errorf("无法打开 Claude Code: %v", err), m.window)
		} else {
			// 成功启动，显示提示
			dialog.ShowInformation("成功", "Claude Code 已启动！\n环境变量已自动设置为K2 API。", m.window)
		}
	} else {
		// 这种情况不应该发生在Windows和Mac上
		dialog.ShowError(fmt.Errorf("不支持的操作系统或无法启动终端"), m.window)
	}
}

// showQRCodeDialog 显示包含二维码的对话框
func (m *Manager) showQRCodeDialog() {
	// 使用嵌入的二维码图片资源
	qrImage := canvas.NewImageFromResource(QRCodeResource)
	qrImage.FillMode = canvas.ImageFillContain
	qrImage.SetMinSize(fyne.NewSize(200, 200))

	// 创建文本内容
	title := widget.NewRichTextFromMarkdown("## 微信号已复制到剪贴板\n")
	title.Wrapping = fyne.TextWrapWord

	content := widget.NewRichTextFromMarkdown("**微信号**: ruan11223344\n\n可以扫描二维码直接进群，或搜索微信号添加好友\n进群分享最新AI知识，一起学习进步！")
	content.Wrapping = fyne.TextWrapWord

	// 创建垂直布局容器 - 标题、二维码、文字内容
	contentContainer := container.NewVBox(
		title,
		qrImage,
		content,
	)

	// 显示自定义对话框
	customDialog := dialog.NewCustom("加微信进群", "关闭", contentContainer, m.window)
	customDialog.Resize(fyne.NewSize(300, 400))
	customDialog.Show()
}
