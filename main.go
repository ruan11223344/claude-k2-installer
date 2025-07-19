package main

import (
	"claude-k2-installer/internal/installer"
	"claude-k2-installer/internal/ui"
	"os"

	"fyne.io/fyne/v2/app"
)

func main() {
	// 设置环境变量以支持中文
	os.Setenv("LANG", "zh_CN.UTF-8")

	myApp := app.New()
	myApp.Settings().SetTheme(&ui.CustomTheme{})

	mainWindow := myApp.NewWindow("Claude Code + K2 环境集成工具")
	mainWindow.Resize(ui.DefaultWindowSize)
	mainWindow.CenterOnScreen()

	// 创建安装器实例
	inst := installer.New()

	// 创建UI管理器
	uiManager := ui.NewManager(mainWindow, inst)

	// 直接显示主界面（包含激活状态）
	mainWindow.SetContent(uiManager.CreateMainContent())

	mainWindow.ShowAndRun()
}

