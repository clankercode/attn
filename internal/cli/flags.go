package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clankercode/attn/internal/tts"
	"gopkg.in/yaml.v3"
)

// ConfigFile is the on-disk YAML schema for ~/.config/attn/config.yaml.
type ConfigFile struct {
	// ProviderPriority is the ordered list of preferred providers when
	// neither --provider nor TTS_PROVIDER is set. First known name wins.
	ProviderPriority []string `yaml:"provider_priority"`

	// Voices holds global preferred/banned lists applied when a provider
	// does not set its own preferred list. Banned lists are always merged.
	Voices GlobalVoices `yaml:"voices"`

	Groq    ProviderConfig `yaml:"groq"`
	Grok    ProviderConfig `yaml:"grok"`
	Minimax ProviderConfig `yaml:"minimax"`
	Mimo    MimoConfig     `yaml:"mimo"`
	Llmp    LlmpConfig     `yaml:"llmp"`
}

// GlobalVoices are cross-provider defaults.
type GlobalVoices struct {
	Preferred []string `yaml:"preferred"`
	Banned    []string `yaml:"banned"`
}

// ProviderConfig holds API credentials and voice preferences for one backend.
type ProviderConfig struct {
	APIKey     string   `yaml:"api_key"`
	Preferred  []string `yaml:"preferred"`
	Banned     []string `yaml:"banned"`
	AlertVoice string   `yaml:"alert_voice"`
}

// MimoConfig holds MiMo credentials, optional base URL, and voice prefs.
type MimoConfig struct {
	APIKey     string   `yaml:"api_key"`
	BaseURL    string   `yaml:"base_url"`
	Preferred  []string `yaml:"preferred"`
	Banned     []string `yaml:"banned"`
	AlertVoice string   `yaml:"alert_voice"`
}

// LlmpConfig holds LLMP gateway settings and llmp-grok voice prefs.
// KeyFile points at a Consumer key file (default ~/.llmp); BaseURL overrides
// the gateway OpenAI-compatible base (default https://omni-dyn-00.amaroolabs.com/v1).
type LlmpConfig struct {
	KeyFile    string   `yaml:"key_file"`
	BaseURL    string   `yaml:"base_url"`
	Preferred  []string `yaml:"preferred"`
	Banned     []string `yaml:"banned"`
	AlertVoice string   `yaml:"alert_voice"`
}

// Config is the resolved runtime configuration after flag parsing.
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
	Silent        bool
	Wait          bool
	DebugPlayFile string
	// VoicePrefs is the resolved preferred/banned/alert policy for Provider.
	VoicePrefs tts.VoicePrefs
	// ProviderExplicit is true when --provider or TTS_PROVIDER selected the
	// provider (no runtime fallback to other candidates).
	ProviderExplicit bool
	// ProviderCandidates is the ordered list of providers to try. When
	// ProviderExplicit is true this has a single element.
	ProviderCandidates []string
}

var (
	globalConfig     *ConfigFile
	globalConfigPath string // overridden in tests
)

func configPath() string {
	if globalConfigPath != "" {
		return globalConfigPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "attn", "config.yaml")
}

// LoadConfig reads and caches ~/.config/attn/config.yaml (or the test path).
// Returns nil when the file is missing. On invalid YAML, logs to stderr and
// returns nil so the tool still runs with built-in defaults.
func LoadConfig() *ConfigFile {
	if globalConfig != nil {
		return globalConfig
	}
	path := configPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Missing file is normal; do not warn.
		return nil
	}
	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring invalid config %s: %v\n", path, err)
		// Cache empty defaults so we do not re-warn, and callers get a
		// consistent "no custom settings" result.
		globalConfig = &ConfigFile{}
		return globalConfig
	}
	globalConfig = &cfg
	return globalConfig
}

// ResetConfigForTest clears the config cache and optionally sets a custom path.
// Pass an empty path to restore the default location.
func ResetConfigForTest(path string) {
	globalConfig = nil
	globalConfigPath = path
}

