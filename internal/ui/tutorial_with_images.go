package ui

import (
	"claude-k2-installer/assets"
	"fmt"
	"image/color"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type TutorialWithImages struct {
	parent  fyne.Window
	current int
	pages   []TutorialPageWithImage
}

// ImageClickable 可点击的图片组件
type ImageClickable struct {
	widget.BaseWidget
	Image   *canvas.Image
	OnClick func()
}

func (i *ImageClickable) CreateRenderer() fyne.WidgetRenderer {
	return &imageClickableRenderer{
		image: i.Image,
		click: i,
	}
}

func (i *ImageClickable) Tapped(*fyne.PointEvent) {
	if i.OnClick != nil {
		i.OnClick()
	}
}

type imageClickableRenderer struct {
	image *canvas.Image
	click *ImageClickable
}

func (r *imageClickableRenderer) Layout(size fyne.Size) {
	r.image.Resize(size)
}

func (r *imageClickableRenderer) MinSize() fyne.Size {
	return r.image.MinSize()
}

func (r *imageClickableRenderer) Refresh() {
	r.image.Refresh()
}

func (r *imageClickableRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.image}
}

func (r *imageClickableRenderer) Destroy() {}

// TappableContainer 可点击的容器，完全透明无悬停效果
type TappableContainer struct {
	widget.BaseWidget
	rect  *canvas.Rectangle
	onTap func()
}

func (t *TappableContainer) CreateRenderer() fyne.WidgetRenderer {
	return &tappableRenderer{
		rect:      t.rect,
		container: t,
	}
}

func (t *TappableContainer) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

type tappableRenderer struct {
	rect      *canvas.Rectangle
	container *TappableContainer
}

func (r *tappableRenderer) Layout(size fyne.Size) {
	r.rect.Resize(size)
}

func (r *tappableRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *tappableRenderer) Refresh() {}

func (r *tappableRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.rect}
}

func (r *tappableRenderer) Destroy() {}

type TutorialPageWithImage struct {
	Title      string
	Content    string
	ImageData  []byte
	ShowButton bool
	ButtonText string
	ButtonURL  string
}

