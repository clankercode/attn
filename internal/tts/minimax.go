package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type minimaxProvider struct {
	voice string
	model string
}

func newMinimax(voice, model string) Provider {
	if voice == "" {
		voice = "female-shaonv"
	}
	if model == "" {
		model = "speech-2.8-turbo"
	}
	return &minimaxProvider{voice: voice, model: model}
}

func (m *minimaxProvider) Name() string { return "minimax" }

func (m *minimaxProvider) Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("MINIMAX_API_KEY not set")
	}

	if voice == "" {
		voice = m.voice
	}
	if model == "" {
		model = m.model
	}

	url := "https://api.minimax.io/v1/t2a_v2"
	payload := map[string]any{
		"model": model,
		"text":  text,
		"voice_setting": map[string]any{
			"voice_id": voice,
			"speed":    1.0,
			"vol":      1.0,
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("minimax API error: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &AudioOutput{Data: data}, nil
}