// VoicePrefsFor builds the effective VoicePrefs for a provider from a config file.
func VoicePrefsFor(cfg *ConfigFile, provider tts.ProviderType) tts.VoicePrefs {
	if cfg == nil {
		return tts.VoicePrefs{}
	}
	var pc ProviderConfig
	switch provider {
	case tts.ProviderGroq:
		pc = cfg.Groq
	case tts.ProviderGrok:
		pc = cfg.Grok
	case tts.ProviderLlmpGrok:
		pc = ProviderConfig{
			Preferred:  cfg.Llmp.Preferred,
			Banned:     cfg.Llmp.Banned,
			AlertVoice: cfg.Llmp.AlertVoice,
		}
	case tts.ProviderMimo:
		pc = ProviderConfig{
			APIKey:     cfg.Mimo.APIKey,
			Preferred:  cfg.Mimo.Preferred,
			Banned:     cfg.Mimo.Banned,
			AlertVoice: cfg.Mimo.AlertVoice,
		}
	default:
		pc = cfg.Minimax
	}

	preferred := pc.Preferred
	if len(preferred) == 0 {
		preferred = cfg.Voices.Preferred
	}
	banned := mergeUnique(cfg.Voices.Banned, pc.Banned)

	return tts.NormalizePrefs(tts.VoicePrefs{
		Preferred: append([]string(nil), preferred...),
		Banned:    banned,
		Alert:     pc.AlertVoice,
	})
}

