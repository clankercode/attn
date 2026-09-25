package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestBuildPlayingShowsFullEscapedTextAndStopCopy(t *testing.T) {
	long := strings.Repeat("word ", 200) + "a < b && c > d"
	s := Build(Meta{Text: long, Project: "hark", Origin: "~/src/hark"}, PhasePlaying, true)

	if !strings.HasSuffix(s.Body, "a &lt; b &amp;&amp; c &gt; d") || len(s.Body) < len(long) {
		t.Fatalf("body must be the full, escaped message; got %q", s.Body[len(s.Body)-40:])
	}
	if s.Summary != "Speaking — hark" {
		t.Fatalf("summary = %q", s.Summary)
	}
	wantActions := playingActions()
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
	if s.Body != "a < b" || s.Summary != "Speaking — attn" {
		t.Fatalf("body=%q summary=%q", s.Body, s.Summary)
	}
}

func TestBuildDoneStoppedSkippedAndAlert(t *testing.T) {
	want := lingerActions()
	done := Build(Meta{Text: "x", Project: "p"}, PhaseDone, true)
	if done.Summary != "Finished — p" || done.Timeout != -1 || strings.Join(done.Actions, ",") != strings.Join(want, ",") {
		t.Fatalf("done = %+v", done)
	}
	stopped := Build(Meta{Text: "x", Project: "p"}, PhaseStopped, true)
	if stopped.Summary != "Stopped — p" || strings.Join(stopped.Actions, ",") != strings.Join(want, ",") {
		t.Fatalf("stopped = %+v", stopped)
	}
	skipped := Build(Meta{Text: "x", Project: "p"}, PhaseSkipped, true)
	if skipped.Summary != "Skipped — p" || strings.Join(skipped.Actions, ",") != strings.Join(want, ",") || skipped.Timeout != -1 {
		t.Fatalf("skipped = %+v", skipped)
	}
	alert := Build(Meta{Text: "x", Project: "p", Alert: true}, PhasePlaying, true)
	if alert.Hints["urgency"] != byte(2) || alert.Summary != "Speaking — p" || alert.Icon != "dialog-warning" {
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

func TestBuildBusy(t *testing.T) {
	s := Build(Meta{Text: "x", Project: "p"}, PhaseBusy, true)
	if s.Summary != "Busy — p (try Replay again)" || strings.Join(s.Actions, ",") != strings.Join(lingerActions(), ",") {
		t.Fatalf("busy = %+v", s)
	}
}

func TestBuildSanitizesBodyWithAndWithoutMarkup(t *testing.T) {
	in := "\x1b[31mred\x1b[0m\tok\r\nnext\x00\x07\x7f \ufffe\uffff<b>é\xff"
	for _, markup := range []bool{false, true} {
		got := Build(Meta{Text: in}, PhasePlaying, markup).Body
		want := "[31mred[0m\tok\nnext <b>é\ufffd"
		if markup {
			want = "[31mred[0m\tok\nnext &lt;b&gt;é\ufffd"
		}
		if got != want {
			t.Errorf("markup=%v: body = %q, want %q", markup, got, want)
		}
	}
}

func TestEncodeCapsTextAndKeepsHTML(t *testing.T) {
	long := strings.Repeat("é", 20000) // 40000 bytes
	enc := Meta{Text: long, Project: "a<b>&c"}.Encode()
	if strings.Contains(enc, "\\u003c") || strings.HasSuffix(enc, "\n") {
		t.Fatalf("encoding must not HTML-escape or end in a newline: %q", enc[:40])
	}
	m := DecodeMeta(enc)
	if m.Project != "a<b>&c" {
		t.Fatalf("project = %q", m.Project)
	}
	if len(m.Text) > maxEnvText || !strings.HasSuffix(m.Text, "…") || !utf8.ValidString(m.Text) {
		t.Fatalf("text len=%d suffix=%q valid=%v", len(m.Text), m.Text[len(m.Text)-6:], utf8.ValidString(m.Text))
	}
	if !strings.HasPrefix(long, strings.TrimSuffix(m.Text, "…")) {
		t.Fatal("truncated text must be a prefix of the original")
	}
	short := Meta{Text: "short"}
	if DecodeMeta(short.Encode()) != short {
		t.Fatal("short text must round-trip unchanged")
	}
}
