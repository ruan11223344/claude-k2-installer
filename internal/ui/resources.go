package ui

import (
	_ "embed"
	"fyne.io/fyne/v2"
)

//go:embed contact_me_qr.png
var qrCodeData []byte

var QRCodeResource = fyne.NewStaticResource("contact_me_qr.png", qrCodeData)