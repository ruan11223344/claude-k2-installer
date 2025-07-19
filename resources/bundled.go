// +build !dev

package resources

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// 使用 Fyne 默认的字体资源，它已经包含了对中文的支持
var resourceNotoSansRegular = theme.TextFont()
var resourceNotoSansBold = theme.TextBoldFont()
var resourceNotoSansItalic = theme.TextItalicFont()
var resourceNotoSansBoldItalic = theme.TextBoldItalicFont()
var resourceNotoMono = theme.TextMonospaceFont()

func init() {
	// Fyne 2.4+ 已经内置了对中文的支持
}