func mergeUnique(lists ...[]string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, list := range lists {
		for _, item := range list {
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func envEmpty(name string) bool {
	return strings.TrimSpace(os.Getenv(name)) == ""
}

func applyAPIKeys(cfg *ConfigFile) {
	if cfg == nil {
		return
	}
	if envEmpty("GROQ_API_KEY") && cfg.Groq.APIKey != "" {
		os.Setenv("GROQ_API_KEY", cfg.Groq.APIKey)
	}
	// Prefer XAI_API_KEY (official); also accept GROK_API_KEY as alias.
	// Config keys only — Grok CLI OIDC tokens are NOT injected into the
	// environment (ResolveGrokAPIKey reads ~/.grok*/auth.json in-memory).
	if envEmpty("XAI_API_KEY") && envEmpty("GROK_API_KEY") && cfg.Grok.APIKey != "" {
		os.Setenv("XAI_API_KEY", cfg.Grok.APIKey)
	}
	if envEmpty("MINIMAX_API_KEY") && cfg.Minimax.APIKey != "" {
		os.Setenv("MINIMAX_API_KEY", cfg.Minimax.APIKey)
	}
	if envEmpty("MIMO_API_KEY") && cfg.Mimo.APIKey != "" {
		os.Setenv("MIMO_API_KEY", cfg.Mimo.APIKey)
	}
	if envEmpty("MIMO_BASE_URL") && cfg.Mimo.BaseURL != "" {
		os.Setenv("MIMO_BASE_URL", cfg.Mimo.BaseURL)
	}
	// Hand the llmp stanza to the tts package (in-memory; no env injection —
	// ResolveLLMPAPIKey reads the key file directly).
	tts.SetLLMPConfig(tts.LLMPConfig{
		KeyFile: cfg.Llmp.KeyFile,
		BaseURL: cfg.Llmp.BaseURL,
	})
}

func loadMimoKeyFile() {
	if !envEmpty("MIMO_API_KEY") {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(home, ".mimo-key"))
	if err != nil {
		return
	}
	key := strings.TrimSpace(string(data))
	if key != "" {
		os.Setenv("MIMO_API_KEY", key)
	}
}

func init() {
	applyAPIKeys(LoadConfig())
	loadMimoKeyFile()
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("attn-tool", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		writeHelp(os.Stdout)
	}

	var (
		output     = fs.String("o", "", "Output file path (default: ~/.tts-output/<timestamp>.mp3)")
		provider   = fs.String("provider", os.Getenv("TTS_PROVIDER"), "Provider: llmp-grok, minimax, groq, grok, or mimo")
		voice      = fs.String("voice", "", "Voice ID")
		model      = fs.String("model", "", "Model ID (provider-specific)")
		style      = fs.String("style", "", "MiMo style preset (e.g. 开心, Happy, 东北话)")
		alert      = fs.Bool("alert", false, "Prepend alert tone and use alert voice")
		fg         = fs.Bool("fg", false, "Play in foreground (blocking)")
		polish     = fs.Bool("polish", false, "Add speech polish (leading pause, trailing punctuation)")
		listVoices = fs.Bool("list-voices", false, "List available voices for the provider")
		dryRun     = fs.Bool("dry-run", false, "Simulate TTS without playing audio (for testing)")
		silent     = fs.Bool("silent", false, "Generate audio but skip playback (still saves to output file)")
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

	fileCfg := LoadConfig()
	var priority []string
	if fileCfg != nil {
		priority = fileCfg.ProviderPriority
	}

	explicitProvider := strings.TrimSpace(*provider)
	providerExplicit := explicitProvider != ""
	candidates := tts.ProviderCandidates(explicitProvider, priority)
	providerType := candidates[0]
	providerVal := string(providerType)

	prefs := VoicePrefsFor(fileCfg, providerType)

	outPath := *output
	if outPath == "" {
		ts := time.Now().UnixNano()
		home, _ := os.UserHomeDir()
		ext := "mp3"
		if providerVal == "groq" || providerVal == "mimo" {
			ext = "wav"
		}
		outPath = home + "/.tts-output/" + fmt.Sprintf("%d", ts) + "." + ext
	}

	candidateNames := make([]string, len(candidates))
	for i, c := range candidates {
		candidateNames[i] = string(c)
	}

	return Config{
		Text:               text,
		Output:             outPath,
		Provider:           providerVal,
		Voice:              *voice,
		Model:              *model,
		Style:              *style,
		Alert:              *alert,
		Fg:                 *fg,
		Polish:             *polish,
		ListVoices:         *listVoices,
		DryRun:             *dryRun,
		Silent:             *silent,
		Wait:               *wait,
		DebugPlayFile:      *debugPlay,
		VoicePrefs:         prefs,
		ProviderExplicit:   providerExplicit,
		ProviderCandidates: candidateNames,
	}, nil
}

func writeHelp(w io.Writer) {
	fmt.Fprint(w, `attn speaks text and saves the generated audio.

Examples:
  attn "Build finished."
  attn --wait "test two."
  attn --provider groq --voice daniel "Heads up."
  attn --provider grok --voice eve "Grok speaking."
  attn --provider mimo --voice default_zh --style 开心 "你好世界"
  attn --provider mimo --voice default_zh --style Happy "hello world"

Common flags:
  --provider llmp-grok|minimax|groq|grok|mimo  Choose the TTS backend
  --voice NAME              Pick a specific voice
  --model NAME              Model ID (provider-specific)
  --style PRESET            MiMo style: 开心, Happy, 东北话, etc.
  --alert                   Prepend alert tone and use alert voice
  --wait                    Queue behind current playback
  --fg                      Block until playback finishes
  --polish                  Add a leading pause and final punctuation
  --dry-run                 Skip synthesis/playback side effects
  --silent                  Generate audio but skip playback (still saves to file)
  --list-voices             Show voices for the selected provider
  -o PATH                   Save output to a specific file

Subcommands:
  history                   Browse past generations in an interactive TUI
                            (view text, provider, voice; replay cached audio)

Debug flags:
  --debug-play-file PATH    Play a file directly and exit (skip synthesis)

Defaults:
  provider: --provider > TTS_PROVIDER > provider_priority in config > llmp-grok, grok, mimo, minimax
            (auto-selected providers fall back through the priority list on failure;
             explicit --provider / TTS_PROVIDER does not fall back)
  voice: random from preferred pool (minus banned), or fixed alert_voice for --alert
  output: ~/.tts-output/<unique timestamp>.mp3 (or .wav for groq/mimo)
  history: JSONL at $XDG_DATA_HOME/attn/history.jsonl (ATTN_NO_HISTORY=1 to disable)
           records text, provider, voice, cwd, and path; older entries without cwd still load

Config file (~/.config/attn/config.yaml):
  provider_priority: [grok, mimo, minimax]   # fallback chain when provider is auto-selected
  voices:
    banned: [troy]                    # merged into every provider's bans
    # preferred: [...]                # only if valid for every provider that inherits it
  groq:
    api_key: ...
    preferred: [daniel, autumn]       # random pool (order is not ranked priority)
    banned: [troy]
    alert_voice: daniel
  grok:
    # api_key optional — auto-loads from XAI_API_KEY or ~/.grok*/auth.json
    preferred: [eve, ara, leo]
    alert_voice: rex
  minimax:
    api_key: ...
    preferred: [Deep_Voice_Man, Wise_Woman]
    alert_voice: Deep_Voice_Man
  mimo:
    api_key: ...
    preferred: [mimo_default]
    alert_voice: mimo_default
  llmp:
    # key_file: ~/.llmp           # Consumer key file (default ~/.llmp; LLMP_API_KEY / LLMP_KEY_FILE also work)
    # base_url: https://omni-dyn-00.amaroolabs.com/v1   # LLMP_BASE_URL also works
    preferred: [eve, ara, leo]    # same Grok voice roster as grok
    alert_voice: rex
`)
}
