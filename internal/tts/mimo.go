package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type mimoProvider struct {
	voice string
	model string
}

func newMimo(voice, model string) Provider {
	if voice == "" {
		voice = "mimo_default"
	}
	if model == "" {
		model = "mimo-v2-tts"
	}
	return &mimoProvider{voice: voice, model: model}
}

func (m *mimoProvider) Name() string { return "mimo" }

func (m *mimoProvider) Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error) {
	apiKey := os.Getenv("MIMO_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("MIMO_API_KEY not set")
	}

	if voice == "" {
		voice = m.voice
	}
	if model == "" {
		model = m.model
	}

	baseURL := os.Getenv("MIMO_BASE_URL")
	if baseURL == "" {
		baseURL = "https://token-plan-sgp.xiaomimimo.com/v1"
	}
	url := strings.TrimRight(baseURL, "/") + "/chat/completions"

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": ""},
			{"role": "assistant", "content": text},
		},
		"audio": map[string]string{
			"format": "wav",
			"voice":  voice,
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mimo API error: %s — %s", resp.Status, string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("mimo: parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("mimo: no choices in response")
	}

	audioB64 := result.Choices[0].Message.Audio.Data
	if audioB64 == "" {
		return nil, fmt.Errorf("mimo: no audio data in response")
	}

	audioBytes, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return nil, fmt.Errorf("mimo: decode base64 audio: %w", err)
	}

	audioBytes = prependWAVSilence(audioBytes, 4800) // 200ms at 24kHz mono 16-bit

	return &AudioOutput{Data: audioBytes}, nil
}

// prependWAVSilence inserts silenceSamples of leading silence into a WAV file.
// Assumes PCM 16-bit mono WAV (as returned by MiMo).
func prependWAVSilence(wav []byte, silenceSamples int) []byte {
	if len(wav) < 44 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return wav
	}

	// Find data chunk
	off := 12
	dataOff := -1
	dataSize := 0
	for off+8 <= len(wav) {
		cid := string(wav[off : off+4])
		csize := int(binary.LittleEndian.Uint32(wav[off+4 : off+8]))
		if cid == "data" {
			dataOff = off + 8
			dataSize = csize
			break
		}
		off += 8 + csize
		if off%2 == 1 {
			off++
		}
	}
	if dataOff < 0 {
		return wav
	}

	silenceBytes := make([]byte, silenceSamples*2) // 16-bit = 2 bytes per sample
	audioData := wav[dataOff : dataOff+dataSize]
	newDataSize := dataSize + len(silenceBytes)
	newFileSize := len(wav) + len(silenceBytes)

	out := make([]byte, 0, newFileSize)
	out = append(out, wav[:4]...)
	var riffSize [4]byte
	binary.LittleEndian.PutUint32(riffSize[:], uint32(newFileSize-8))
	out = append(out, riffSize[:]...)
	out = append(out, wav[8:dataOff]...)
	// Update data chunk size
	binary.LittleEndian.PutUint32(out[len(out)-4:], uint32(newDataSize))
	out = append(out, silenceBytes...)
	out = append(out, audioData...)
	return out
}
