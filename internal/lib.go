package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/clankercode/attn/internal/audio"
	"github.com/clankercode/attn/internal/cli"
	"github.com/clankercode/attn/internal/history"
	"github.com/clankercode/attn/internal/tts"
)

// providerFactory is swappable in tests so run() can be exercised with fake
// TTS backends without making network calls.
var providerFactory = tts.NewProvider

func Run(args []string) {
	os.Exit(run(args))
}

func run(args []string) int {
	if handled, err := audio.HandleDetachedPlayback(args); handled || err != nil {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if len(args) > 0 && args[0] == "history" {
		history.RunCommand(args[1:])
		return 0
	}

	cfg, err := cli.Parse(args)
	if err != nil {
		return 2
	}

	if cfg.DebugPlayFile != "" {
		if cfg.Fg {
			if err := audio.Play(cfg.DebugPlayFile); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			return 0
		}
		data, err := os.ReadFile(cfg.DebugPlayFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading debug file: %v\n", err)
			return 1
		}
		tmpOutput := cfg.DebugPlayFile + ".attn-debug-tmp"
		if err := audio.PlayAndSave(data, tmpOutput, true, false, false); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		os.Remove(tmpOutput)
		return 0
	}

	providerType := tts.ProviderType(cfg.Provider)

	if cfg.ListVoices {
		return printVoices(providerType)
	}

	if cfg.Text == "" {
		fmt.Fprintln(os.Stderr, "error: no text provided")
		return 1
	}

	baseText := cfg.Text
	if cfg.Polish {
		polished := polishText(baseText)
		baseText = polished
		fmt.Printf("[polished] %s → %s\n", cfg.Text, baseText)
	}

	if cfg.DryRun {
		voice := tts.SelectVoice(providerType, cfg.VoicePrefs, cfg.Alert, cfg.Voice)
		fmt.Printf("[dry-run] provider=%s voice=%s → %s\n", providerType, voice, cfg.Output)
		return 0
	}

	fileCfg := cli.LoadConfig()
	spokenText, voice, finalAudio, err := synthesizeWithProviders(&cfg, baseText, fileCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if cfg.Alert && !cfg.Silent {
		alertFile, err := os.CreateTemp("", "attn-alert-*.wav")
		if err == nil {
			alertFile.Write(audio.AlertTone())
			alertFile.Close()
			defer os.Remove(alertFile.Name())
			audio.Play(alertFile.Name())
		}
	}

	if cfg.Silent {
		if err := audio.Save(finalAudio, cfg.Output); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		recordHistory(cfg, spokenText, voice, finalAudio)
		fmt.Printf("Saved to %s (silent)\n", cfg.Output)
		return 0
	}

	if err := audio.PlayAndSave(finalAudio, cfg.Output, true, cfg.Fg, cfg.Wait); err != nil {
		// PlayAndSave writes the output file before playing, so a playback
		// failure can leave a valid artifact on disk. Record it so the file
		// is still browsable via `attn history`.
		if _, statErr := os.Stat(cfg.Output); statErr == nil {
			recordHistory(cfg, spokenText, voice, finalAudio)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	recordHistory(cfg, spokenText, voice, finalAudio)
	fmt.Printf("Saved to %s\n", cfg.Output)
	return 0
}

// synthesizeWithProviders tries each candidate provider until one succeeds.
// On success it updates cfg.Provider and cfg.Output (extension) to match the
// winning provider. When ProviderExplicit is true, only one candidate is tried.
func synthesizeWithProviders(cfg *cli.Config, baseText string, fileCfg *cli.ConfigFile) (spokenText, voice string, data []byte, err error) {
	candidates := cfg.ProviderCandidates
	if len(candidates) == 0 {
		candidates = []string{cfg.Provider}
	}
	if cfg.ProviderExplicit && len(candidates) > 1 {
		candidates = candidates[:1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var lastErr error
	for i, name := range candidates {
		pt := tts.ProviderType(name)
		prefs := cli.VoicePrefsFor(fileCfg, pt)
		if i == 0 && cfg.Provider == name {
			// Prefer already-resolved prefs for the first (default) provider.
			prefs = cfg.VoicePrefs
		}
		v := tts.SelectVoice(pt, prefs, cfg.Alert, cfg.Voice)

		text := baseText
		if cfg.Style != "" && pt == tts.ProviderMimo {
			resolved := tts.ResolveStyle(cfg.Style)
			text = "<style>" + resolved + "</style>" + text
			fmt.Printf("[style] %s\n", resolved)
		}

		provider := providerFactory(pt, v, cfg.Model)
		audioOut, synthErr := provider.Synthesize(ctx, text, v, cfg.Model)
		if synthErr != nil {
			lastErr = synthErr
			if i+1 < len(candidates) {
				fmt.Fprintf(os.Stderr, "warning: %s failed: %v; trying %s\n", name, synthErr, candidates[i+1])
			}
			continue
		}
		if audioOut == nil || len(audioOut.Data) == 0 {
			lastErr = fmt.Errorf("provider returned no audio")
			if i+1 < len(candidates) {
				fmt.Fprintf(os.Stderr, "warning: %s failed: %v; trying %s\n", name, lastErr, candidates[i+1])
			}
			continue
		}

		cfg.Provider = name
		cfg.Output = adjustOutputExt(cfg.Output, pt)
		cfg.VoicePrefs = prefs
		return text, v, audioOut.Data, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no providers available")
	}
	return "", "", nil, lastErr
}

// adjustOutputExt swaps .mp3/.wav on default cache paths to match the provider.
// Custom -o paths are left unchanged unless they end in .mp3 or .wav.
func adjustOutputExt(path string, pt tts.ProviderType) string {
	want := ".mp3"
	if pt == tts.ProviderGroq || pt == tts.ProviderMimo {
		want = ".wav"
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == want {
		return path
	}
	if ext == ".mp3" || ext == ".wav" {
		return strings.TrimSuffix(path, ext) + want
	}
	return path
}

// recordHistory appends the generation to the history log. Failures are
// non-fatal: history must never break TTS.
func recordHistory(cfg cli.Config, spokenText, voice string, data []byte) {
	if os.Getenv("ATTN_NO_HISTORY") == "1" {
		return
	}
	cwd, _ := os.Getwd() // best-effort; empty if unavailable
	err := history.Record(history.Entry{
		Time:       time.Now(),
		Text:       cfg.Text,
		SpokenText: spokenText,
		Provider:   cfg.Provider,
		Voice:      voice,
		Model:      cfg.Model,
		Style:      cfg.Style,
		Alert:      cfg.Alert,
		Path:       cfg.Output,
		Bytes:      len(data),
		CWD:        cwd,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: history not recorded: %v\n", err)
	}
}

func printVoices(pt tts.ProviderType) int {
	switch pt {
	case tts.ProviderGroq:
		fmt.Println("Groq voices (canopylabs/orpheus-v1-english):")
		for _, v := range tts.VoiceListGroq {
			fmt.Printf("  %s\n", v)
		}
	case tts.ProviderGrok:
		fmt.Println("Grok (xAI) voices (all built-in):")
		for _, v := range tts.VoiceListGrok {
			fmt.Printf("  %s\n", v)
		}
		fmt.Printf("\n%d voices. Custom IDs from the xAI console also work with --voice.\n", len(tts.VoiceListGrok))
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
		return 1
	}
	return 0
}

func polishText(text string) string {
	runes := []rune(text)
	if len(runes) > 0 && unicode.IsLetter(runes[len(runes)-1]) {
		return "... " + string(runes) + "."
	}
	return "... " + text
}
