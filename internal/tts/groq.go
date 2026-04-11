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

type groqProvider struct {
	voice string
	model string
}

func newGroq(voice, model string) Provider {
	if voice == "" {
		voice = "af_bella-Aurora"
	}
	if model == "" {
		model = "canopylabs/orpheus-v1-english"
	}
	return &groqProvider{voice: voice, model: model}
}

func (g *groqProvider) Name() string { return "groq" }

func (g *groqProvider) Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY not set")
	}

	if voice == "" {
		voice = g.voice
	}
	if model == "" {
		model = g.model
	}

	url := "https://api.groq.com/openai/v1/audio/speech"
	payload := map[string]any{
		"model": model,
		"voice": voice,
		"input": text,
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
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq API error: %s — %s", resp.Status, string(bodyBytes))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &AudioOutput{Data: data}, nil
}
