package tts

import (
	"bytes"
	"context"
	"encoding/hex"
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
		model = "speech-2.8-hd"
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

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var errResp map[string]any
		json.Unmarshal(bodyBytes, &errResp)
		if msg, ok := errResp["base_resp"].(map[string]any); ok {
			if code, ok := msg["status_code"].(float64); ok && code != 0 {
				return nil, fmt.Errorf("minimax API error %d: %v", int(code), msg["status_msg"])
			}
		}
		return nil, fmt.Errorf("minimax API error: %s", resp.Status)
	}

	var result struct {
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("minimax: parse response: %w", err)
	}

	if result.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("minimax API error %d: %s", result.BaseResp.StatusCode, result.BaseResp.StatusMsg)
	}

	audioBytes, err := hex.DecodeString(result.Data.Audio)
	if err != nil {
		return nil, fmt.Errorf("minimax: decode hex audio: %w", err)
	}

	return &AudioOutput{Data: audioBytes}, nil
}
