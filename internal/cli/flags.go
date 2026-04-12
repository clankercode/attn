package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type ConfigFile struct {
	Groq    GroqConfig    `yaml:"groq"`
	Minimax MinimaxConfig `yaml:"minimax"`
}

type GroqConfig struct {
	APIKey string `yaml:"api_key"`
}

type MinimaxConfig struct {
	APIKey string `yaml:"api_key"`
}

type Config struct {
	Text          string
	Output        string
	Provider      string
	Voice         string
	Model         string
	Alert         bool
	Fg            bool
	Polish        bool
	ListVoices    bool
	DryRun        bool
	Wait          bool
	DebugPlayFile string
}

var globalConfig *ConfigFile

func loadConfig() *ConfigFile {
	if globalConfig != nil {
		return globalConfig
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".config", "attn", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	globalConfig = &cfg
	return &cfg
}

func init() {
	cfg := loadConfig()
	if cfg != nil {
		if os.Getenv("GROQ_API_KEY") == "" && cfg.Groq.APIKey != "" {
			os.Setenv("GROQ_API_KEY", cfg.Groq.APIKey)
		}
		if os.Getenv("MINIMAX_API_KEY") == "" && cfg.Minimax.APIKey != "" {
			os.Setenv("MINIMAX_API_KEY", cfg.Minimax.APIKey)
		}
	}
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("attn-tool", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		writeHelp(os.Stdout)
	}

	var (
		output     = fs.String("o", "", "Output file path (default: ~/.tts-output/<timestamp>.mp3)")
		provider   = fs.String("provider", os.Getenv("TTS_PROVIDER"), "Provider: minimax or groq")
		voice      = fs.String("voice", "", "Voice ID")
		model      = fs.String("model", "", "Model ID (provider-specific)")
		alert      = fs.Bool("alert", false, "Prepend alert tone and use alert voice")
		fg         = fs.Bool("fg", false, "Play in foreground (blocking)")
		polish     = fs.Bool("polish", false, "Add speech polish (leading pause, trailing punctuation)")
		listVoices = fs.Bool("list-voices", false, "List available voices for the provider")
		dryRun     = fs.Bool("dry-run", false, "Simulate TTS without playing audio (for testing)")
		wait       = fs.Bool("wait", false, "Wait for any currently playing audio to finish before playing")
		debugPlay  = fs.String("debug-play-file", "", "Debug: play a file directly and exit (skip synthesis)")
	)

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	text := ""
	if args := fs.Args(); len(args) > 0 {
		text = args[0]
	}

	providerVal := *provider
	if providerVal == "" {
		providerVal = "minimax"
	}

	outPath := *output
	if outPath == "" {
		ts := time.Now().UnixNano()
		home, _ := os.UserHomeDir()
		ext := "mp3"
		if providerVal == "groq" {
			ext = "wav"
		}
		outPath = home + "/.tts-output/" + fmt.Sprintf("%d", ts) + "." + ext
	}

	return Config{
		Text:          text,
		Output:        outPath,
		Provider:      providerVal,
		Voice:         *voice,
		Model:         *model,
		Alert:         *alert,
		Fg:            *fg,
		Polish:        *polish,
		ListVoices:    *listVoices,
		DryRun:        *dryRun,
		Wait:          *wait,
		DebugPlayFile: *debugPlay,
	}, nil
}

func writeHelp(w io.Writer) {
	fmt.Fprint(w, `attn speaks text and saves the generated audio.

Examples:
  attn "Build finished."
  attn --wait "test two."
  attn --provider groq --voice daniel "Heads up."

Common flags:
  --provider minimax|groq   Choose the TTS backend
  --voice NAME              Pick a specific voice
  --wait                    Queue behind current playback
  --fg                      Block until playback finishes
  --polish                  Add a leading pause and final punctuation
  --dry-run                 Skip synthesis/playback side effects
  --list-voices             Show voices for the selected provider
  -o PATH                   Save output to a specific file

Defaults:
  provider: minimax
  voice: random for normal playback, fixed alert voice for --alert
  output: ~/.tts-output/<unique timestamp>.mp3 (or .wav for groq)
`)
}