func NewTutorialWithImages(parent fyne.Window) *TutorialWithImages {
	return &TutorialWithImages{
		parent:  parent,
		current: 0,
		pages: []TutorialPageWithImage{
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
				Title: "重要提醒：请先充值",
				Content: `⚠️ 重要提醒：使用前请先充值！

免费账户限制：
• RPM（每分钟请求数）仅为 3 次
• 无法满足 Claude Code 正常使用需求
• 会频繁出现 429 错误

建议操作：
• 实测最少充值 50 元才不会影响使用
• 充值后 RPM 限制将提升至 200
• 确保工具能够正常使用`,
				ShowButton: true,
				ButtonText: "前往充值",
				ButtonURL:  "https://platform.moonshot.cn/console/pay",
			},
			{
				Title: "步骤1：进入 API Key 管理页面",
				Content: `登录 Kimi 平台后，点击左侧菜单的"API Key 管理"。

在页面右上角，点击"新建 API Key"按钮（如下图红色箭头所示）。`,
				ImageData:  assets.APIKeyPageImage,
				ShowButton: true,
				ButtonText: "打开 API Key 管理页面",
				ButtonURL:  "https://platform.moonshot.cn/console/api-keys",
			},
			{
				Title: "步骤2：创建新的 API Key",
				Content: `在弹出的对话框中：

1. 输入 API Key 名称（如：这里使用默认）
2. 选择项目（默认为 default）
3. 点击"确定"按钮创建

注意：创建前请确保已经充值，否则无法正常使用。`,
				ImageData: assets.CreateAPIKeyImage,
			},
			{
				Title: "步骤3：保存你的 API Key",
				Content: `⚠️ 重要：请立即复制并保存你的 API Key！

• 密钥只会显示一次
• 关闭对话框后将无法再次查看
• 请将密钥保存在安全的地方

复制 sk- 开头的完整密钥，然后将其粘贴到本工具的 API Key 输入框中。`,
				ImageData: assets.APIKeyCreatedImage,
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

func (t *TutorialWithImages) Show() {
	content := t.createContent()

	d := dialog.NewCustom("使用教程", "关闭", content, t.parent)
	d.Resize(fyne.NewSize(800, 600))
	d.Show()
}

func (t *TutorialWithImages) createContent() fyne.CanvasObject {
	titleLabel := widget.NewLabelWithStyle(
		t.pages[t.current].Title,
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	contentLabel := widget.NewLabel(t.pages[t.current].Content)
	contentLabel.Wrapping = fyne.TextWrapWord

	var mainContent fyne.CanvasObject

	// 如果当前页有图片，显示图片
	if t.pages[t.current].ImageData != nil {
		imageResource := fyne.NewStaticResource("tutorial-image", t.pages[t.current].ImageData)
		image := canvas.NewImageFromResource(imageResource)
		image.FillMode = canvas.ImageFillContain
		image.SetMinSize(fyne.NewSize(600, 400))

		// 创建完全透明的矩形作为点击层
		clickRect := canvas.NewRectangle(color.RGBA{0, 0, 0, 0}) // 完全透明
		clickRect.Resize(image.Size())
		
		// 创建点击区域容器
		clickContainer := &TappableContainer{
			rect: clickRect,
			onTap: func() {
				t.showLargeImage(imageResource)
			},
		}
		clickContainer.ExtendBaseWidget(clickContainer)
		clickContainer.Resize(image.Size())
		
		// 使用 Stack 容器，图片在底层，点击区域在上层
		clickableImage := container.NewStack(image, clickContainer)

		// 添加提示文字
		tipLabel := widget.NewLabel("💡 点击图片可放大查看")
		tipLabel.TextStyle = fyne.TextStyle{Italic: true}
		tipLabel.Alignment = fyne.TextAlignCenter

		mainContent = container.NewVBox(
			contentLabel,
			widget.NewSeparator(),
			container.NewCenter(clickableImage),
			tipLabel,
		)
	} else {
		mainContent = contentLabel
	}

	// 如果有按钮，添加按钮
	if t.pages[t.current].ShowButton {
		button := widget.NewButton(t.pages[t.current].ButtonText, func() {
			u, err := url.Parse(t.pages[t.current].ButtonURL)
			if err == nil && u != nil {
				fyne.CurrentApp().OpenURL(u)
			}
		})
		button.Importance = widget.HighImportance

		mainContent = container.NewVBox(
			mainContent,
			container.NewCenter(button),
		)
	}

	contentScroll := container.NewScroll(mainContent)
	contentScroll.SetMinSize(fyne.NewSize(0, 450))

	pageLabel := widget.NewLabel("")
	t.updatePageLabel(pageLabel)

	// 声明按钮变量
	var prevButton, nextButton *widget.Button

	// 创建导航按钮
	prevButton = widget.NewButton("上一步", func() {
		if t.current > 0 {
			t.current--
			t.updateContent(titleLabel, contentLabel, contentScroll)
			t.updateButtons(prevButton, nextButton)
			t.updatePageLabel(pageLabel)
		}
	})

	nextButton = widget.NewButton("下一步", func() {
		if t.current < len(t.pages)-1 {
			t.current++
			t.updateContent(titleLabel, contentLabel, contentScroll)
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

	return container.NewBorder(
		titleLabel,
		navContainer,
		nil, nil,
		contentScroll,
	)
}

func (t *TutorialWithImages) updateContent(title, content *widget.Label, scroll *container.Scroll) {
	title.SetText(t.pages[t.current].Title)
	content.SetText(t.pages[t.current].Content)

	var mainContent fyne.CanvasObject

	// 如果当前页有图片，显示图片
	if t.pages[t.current].ImageData != nil {
		imageResource := fyne.NewStaticResource("tutorial-image", t.pages[t.current].ImageData)
		image := canvas.NewImageFromResource(imageResource)
		image.FillMode = canvas.ImageFillContain
		image.SetMinSize(fyne.NewSize(600, 400))

		// 创建完全透明的矩形作为点击层
		clickRect := canvas.NewRectangle(color.RGBA{0, 0, 0, 0}) // 完全透明
		clickRect.Resize(image.Size())
		
		// 创建点击区域容器
		clickContainer := &TappableContainer{
			rect: clickRect,
			onTap: func() {
				t.showLargeImage(imageResource)
			},
		}
		clickContainer.ExtendBaseWidget(clickContainer)
		clickContainer.Resize(image.Size())
		
		// 使用 Stack 容器，图片在底层，点击区域在上层
		clickableImage := container.NewStack(image, clickContainer)

		// 添加提示文字
		tipLabel := widget.NewLabel("💡 点击图片可放大查看")
		tipLabel.TextStyle = fyne.TextStyle{Italic: true}
		tipLabel.Alignment = fyne.TextAlignCenter

		mainContent = container.NewVBox(
			content,
			widget.NewSeparator(),
			container.NewCenter(clickableImage),
			tipLabel,
		)
	} else {
		mainContent = content
	}

	// 如果有按钮，添加按钮
	if t.pages[t.current].ShowButton {
		button := widget.NewButton(t.pages[t.current].ButtonText, func() {
			u, err := url.Parse(t.pages[t.current].ButtonURL)
			if err == nil && u != nil {
				fyne.CurrentApp().OpenURL(u)
			}
		})
		button.Importance = widget.HighImportance

		mainContent = container.NewVBox(
			mainContent,
			container.NewCenter(button),
		)
	}

	scroll.Content = mainContent
	scroll.Refresh()
}

func (t *TutorialWithImages) updateButtons(prev, next *widget.Button) {
	prev.Enable()
	next.Enable()

	if t.current == 0 {
		prev.Disable()
	}
	if t.current == len(t.pages)-1 {
		next.Disable()
	}
}

func (t *TutorialWithImages) updatePageLabel(label *widget.Label) {
	label.Alignment = fyne.TextAlignCenter
	label.SetText(fmt.Sprintf("%d / %d", t.current+1, len(t.pages)))
}

// showLargeImage 显示放大的图片
func (t *TutorialWithImages) showLargeImage(imageResource fyne.Resource) {
	// 创建放大的图片
	largeImage := canvas.NewImageFromResource(imageResource)
	largeImage.FillMode = canvas.ImageFillOriginal // 改为原始尺寸

	// 创建滚动容器以防图片太大
	imageScroll := container.NewScroll(largeImage)
	imageScroll.SetMinSize(fyne.NewSize(800, 500))

	// 创建关闭按钮
	closeBtn := widget.NewButton("关闭", nil)
	closeBtn.Importance = widget.HighImportance

	// 使用 Border 布局，确保图片占据主要空间
	content := container.NewBorder(
		nil,                           // top
		container.NewCenter(closeBtn), // bottom
		nil, nil,                      // left, right
		imageScroll, // center
	)

	// 使用 NewCustomConfirm 并只显示确认按钮
	imageDialog := dialog.NewCustomConfirm("图片预览", "关闭", "", content, func(bool) {}, t.parent)

	// 设置关闭按钮的动作
	closeBtn.OnTapped = func() {
		imageDialog.Hide()
	}

	// 设置对话框大小为较大尺寸
	imageDialog.Resize(fyne.NewSize(1000, 700))
	imageDialog.Show()
}
