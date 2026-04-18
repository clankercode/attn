package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ConfigFile struct {
	Groq    GroqConfig    `yaml:"groq"`
	Minimax MinimaxConfig `yaml:"minimax"`
	Mimo    MimoConfig    `yaml:"mimo"`
}

type GroqConfig struct {
	APIKey string `yaml:"api_key"`
}

type MinimaxConfig struct {
	APIKey string `yaml:"api_key"`
}

type MimoConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

type Config struct {
	Text          string
	Output        string
	Provider      string
	Voice         string
	Model         string
	Style         string
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
		if os.Getenv("MIMO_API_KEY") == "" && cfg.Mimo.APIKey != "" {
			os.Setenv("MIMO_API_KEY", cfg.Mimo.APIKey)
		}
		if os.Getenv("MIMO_BASE_URL") == "" && cfg.Mimo.BaseURL != "" {
			os.Setenv("MIMO_BASE_URL", cfg.Mimo.BaseURL)
		}
	}

	if os.Getenv("MIMO_API_KEY") == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			data, err := os.ReadFile(home + "/.mimo-key")
			if err == nil {
				key := strings.TrimSpace(string(data))
				if key != "" {
					os.Setenv("MIMO_API_KEY", key)
				}
			}
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
		provider   = fs.String("provider", os.Getenv("TTS_PROVIDER"), "Provider: minimax, groq, or mimo")
		voice      = fs.String("voice", "", "Voice ID")
		model      = fs.String("model", "", "Model ID (provider-specific)")
		style      = fs.String("style", "", "MiMo style preset (e.g. 开心, Happy, 东北话)")
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
		} else if providerVal == "mimo" {
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
		Style:         *style,
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
  attn --provider mimo --voice default_zh --style 开心 "你好世界"
  attn --provider mimo --voice default_zh --style Happy "hello world"

Common flags:
  --provider minimax|groq|mimo  Choose the TTS backend
  --voice NAME              Pick a specific voice
  --style PRESET            MiMo style: 开心, Happy, 东北话, etc.
  --wait                    Queue behind current playback
  --fg                      Block until playback finishes
  --polish                  Add a leading pause and final punctuation
  --dry-run                 Skip synthesis/playback side effects
  --list-voices             Show voices for the selected provider
  -o PATH                   Save output to a specific file

Defaults:
  provider: minimax
  voice: random for normal playback, fixed alert voice for --alert
  output: ~/.tts-output/<unique timestamp>.mp3 (or .wav for groq/mimo)
`)
}
