package history

// Interactive TUI for `attn history`, built on bubbletea + lipgloss.
// All rows are built through runewidth-aware truncation and lipgloss width
// constraints so the layout survives narrow terminals, wide text (CJK),
// emoji, and resizes without broken line wrapping.

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clankercode/attn/internal/audio"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/truncate"
)

const (
	minWidth  = 60
	minHeight = 12

	headerLines = 2 // title + filter/status line
	footerLines = 2 // status + key help
)

var (
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	styleHeaderMeta = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	styleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230"))

	stylePlaying = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	styleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleMissing = lipgloss.NewStyle().
			Foreground(lipgloss.Color("160"))

	styleDetailBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("238"))

	styleStatusErr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("160"))

	styleKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))
)

type playbackDoneMsg struct{ proc *playback }

type model struct {
	entries []Entry
	view    []int // indices into entries, after filtering
	cursor  int   // position within view
	listTop int   // first visible row within view

	width  int
	height int

	filter     textinput.Model
	filtering  bool
	filterText string

	detail viewport.Model

	playingPath string // path of entry currently playing, "" = none
	playProc    *playback

	confirmingDelete bool
	status           string
	statusIsErr      bool
}

// RunTUI starts the interactive browser and blocks until the user quits.
func RunTUI(entries []Entry) error {
	ti := textinput.New()
	ti.Placeholder = "filter by text, provider, or voice…"
	ti.Prompt = "/"
	ti.CharLimit = 128

	m := &model{
		entries: entries,
		filter:  ti,
	}
	m.applyFilter()

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) applyFilter() {
	m.view = m.view[:0]
	q := strings.ToLower(strings.TrimSpace(m.filterText))
	for i, e := range m.entries {
		if q == "" {
			m.view = append(m.view, i)
			continue
		}
		hay := strings.ToLower(e.Text + " " + e.Provider + " " + e.Voice + " " + e.Path)
		if strings.Contains(hay, q) {
			m.view = append(m.view, i)
		}
	}
	if m.cursor >= len(m.view) {
		m.cursor = max(0, len(m.view)-1)
	}
	m.clampScroll()
	m.refreshDetail()
}

func (m *model) selected() (Entry, bool) {
	if len(m.view) == 0 || m.cursor < 0 || m.cursor >= len(m.view) {
		return Entry{}, false
	}
	return m.entries[m.view[m.cursor]], true
}

func (m *model) listHeight() int {
	return max(1, m.height-headerLines-footerLines)
}

func (m *model) listWidth() int {
	w := m.width * 2 / 5
	return min(48, max(28, w))
}

func (m *model) clampScroll() {
	h := m.listHeight()
	if m.cursor < m.listTop {
		m.listTop = m.cursor
	}
	if m.cursor >= m.listTop+h {
		m.listTop = m.cursor - h + 1
	}
	maxTop := max(0, len(m.view)-h)
	if m.listTop > maxTop {
		m.listTop = maxTop
	}
	if m.listTop < 0 {
		m.listTop = 0
	}
}

func (m *model) move(delta int) {
	if len(m.view) == 0 {
		return
	}
	m.cursor = min(len(m.view)-1, max(0, m.cursor+delta))
	m.clampScroll()
	m.refreshDetail()
}

func (m *model) refreshDetail() {
	if m.detail.Width == 0 {
		return
	}
	e, ok := m.selected()
	if !ok {
		m.detail.SetContent(m.detailStyle().Render(styleDim.Render("No entries.")))
		return
	}
	m.detail.SetContent(m.renderDetail(e))
	m.detail.GotoTop()
}

func (m *model) detailStyle() lipgloss.Style {
	return styleDetailBorder.
		Width(m.detailWidth()).
		PaddingLeft(1)
}

func (m *model) detailWidth() int {
	w := m.width - m.listWidth() - 1
	return max(10, w)
}

