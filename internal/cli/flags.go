package cli

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Text     string
	Output   string
	Provider string
	Voice    string
	Model    string
	Alert    bool
}

func Parse(args []string) Config {
	fs := flag.NewFlagSet("attn-tool", flag.ContinueOnError)
	fs.Usage = func() {
		println("Usage: attn [options] \"message\"")
		fs.PrintDefaults()
	}

	var (
		output   = fs.String("o", "", "Output file path (default: ~/.tts-output/<timestamp>.mp3)")
		provider = fs.String("provider", os.Getenv("TTS_PROVIDER"), "Provider: minimax or groq")
		voice    = fs.String("voice", "", "Voice ID")
		model    = fs.String("model", "", "Model ID (provider-specific)")
		alert    = fs.Bool("alert", false, "Prepend alert tone and use alert voice")
	)

	fs.Parse(args)

	text := ""
	if args := fs.Args(); len(args) > 0 {
		text = args[0]
	}

	outPath := *output
	if outPath == "" {
		ts := time.Now().Unix()
		home, _ := os.UserHomeDir()
		outPath = home + "/.tts-output/" + fmt.Sprintf("%d", ts) + ".mp3"
	}

	providerVal := *provider
	if providerVal == "" {
		providerVal = "minimax"
	}

	return Config{
		Text:     text,
		Output:   outPath,
		Provider: providerVal,
		Voice:    *voice,
		Model:    *model,
		Alert:    *alert,
	}
}
