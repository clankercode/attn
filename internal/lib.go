package internal

import (
	"context"
	"fmt"
	"os"

	"attn-tool/internal/audio"
	"attn-tool/internal/cli"
	"attn-tool/internal/tts"
)

func Run(args []string) {
	cfg := cli.Parse(args)

	if cfg.Text == "" {
		fmt.Fprintln(os.Stderr, "error: no text provided")
		os.Exit(1)
	}

	providerType := tts.ProviderType(cfg.Provider)
	provider := tts.NewProvider(providerType, cfg.Voice, cfg.Model)

	voice := cfg.Voice
	if voice == "" {
		voice = defaultVoice(providerType, cfg.Alert)
	}

	ctx := context.Background()
	audioOut, err := provider.Synthesize(ctx, cfg.Text, voice, cfg.Model)
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
			audio.PlayMpv(alertFile.Name())
		}
	}

	if err := audio.PlayAndSave(finalAudio, cfg.Output, true); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved to %s\n", cfg.Output)
}

func defaultVoice(pt tts.ProviderType, alert bool) string {
	if alert {
		switch pt {
		case tts.ProviderGroq:
			return "austin"
		default:
			return "male-tianmei"
		}
	}
	switch pt {
	case tts.ProviderGroq:
		return "austin"
	default:
		return "female-shaonv"
	}
}

func isFileOutput(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}
