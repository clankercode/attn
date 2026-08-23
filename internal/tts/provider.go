package tts

import (
	"context"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 90 * time.Second}

type AudioOutput struct {
	Data []byte
}

type Provider interface {
	Name() string
	Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error)
}

type ProviderType string

const (
	ProviderMinimax  ProviderType = "minimax"
	ProviderGroq     ProviderType = "groq"
	ProviderGrok     ProviderType = "grok"
	ProviderMimo     ProviderType = "mimo"
	ProviderLlmpGrok ProviderType = "llmp-grok"
)

func NewProvider(t ProviderType, voice, model string) Provider {
	switch t {
	case ProviderGroq:
		return newGroq(voice, model)
	case ProviderGrok:
		return newGrok(voice, model)
	case ProviderMimo:
		return newMimo(voice, model)
	case ProviderLlmpGrok:
		return newLlmpGrok(voice, model)
	default:
		return newMinimax(voice, model)
	}
}
