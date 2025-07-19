package ui

import (
	"fmt"
	"image/color"
	
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type Tutorial struct {
	parent  fyne.Window
	current int
	pages   []TutorialPage
}

type TutorialPage struct {
	Title   string
	Content string
}

func NewTutorial(parent fyne.Window) *Tutorial {
	return &Tutorial{
		parent:  parent,
		current: 0,
		pages: []TutorialPage{
			{
				Title: "欢迎使用 Claude Code + K2 集成工具",
				Content: `本工具将帮助你一键安装和配置 Claude Code 与 Kimi K2 大模型环境。

主要功能：
• 自动检测并安装必要的依赖（Node.js、Git）
• 一键安装 Claude Code CLI 工具
• 自动配置 Kimi K2 API
• 无需手动输入复杂命令

点击"下一步"继续了解详细信息。`,
			},
			{
				Title: "什么是 Claude Code？",
				Content: `Claude Code 是 Anthropic 官方推出的 AI 编程助手工具。

特点：
• 使用强大的 Claude 模型
• 支持多种编程语言
• 可以理解项目上下文
• 提供智能代码补全和重构建议

通过集成 Kimi K2 模型，可以获得更高性价比的使用体验。`,
			},
			{
				Title: "Kimi K2 模型介绍",
				Content: `Kimi K2 是月之暗面推出的新一代大语言模型。

技术特性：
• 1T 参数量的超大模型
• 能力介于 Claude 3.7 和 Claude 4 之间
• 提供兼容 Claude API 的接口
• 性价比极高

注册即送 15 元额度，充值 50 元即可正常使用。`,
			},
			{
				Title: "获取 Kimi API Key",
				Content: `要使用 Kimi K2 模型，需要先获取 API Key：

1. 访问 https://platform.moonshot.cn/console/account
2. 注册或登录账号
3. 充值至少 50 元（避免 RPM 限制）
4. 在 API Key 管理页面创建新的 Key
5. 复制 sk 开头的密钥

将获取的 API Key 填入本工具即可自动配置。`,
			},
			{
				Title: "安装完成后的使用",
				Content: `安装完成后，你可以：

1. 在终端运行 'claude' 命令启动 Claude Code
2. 使用 Claude Code 进行 AI 辅助编程
3. 享受 K2 模型带来的高性价比体验

常用命令：
• claude - 启动交互模式
• claude --help - 查看帮助信息
• claude --version - 查看版本

祝你使用愉快！`,
			},
		},
	}
}

func (t *Tutorial) Show() {
	content := t.createContent()
	
	d := dialog.NewCustom("使用教程", "关闭", content, t.parent)
	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}

func (t *Tutorial) createContent() fyne.CanvasObject {
	// 创建明亮背景
	bg := canvas.NewRectangle(color.RGBA{R: 255, G: 255, B: 255, A: 255})
	
	titleLabel := widget.NewLabelWithStyle(
		t.pages[t.current].Title,
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	
	contentLabel := widget.NewLabel(t.pages[t.current].Content)
	contentLabel.Wrapping = fyne.TextWrapWord
	
	contentScroll := container.NewScroll(contentLabel)
	contentScroll.SetMinSize(fyne.NewSize(0, 250))
	
	pageLabel := widget.NewLabel("")
	t.updatePageLabel(pageLabel)
	
	// 声明按钮变量
	var prevButton, nextButton *widget.Button
	
	// 创建导航按钮
	prevButton = widget.NewButton("上一步", func() {
		if t.current > 0 {
			t.current--
			t.updateContent(titleLabel, contentLabel)
			t.updateButtons(prevButton, nextButton)
			t.updatePageLabel(pageLabel)
		}
	})
	
	nextButton = widget.NewButton("下一步", func() {
		if t.current < len(t.pages)-1 {
			t.current++
			t.updateContent(titleLabel, contentLabel)
			t.updateButtons(prevButton, nextButton)
			t.updatePageLabel(pageLabel)
		}
	})
	
	// 更新按钮状态
	t.updateButtons(prevButton, nextButton)
	
	navContainer := container.NewBorder(
		nil, nil,
		prevButton,
		nextButton,
		pageLabel,
	)
	
	content := container.NewBorder(
		titleLabel,
		navContainer,
		nil, nil,
		contentScroll,
	)
	
	// 使用白色背景容器包装
	return container.NewStack(bg, content)
}

func (t *Tutorial) updateContent(title, content *widget.Label) {
	title.SetText(t.pages[t.current].Title)
	content.SetText(t.pages[t.current].Content)
}

func (t *Tutorial) updateButtons(prev, next *widget.Button) {
	prev.Enable()
	next.Enable()
	
	if t.current == 0 {
		prev.Disable()
	}
	if t.current == len(t.pages)-1 {
		next.Disable()
	}
}

func (t *Tutorial) updatePageLabel(label *widget.Label) {
	label.Alignment = fyne.TextAlignCenter
	label.SetText(fmt.Sprintf("%d / %d", t.current+1, len(t.pages)))
}