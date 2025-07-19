package assets

import (
	_ "embed"
)

//go:embed images/api-key-page.png
var APIKeyPageImage []byte

//go:embed images/create-api-key.png
var CreateAPIKeyImage []byte

//go:embed images/api-key-created.png
var APIKeyCreatedImage []byte

//go:embed images/select-key.png
var ClaudeFirstRunImage []byte