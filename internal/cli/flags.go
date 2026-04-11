package cli

import (
	"flag"
	"fmt"
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
	Text       string
	Output     string
	Provider   string
	Voice      string
	Model      string
	Alert      bool
	Fg         bool
	Polish     bool
	ListVoices bool
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

func Parse(args []string) Config {
	fs := flag.NewFlagSet("attn-tool", flag.ContinueOnError)
	fs.Usage = func() {
		println("Usage: attn [options] \"message\"")
		fs.PrintDefaults()
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
	)

	fs.Parse(args)

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
		ts := time.Now().Unix()
		home, _ := os.UserHomeDir()
		ext := "mp3"
		if providerVal == "groq" {
			ext = "wav"
		}
		outPath = home + "/.tts-output/" + fmt.Sprintf("%d", ts) + "." + ext
	}

	return Config{
		Text:       text,
		Output:     outPath,
		Provider:   providerVal,
		Voice:      *voice,
		Model:      *model,
		Alert:      *alert,
		Fg:         *fg,
		Polish:     *polish,
		ListVoices: *listVoices,
	}
}
