package internal

import (
	"context"
	"fmt"
	"os"
	"unicode"

	"attn-tool/internal/audio"
	"attn-tool/internal/cli"
	"attn-tool/internal/tts"
)

func Run(args []string) {
	cfg := cli.Parse(args)

	providerType := tts.ProviderType(cfg.Provider)

	if cfg.ListVoices {
		printVoices(providerType)
		return
	}

	if cfg.Text == "" {
		fmt.Fprintln(os.Stderr, "error: no text provided")
		os.Exit(1)
	}

	text := cfg.Text
	if cfg.Polish {
		polished := polishText(text)
		text = polished
		fmt.Printf("[polished] %s → %s\n", cfg.Text, text)
	}

	provider := tts.NewProvider(providerType, cfg.Voice, cfg.Model)

	voice := cfg.Voice
	if voice == "" {
		voice = defaultVoice(providerType, cfg.Alert)
	}

	ctx := context.Background()
	audioOut, err := provider.Synthesize(ctx, text, voice, cfg.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	finalAudio := audioOut.Data

	if cfg.Alert {
		alertFile, err := os.CreateTemp("", "attn-alert-*.wav")
		if err == nil {
			alertFile.Write(audio.AlertTone())
			alertFile.Close()
			defer os.Remove(alertFile.Name())
			audio.PlayMpvBg(alertFile.Name())
		}
	}

	if err := audio.PlayAndSave(finalAudio, cfg.Output, true, cfg.Fg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved to %s\n", cfg.Output)
}

func defaultVoice(pt tts.ProviderType, alert bool) string {
	if alert {
		switch pt {
		case tts.ProviderGroq:
			return "daniel"
		default:
			return "Deep_Voice_Man"
		}
	}
	switch pt {
	case tts.ProviderGroq:
		return "daniel"
	default:
		return "Friendly_Person"
	}
}

func printVoices(pt tts.ProviderType) {
	switch pt {
	case tts.ProviderGroq:
		fmt.Println("Groq voices (canopylabs/orpheus-v1-english):")
		for _, v := range tts.VoiceListGroq {
			fmt.Printf("  %s\n", v)
		}
	case tts.ProviderMinimax:
		fmt.Println("MiniMax voices (speech-2.8-hd):")
		for _, v := range tts.VoiceListMinimax {
			fmt.Printf("  %s\n", v)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown provider: %s\n", pt)
		os.Exit(1)
	}
}

func polishText(text string) string {
	runes := []rune(text)
	if len(runes) > 0 && unicode.IsLetter(runes[len(runes)-1]) {
		return "... " + string(runes) + "."
	}
	return "... " + text
}
