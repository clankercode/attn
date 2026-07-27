package history

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// RunCommand implements `attn history`. With a TTY it opens the interactive
// browser; otherwise it prints a plain list (scriptable / testable).
func RunCommand(args []string) {
	var unknown []string
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			printUsage()
			return
		}
		unknown = append(unknown, a)
	}
	if len(unknown) > 0 {
		fmt.Fprintf(os.Stderr, "attn history: ignoring unknown arguments: %s\n", strings.Join(unknown, " "))
	}

	entries, err := Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading history: %v\n", err)
		os.Exit(1)
	}

	if !isTerminal() {
		printPlain(entries)
		return
	}

	if err := RunTUI(entries); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func isTerminal() bool {
	fd := os.Stdout.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func printUsage() {
	fmt.Print(`attn history — browse past TTS generations

Usage:
  attn history           Open the interactive history browser (TUI)
  attn history --help    Show this help

When stdout is not a terminal, a plain list is printed instead.

Keys (TUI):
  ↑/k, ↓/j       Move selection          PgUp/PgDn   Page
  g / G          Top / bottom            Enter/Space Play or stop
  /              Filter entries          Esc         Clear filter / back
  d              Delete entry (asks y/n) r           Reload from disk
  q              Quit

History log: ` + Path() + `
Audio cache: ` + DefaultOutputDir() + `
Set ATTN_NO_HISTORY=1 to disable recording.
`)
}

func printPlain(entries []Entry) {
	if len(entries) == 0 {
		fmt.Println("No history yet.")
		return
	}
	for _, e := range entries {
		meta := e.Provider + "·" + e.Voice
		if e.Legacy {
			meta = "legacy"
		}
		flags := ""
		if e.Missing {
			flags = " [missing]"
		}
		cwd := abbrevPath(e.CWD)
		if cwd == "" {
			cwd = "-"
		}
		fmt.Printf("%s  %-24s  %-20s  %s%s\n\t%s\n",
			e.Time.Local().Format("2006-01-02 15:04:05"), cwd, meta, abbrevPath(e.Path), flags, oneLine(e.Label()))
	}
}

func oneLine(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			out = append(out, ' ')
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}
