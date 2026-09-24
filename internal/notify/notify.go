// Package notify shows the desktop notification that accompanies attn
// playback: the full message, the project it came from, and reaction
// buttons (Stop / Replay / Copy text).
//
// Everything is best-effort: a missing session bus or notification server
// disables the UI but never fails playback.
package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MetaEnv carries the encoded Meta from the attn caller to the detached
// playback child.
const MetaEnv = "ATTN_NOTIFY_META"

// DefaultLinger is how long the notification (and its Replay / Copy
// buttons) stays live after playback ends.
const DefaultLinger = 15 * time.Minute

// Action keys used in Notify actions and ActionInvoked signals.
const (
	ActionStop   = "stop"
	ActionReplay = "replay"
	ActionCopy   = "copy"
)

// Meta describes one attn message for the notification.
type Meta struct {
	// Text is the full message as the caller wrote it.
	Text string `json:"text"`
	// Project is a short label for where the call came from (repo name).
	Project string `json:"project,omitempty"`
	// Origin is the caller's working directory, home-shortened.
	Origin string `json:"origin,omitempty"`
	Alert  bool   `json:"alert,omitempty"`
	// Linger keeps the notification live after playback; 0 closes it when
	// playback ends.
	Linger time.Duration `json:"linger,omitempty"`
	// Disabled suppresses the notification entirely.
	Disabled bool `json:"disabled,omitempty"`
}

// Encode serialises m for MetaEnv.
func (m Meta) Encode() string {
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// DecodeMeta parses a MetaEnv value. An empty or invalid value yields a
// generic Meta so playback still gets a (plain) notification.
func DecodeMeta(s string) Meta {
	var m Meta
	if s == "" || json.Unmarshal([]byte(s), &m) != nil {
		return Meta{}
	}
	return m
}

// Phase is the notification's lifecycle state.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
	PhaseStopped
)

// Spec is one fully-resolved org.freedesktop.Notifications.Notify call.
type Spec struct {
	AppName string
	Icon    string
	Summary string
	Body    string
	// Actions is the flat key,label,key,label list the spec expects.
	Actions []string
	Hints   map[string]any
	// Timeout in ms: -1 server default, 0 never expire.
	Timeout int32
}

// Build resolves the notification for m in phase p. markup reports whether
// the server parses the body as markup (then the text must be escaped).
func Build(m Meta, p Phase, markup bool) Spec {
	project := m.Project
	if project == "" {
		project = "attn"
	}
	body := m.Text
	if markup {
		body = EscapeMarkup(body)
	}

	urgency := byte(1)
	if m.Alert {
		urgency = 2
	}
	hints := map[string]any{
		"urgency": urgency,
		// Keep the notification when a button is pressed; attn replaces or
		// closes it itself.
		"resident": true,
		// attn is already making noise; don't stack the server's chime on it.
		"suppress-sound": true,
	}
	if m.Origin != "" {
		hints["x-kde-origin-name"] = m.Origin
	}

	s := Spec{AppName: "attn", Body: body, Hints: hints}
	switch p {
	case PhasePlaying:
		s.Icon = "audio-volume-high"
		s.Summary = "🔊 " + project
		s.Actions = []string{ActionStop, "Stop", ActionCopy, "Copy text"}
		// Stay up while speaking so Stop is reachable; replaced at the end.
		s.Timeout = 0
	case PhaseDone, PhaseStopped:
		s.Icon = "dialog-information"
		s.Summary = project
		if p == PhaseStopped {
			s.Summary += " (stopped)"
		}
		s.Actions = []string{ActionReplay, "Replay", ActionCopy, "Copy text"}
		s.Timeout = -1
	}
	if m.Alert {
		s.Icon = "dialog-warning"
		s.Summary = "⚠ " + s.Summary
	}
	return s
}

// EscapeMarkup escapes text for servers that parse the body as markup.
func EscapeMarkup(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

// Where returns (project, origin) labels for dir: the git repo name when dir
// is inside one (worktrees report their parent repo), else dir's base name;
// origin is dir with $HOME shortened to ~.
func Where(dir string) (project, origin string) {
	if dir == "" {
		return "", ""
	}
	origin = dir
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if dir == home {
			origin = "~"
		} else if rel, ok := strings.CutPrefix(dir, home+string(filepath.Separator)); ok {
			origin = "~/" + rel
		}
	}

	project = filepath.Base(dir)
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			project = filepath.Base(d)
			// <repo>/.worktrees/<name> → <repo> (<name>)
			parent := filepath.Dir(d)
			if filepath.Base(parent) == ".worktrees" {
				project = filepath.Base(filepath.Dir(parent)) + " (" + filepath.Base(d) + ")"
			}
			break
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	if origin == "~" {
		project = ""
	}
	return project, origin
}
