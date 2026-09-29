package comfyui

import "errors"

var (
	ErrProviderUnavailable = errors.New("comfyui provider unavailable")
	ErrProviderInvalid = errors.New("comfyui request rejected")
	ErrProviderCancelled = errors.New("comfyui operation cancelled")
)
