package ai

import (
	"moziboard-backend/internal/platform/config"

	"github.com/sashabaranov/go-openai"
)

func NewOpenAIClient(cfg config.Config) *openai.Client {
	if cfg.OpenAIKey == "" {
		return nil
	}
	return openai.NewClient(cfg.OpenAIKey)
}
