package internal

import (
	"context"
	"fmt"
	"os"
	"time"
	"unicode"

	"github.com/clankercode/attn/internal/audio"
	"github.com/clankercode/attn/internal/cli"
	"github.com/clankercode/attn/internal/tts"
)

func Run(args []string) {
	if handled, err := audio.HandleDetachedPlayback(args); handled || err != nil {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := cli.Parse(args)
	if err != nil {
		os.Exit(2)
	}

	if cfg.DebugPlayFile != "" {
		if cfg.Fg {
			if err := audio.Play(cfg.DebugPlayFile); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		data, err := os.ReadFile(cfg.DebugPlayFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading debug file: %v\n", err)
			os.Exit(1)
		}
		tmpOutput := cfg.DebugPlayFile + ".attn-debug-tmp"
		if err := audio.PlayAndSave(data, tmpOutput, true, false, false); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Remove(tmpOutput)
		return
	}

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
	if cfg.Style != "" && providerType == tts.ProviderMimo {
		resolved := tts.ResolveStyle(cfg.Style)
		text = "<style>" + resolved + "</style>" + text
		fmt.Printf("[style] %s\n", resolved)
	}

	provider := tts.NewProvider(providerType, cfg.Voice, cfg.Model)

	voice := cfg.Voice
	if voice == "" {
		voice = defaultVoice(providerType, cfg.Alert)
	}

	if cfg.DryRun {
		fmt.Printf("[dry-run] would have saved to %s\n", cfg.Output)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

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
			audio.Play(alertFile.Name())
		}
	}

	if err := audio.PlayAndSave(finalAudio, cfg.Output, true, cfg.Fg, cfg.Wait); err != nil {
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
		case tts.ProviderMimo:
			return "mimo_default"
		default:
			return "Deep_Voice_Man"
		}
	}
	return tts.RandomVoice(pt)
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
	case tts.ProviderMimo:
		fmt.Println("MiMo voices (mimo-v2-tts):")
		for _, v := range tts.VoiceListMimo {
			fmt.Printf("  %s\n", v)
		}
		fmt.Println("\nStyle presets (use with --style):")
		for i, v := range tts.MimoStylePresets {
			fmt.Printf("  %s (%s)\n", v, tts.MimoStylePresetsEnglish[i])
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
