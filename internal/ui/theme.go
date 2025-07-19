package ui

import (
	"image/color"
	
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type CustomTheme struct{}

func (m *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return color.RGBA{R: 0, G: 122, B: 255, A: 255} // iOS 蓝色
	case theme.ColorNameButton:
		return color.RGBA{R: 0, G: 122, B: 255, A: 255}
	case theme.ColorNameBackground:
		return color.RGBA{R: 248, G: 250, B: 252, A: 255} // 明亮的浅蓝灰色背景
	case theme.ColorNameForeground:
		return color.RGBA{R: 30, G: 41, B: 59, A: 255} // 深蓝灰色文字
	case theme.ColorNameDisabled:
		return color.RGBA{R: 156, G: 163, B: 175, A: 255} // 柔和的灰色
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 107, G: 114, B: 128, A: 255} // 中等灰色
	case theme.ColorNamePressed:
		return color.RGBA{R: 59, G: 130, B: 246, A: 255} // 按下时的蓝色
	case theme.ColorNameHover:
		return color.RGBA{R: 147, G: 197, B: 253, A: 255} // 悬停时的浅蓝色
	case theme.ColorNameFocus:
		return color.RGBA{R: 99, G: 102, B: 241, A: 255} // 聚焦时的紫蓝色
	case theme.ColorNameSelection:
		return color.RGBA{R: 219, G: 234, B: 254, A: 255} // 选择时的浅蓝色
	case theme.ColorNameSeparator:
		return color.RGBA{R: 229, G: 231, B: 235, A: 255} // 分隔线颜色
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 255, G: 255, B: 255, A: 255} // 输入框白色背景
	case theme.ColorNameMenuBackground:
		return color.RGBA{R: 255, G: 255, B: 255, A: 255} // 菜单白色背景
	case theme.ColorNameOverlayBackground:
		return color.RGBA{R: 255, G: 255, B: 255, A: 255} // 弹窗白色背景
	case theme.ColorNameError:
		return color.RGBA{R: 239, G: 68, B: 68, A: 255} // 红色错误
	case theme.ColorNameSuccess:
		return color.RGBA{R: 34, G: 197, B: 94, A: 255} // 绿色成功
	case theme.ColorNameWarning:
		return color.RGBA{R: 245, G: 158, B: 11, A: 255} // 橙色警告
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (m *CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	// 使用默认主题的字体，Fyne 2.6+ 会自动处理中文
	return theme.DefaultTheme().Font(style)
}

func (m *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 16
	case theme.SizeNameText:
		return 15
	}
	return theme.DefaultTheme().Size(name)
}

var (
	DefaultWindowSize = fyne.NewSize(1200, 1000) // 进一步增大高度
	SuccessColor     = color.RGBA{R: 52, G: 199, B: 89, A: 255}
	ErrorColor       = color.RGBA{R: 255, G: 59, B: 48, A: 255}
	WarningColor     = color.RGBA{R: 255, G: 149, B: 0, A: 255}
)