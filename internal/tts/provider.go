package tts

import "context"

type AudioOutput struct {
	Data []byte
}

type Provider interface {
	Name() string
	Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error)
}

type ProviderType string

const (
	ProviderMinimax ProviderType = "minimax"
	ProviderGroq    ProviderType = "groq"
)

func NewProvider(t ProviderType, voice, model string) Provider {
	switch t {
	case ProviderGroq:
		return newGroq(voice, model)
	default:
		return newMinimax(voice, model)
	}
}
