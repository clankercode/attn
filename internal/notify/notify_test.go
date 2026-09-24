package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildPlayingShowsFullEscapedTextAndStopCopy(t *testing.T) {
	long := strings.Repeat("word ", 200) + "a < b && c > d"
	s := Build(Meta{Text: long, Project: "hark", Origin: "~/src/hark"}, PhasePlaying, true)

	if !strings.HasSuffix(s.Body, "a &lt; b &amp;&amp; c &gt; d") || len(s.Body) < len(long) {
		t.Fatalf("body must be the full, escaped message; got %q", s.Body[len(s.Body)-40:])
	}
	if s.Summary != "🔊 hark" {
		t.Fatalf("summary = %q", s.Summary)
	}
	wantActions := []string{ActionStop, "Stop", ActionCopy, "Copy text"}
	if strings.Join(s.Actions, ",") != strings.Join(wantActions, ",") {
		t.Fatalf("actions = %v", s.Actions)
	}
	if s.Timeout != 0 {
		t.Fatalf("playing notification must not expire, timeout=%d", s.Timeout)
	}
	if s.Hints["suppress-sound"] != true || s.Hints["resident"] != true {
		t.Fatalf("expected suppress-sound and resident hints, got %v", s.Hints)
	}
	if s.Hints["x-kde-origin-name"] != "~/src/hark" {
		t.Fatalf("origin hint = %v", s.Hints["x-kde-origin-name"])
	}
	if s.Hints["urgency"] != byte(1) {
		t.Fatalf("urgency = %v", s.Hints["urgency"])
	}
}

func TestBuildNoMarkupLeavesTextRaw(t *testing.T) {
	s := Build(Meta{Text: "a < b"}, PhasePlaying, false)
	if s.Body != "a < b" || s.Summary != "🔊 attn" {
		t.Fatalf("body=%q summary=%q", s.Body, s.Summary)
	}
}

func TestBuildDoneStoppedAndAlert(t *testing.T) {
	done := Build(Meta{Text: "x", Project: "p"}, PhaseDone, true)
	if done.Summary != "p" || done.Actions[0] != ActionReplay || done.Timeout != -1 {
		t.Fatalf("done = %+v", done)
	}
	stopped := Build(Meta{Text: "x", Project: "p"}, PhaseStopped, true)
	if stopped.Summary != "p (stopped)" {
		t.Fatalf("stopped summary = %q", stopped.Summary)
	}
	alert := Build(Meta{Text: "x", Project: "p", Alert: true}, PhasePlaying, true)
	if alert.Hints["urgency"] != byte(2) || !strings.HasPrefix(alert.Summary, "⚠ ") || alert.Icon != "dialog-warning" {
		t.Fatalf("alert = %+v", alert)
	}
}

func TestMetaRoundTrip(t *testing.T) {
	m := Meta{Text: "héllo\n\"quoted\"", Project: "p", Origin: "~/x", Alert: true, Linger: 90 * time.Second}
	if got := DecodeMeta(m.Encode()); got != m {
		t.Fatalf("round trip: got %+v want %+v", got, m)
	}
	if got := DecodeMeta("{not json"); got != (Meta{}) {
		t.Fatalf("invalid meta should decode to zero value, got %+v", got)
	}
}

func TestWhereUsesRepoNameAndWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "src", "myrepo")
	wt := filepath.Join(repo, ".worktrees", "feat")
	sub := filepath.Join(wt, "internal", "pkg")
	for _, d := range []string{filepath.Join(repo, ".git"), sub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if p, o := Where(filepath.Join(repo, "cmd")); p != "myrepo" || o != "~/src/myrepo/cmd" {
		t.Fatalf("repo subdir: %q %q", p, o)
	}
	if p, _ := Where(sub); p != "myrepo (feat)" {
		t.Fatalf("worktree subdir: %q", p)
	}
	if p, o := Where(home); p != "" || o != "~" {
		t.Fatalf("home: %q %q", p, o)
	}
	plain := filepath.Join(home, "scratch")
	if p, _ := Where(plain); p != "scratch" {
		t.Fatalf("non-repo dir: %q", p)
	}
}