func (m *model) renderDetail(e Entry) string {
	inner := max(8, m.detailWidth()-4) // border + padding
	wrap := lipgloss.NewStyle().Width(inner)

	var b strings.Builder
	b.WriteString(styleTitle.Render(e.Time.Local().Format("2006-01-02 15:04:05")))
	b.WriteString("\n\n")

	meta := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(styleHeaderMeta.Render(fmt.Sprintf("%-9s ", k)))
		b.WriteString(wrap.Render(v))
		b.WriteString("\n")
	}
	if e.Legacy {
		meta("origin", "legacy (predates history recording)")
	} else {
		meta("provider", e.Provider)
		meta("voice", e.Voice)
		meta("model", e.Model)
		meta("style", e.Style)
		if e.Alert {
			meta("alert", "yes")
		}
	}
	size := ""
	if e.Bytes > 0 {
		size = " (" + audio.FormatBytes(e.Bytes) + ")"
	}
	pathLine := e.Path + size
	if e.Missing {
		pathLine += "  " + styleMissing.Render("[file missing]")
	}
	meta("file", pathLine)

	if e.Text != "" {
		b.WriteString("\n")
		b.WriteString(styleHeaderMeta.Render("text"))
		b.WriteString("\n")
		b.WriteString(wrap.Render(e.Text))
		b.WriteString("\n")
	}
	if e.SpokenText != "" && e.SpokenText != e.Text {
		b.WriteString("\n")
		b.WriteString(styleHeaderMeta.Render("spoken"))
		b.WriteString("\n")
		b.WriteString(wrap.Render(e.SpokenText))
		b.WriteString("\n")
	}
	if m.playingPath != "" && m.playingPath == e.Path {
		b.WriteString("\n")
		b.WriteString(stylePlaying.Render("▶ playing"))
		b.WriteString("\n")
	}
	return m.detailStyle().Render(b.String())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		dw := m.detailWidth()
		m.detail.Width = dw
		m.detail.Height = m.listHeight()
		m.clampScroll()
		m.refreshDetail()
		return m, nil

	case playbackDoneMsg:
		if m.playProc == msg.proc {
			m.playProc = nil
			m.playingPath = ""
			m.setStatus("playback finished", false)
			m.refreshDetail()
		}
		return m, nil

	case tea.KeyMsg:
		// Fast typing or pastes arrive as one KeyMsg carrying several runes.
		// Replay each rune as its own keypress so bursts are not dropped.
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 {
			var cmds []tea.Cmd
			for _, r := range msg.Runes {
				cmds = append(cmds, m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}))
			}
			return m, tea.Batch(cmds...)
		}
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Filter input captures keys while focused.
	if m.filtering {
		return m.updateFilter(msg)
	}
	if m.confirmingDelete {
		return m.updateDeleteConfirm(msg)
	}
	return m.updateNormal(msg)
}

func (m *model) updateFilter(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		m.stopPlayback()
		return tea.Quit
	case "enter":
		m.filtering = false
		m.filter.Blur()
		return nil
	case "esc":
		m.filtering = false
		m.filter.Blur()
		m.filter.SetValue("")
		m.filterText = ""
		m.applyFilter()
		return nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.filterText = m.filter.Value()
	m.applyFilter()
	return cmd
}

func (m *model) updateDeleteConfirm(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		m.confirmingDelete = false
		e, ok := m.selected()
		if !ok {
			return nil
		}
		if m.playingPath != "" && m.playingPath == e.Path {
			m.stopPlayback()
		}
		if err := Delete(e); err != nil {
			m.setStatus("delete failed: "+err.Error(), true)
			return nil
		}
		m.reload()
		m.setStatus("deleted", false)
		return nil
	default:
		m.confirmingDelete = false
		m.setStatus("delete cancelled", false)
		return nil
	}
}

func (m *model) updateNormal(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		m.stopPlayback()
		return tea.Quit
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup", "ctrl+u":
		m.move(-m.listHeight())
	case "pgdown", "ctrl+d", "ctrl+f":
		m.move(m.listHeight())
	case "home", "g":
		m.move(-len(m.view))
	case "end", "G":
		m.move(len(m.view))
	case "/":
		m.filtering = true
		m.filter.Focus()
		return textinput.Blink
	case "esc":
		if m.filterText != "" {
			m.filter.SetValue("")
			m.filterText = ""
			m.applyFilter()
		}
	case "r":
		m.reload()
		m.setStatus("reloaded", false)
	case "d":
		if e, ok := m.selected(); ok {
			m.confirmingDelete = true
			m.setStatus("delete "+e.Label()+"? (y/n)", false)
		}
	case "enter", " ":
		return m.togglePlayback()
	}
	return nil
}

func (m *model) reload() {
	entries, err := Load()
	if err != nil {
		m.setStatus("reload failed: "+err.Error(), true)
		return
	}
	m.entries = entries
	m.applyFilter()
}

func (m *model) setStatus(s string, isErr bool) {
	m.status = s
	m.statusIsErr = isErr
}

func (m *model) togglePlayback() tea.Cmd {
	e, ok := m.selected()
	if !ok {
		return nil
	}
	if m.playingPath == e.Path {
		m.stopPlayback()
		m.setStatus("stopped", false)
		m.refreshDetail()
		return nil
	}
	if e.Missing {
		m.setStatus("audio file is missing: "+e.Path, true)
		return nil
	}
	m.stopPlayback()

	proc, err := startPlayback(e.Path)
	if err != nil {
		m.setStatus("playback failed: "+err.Error(), true)
		return nil
	}
	m.playProc = proc
	m.playingPath = e.Path
	m.setStatus("playing", false)
	m.refreshDetail()
	return waitForPlayback(proc)
}

