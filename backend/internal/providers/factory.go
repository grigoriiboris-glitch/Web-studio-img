package providers

import (	"errors"

	"github.com/oleg3190/Web-studio-img/backend/internal/config"
	"github.com/oleg3190/Web-studio-img/backend/internal/generation"
	"github.com/oleg3190/Web-studio-img/backend/internal/providers/comfyui"
	"github.com/oleg3190/Web-studio-img/backend/internal/providers/yandexart"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

func NewImageProvider(cfg config.Config, objectStorage storage.StorageProvider) (generation.Provider, error) {
	switch cfg.ImageProvider {
	case "comfyui":
		if objectStorage == nil {
			return nil, errors.New("comfyui provider requires object storage")
		}
		return comfyui.New(comfyui.Config{
			Endpoint: cfg.ComfyUIEndpoint,
			Model: cfg.ComfyUIModel,
			ModelVersion: cfg.ComfyUIModelVersion,
			WorkflowJSON: cfg.ComfyUIWorkflowJSON,
			WorkflowPath: cfg.ComfyUIWorkflowPath,
			Timeout: cfg.ComfyUITimeout,
			PollEvery: cfg.ComfyUIPollEvery,
			Storage: objectStorage,
		})
	case "yandexart":
		if cfg.YandexARTAPIKey == "" || cfg.YandexARTFolderID == "" {
			return nil, errors.New("yandexart provider requires API key and folder id")
		}
		return yandexart.New(yandexart.Config{
			Endpoint: cfg.YandexARTEndpoint,
			OperationEndpoint: cfg.YandexARTOperationEndpoint,
			APIKey: cfg.YandexARTAPIKey,
			FolderID: cfg.YandexARTFolderID,
			Model: cfg.YandexARTModel,
		})
	default:
		return nil, errors.New("unsupported image provider")
	}
}

func ProviderReady(cfg config.Config, objectStorage storage.StorageProvider) bool {
	if objectStorage == nil || cfg.RedisURL == "" { return false }
	if cfg.ImageProvider == "comfyui" {
		return cfg.ComfyUIWorkflowJSON != "" || cfg.ComfyUIWorkflowPath != ""
	}
	return cfg.YandexARTAPIKey != "" && cfg.YandexARTFolderID != ""
}