func (m *model) stopPlayback() {
	if m.playProc != nil {
		m.playProc.stop()
		m.playProc = nil
	}
	m.playingPath = ""
}

func (m *model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		notice := fmt.Sprintf("Terminal too small (%dx%d).\nNeed at least %dx%d. Press q to quit.",
			m.width, m.height, minWidth, minHeight)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, notice)
	}

	header := m.renderHeader()
	body := m.renderBody()
	footer := m.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *model) renderHeader() string {
	count := fmt.Sprintf("%d entries", len(m.view))
	if m.filterText != "" {
		count += fmt.Sprintf(" (of %d)", len(m.entries))
	}
	left := styleTitle.Render("attn history")
	right := styleHeaderMeta.Render(count)
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	title := left + strings.Repeat(" ", gap) + right

	var filterLine string
	if m.filtering {
		filterLine = m.filter.View()
	} else if m.filterText != "" {
		filterLine = styleDim.Render("/" + m.filterText + "  (esc to clear)")
	} else {
		filterLine = styleDim.Render("press / to filter")
	}
	return truncateLine(title, m.width) + "\n" + truncateLine(filterLine, m.width)
}

func (m *model) renderBody() string {
	listW := m.listWidth()
	h := m.listHeight()

	var rows []string
	for row := 0; row < h; row++ {
		i := m.listTop + row
		if i >= len(m.view) {
			rows = append(rows, strings.Repeat(" ", listW))
			continue
		}
		rows = append(rows, m.renderRow(m.entries[m.view[i]], i == m.cursor, listW))
	}
	list := strings.Join(rows, "\n")

	detail := m.detail.View()
	return lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
}

func (m *model) renderRow(e Entry, selected bool, width int) string {
	marker := "  "
	if m.playingPath != "" && m.playingPath == e.Path {
		marker = stylePlaying.Render("▶ ")
	}
	ts := e.Time.Local().Format("01-02 15:04")

	var tag string
	switch {
	case e.Legacy:
		tag = "legacy"
	case e.Missing:
		tag = e.Provider + "·" + e.Voice + " ✗"
	default:
		tag = e.Provider + "·" + e.Voice
	}

	// Fixed columns: marker(2) + ts(11) + gap(1); rest split tag/label.
	prefix := marker + ts + " "
	avail := width - lipgloss.Width(prefix) - 1
	if avail < 8 {
		avail = 8
	}
	tagW := min(22, max(8, avail/3))
	labelW := avail - tagW - 1
	if labelW < 4 {
		labelW = 4
	}

	tag = runewidth.Truncate(tag, tagW, "…")
	label := runewidth.Truncate(oneLine(e.Label()), labelW, "…")

	row := prefix + runewidth.FillRight(tag, tagW) + " " + label
	if e.Missing {
		row = styleMissing.Render(row)
	} else if e.Legacy {
		row = styleDim.Render(row)
	}
	if selected {
		row = styleSelected.Width(width).Render(row)
	}
	return truncateLine(row, width)
}

func (m *model) renderFooter() string {
	var status string
	if m.statusIsErr {
		status = styleStatusErr.Render(m.status)
	} else if m.status != "" {
		status = stylePlaying.Render(m.status)
	}
	keys := styleKey.Render("↑/↓ navigate · enter play/stop · / filter · d delete · r reload · q quit")
	return truncateLine(status, m.width) + "\n" + truncateLine(keys, m.width)
}

// truncateLine hard-clips a rendered line to the given cell width so no row
// can ever wrap and corrupt the layout. ANSI-aware: escape sequences are not
// counted and are never split.
func truncateLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return truncate.String(s, uint(width))
}

// --- playback subprocess ---

type playback struct {
	cmd *exec.Cmd
}

// startPlayback plays path via a child copy of this binary using the existing
// --debug-play-file path, in its own process group so stop() can kill the
// whole tree (including pw-play/ffplay grandchildren).
func startPlayback(path string) (*playback, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}
	cmd := exec.Command(exe, "--debug-play-file", path, "--fg")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &playback{cmd: cmd}, nil
}

func (p *playback) stop() {
	if p.cmd.Process == nil {
		return
	}
	// Kill the whole process group; ignore errors (already exited).
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
}

func waitForPlayback(p *playback) tea.Cmd {
	return func() tea.Msg {
		_ = p.cmd.Wait()
		return playbackDoneMsg{proc: p}
	}
}
