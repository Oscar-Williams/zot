package modes

import (
	"fmt"
	"slices"
	"strings"

	"github.com/patriceckhart/zot/packages/agent/skills"
	"github.com/patriceckhart/zot/packages/tui"
)

// skillsDialog lists discovered skills, their pin scopes, and their bodies.
// Pin persistence is handled by the interactive host.
type skillsDialog struct {
	active  bool
	pins    skills.Pins
	canPin  bool
	skills  []*skills.Skill
	cursor  int
	viewing *skills.Skill // when non-nil, render the body instead of the list
	scroll  int           // body view scroll offset (in wrapped lines)
	maxRows int           // scrollable rows in the body view, 0 uses the default
}

func newSkillsDialog() *skillsDialog { return &skillsDialog{} }

// Open populates and shows the dialog with the given snapshot.
func (d *skillsDialog) Open(s []*skills.Skill) {
	d.active = true
	d.skills = s
	d.cursor = 0
	d.viewing = nil
	d.scroll = 0
}

// Close hides the dialog.
func (d *skillsDialog) Close() { d.active = false }

// Active reports whether the dialog is visible.
func (d *skillsDialog) Active() bool { return d != nil && d.active }

// HandleKey advances the dialog state.
func (d *skillsDialog) HandleKey(k tui.Key) (closed bool) {
	if !d.Active() {
		return false
	}

	if d.viewing != nil {
		// Body view keys.
		switch k.Kind {
		case tui.KeyEsc, tui.KeyEnter:
			d.viewing = nil
			d.scroll = 0
		case tui.KeyUp:
			if d.scroll > 0 {
				d.scroll--
			}
		case tui.KeyDown:
			d.scroll++
		case tui.KeyPageUp:
			d.scroll -= 8
			if d.scroll < 0 {
				d.scroll = 0
			}
		case tui.KeyPageDown:
			d.scroll += 8
		}
		return false
	}

	// List view keys.
	switch k.Kind {
	case tui.KeyEsc:
		d.Close()
		return true
	case tui.KeyUp:
		if d.cursor > 0 {
			d.cursor--
		}
	case tui.KeyDown:
		if d.cursor < len(d.skills)-1 {
			d.cursor++
		}
	case tui.KeyEnter:
		if len(d.skills) > 0 {
			d.viewing = d.skills[d.cursor]
			d.scroll = 0
		}
	}
	return false
}

// Render draws the picker or the body view.
func (d *skillsDialog) Render(th tui.Theme, width int) []string {
	if !d.Active() {
		return nil
	}

	if d.viewing != nil {
		return d.renderBody(th, width)
	}

	out := []string{frameHeader(th, "skills (enter to view, esc to close)", width)}
	if d.canPin {
		out = append(out, wrapDialogMutedRows(th, "p: project pin, g: global pin (toggle)", width)...)
	}
	if len(d.skills) == 0 {
		out = append(out, wrapDialogMutedRows(th, "no user skills loaded", width)...)
		out = append(out, wrapDialogMutedRows(th, "add SKILL.md under $ZOT_HOME/skills, .zot/skills, .claude/skills, or .agents/skills", width)...)
		out = append(out, frameRule(th, width))
		return out
	}

	const maxRows = 12
	start, end := visibleWindow(d.cursor, len(d.skills), maxRows)
	if start > 0 {
		out = append(out, "  "+th.FG256(th.Muted, fmt.Sprintf("\u2191 %d more above", start)))
	}
	for i := start; i < end; i++ {
		s := d.skills[i]
		marker := ""
		if d.canPin {
			project, global := "-", "-"
			if slices.Contains(d.pins.Project, s.Name) {
				project = "p"
			}
			if slices.Contains(d.pins.Global, s.Name) {
				global = "g"
			}
			marker = "[" + project + global + "] "
		}
		row := marker + formatSkillRow(s, width-2-len(marker))
		if i == d.cursor {
			out = append(out, th.PadHighlight("  "+row, width))
		} else {
			out = append(out, "  "+th.FG256(th.Muted, row))
		}
	}
	if end < len(d.skills) {
		out = append(out, "  "+th.FG256(th.Muted, fmt.Sprintf("\u2193 %d more below", len(d.skills)-end)))
	}
	out = append(out, "  "+th.FG256(th.Muted, "run with /skill:<name> [request]"))
	out = append(out, frameRule(th, width))
	return out
}

// fitBodyRows reserves the fixed dialog chrome and the rows outside the dialog.
// The metadata scrolls with the body so even a long path cannot hide its start.
func (d *skillsDialog) fitBodyRows(terminalRows, otherRows int) {
	// Header, rule, scroll hint, two frame gaps, and the renderer's bottom margin.
	d.maxRows = max(1, min(16, terminalRows-otherRows-6))
}

func (d *skillsDialog) renderBody(th tui.Theme, width int) []string {
	s := d.viewing
	out := []string{frameHeader(th, "skill: "+s.Name+"  (esc / enter to go back)", width)}
	content := wrapDialogMutedRows(th, s.Description, width)
	content = append(content, wrapDialogMutedRows(th, "source: "+s.Source+"  ("+s.Path+")", width)...)
	content = append(content, "")

	// Fold the markdown body to the dialog width. The renderer
	// hard-truncates over-wide rows, and skill bodies are prose that
	// routinely exceeds one row, so an unfolded body silently loses
	// everything past the right edge.
	content = append(content, renderDialogMarkdownRows(s.Body, th, width)...)

	maxRows := d.maxRows
	if maxRows <= 0 || maxRows > 16 {
		maxRows = 16
	}
	if d.scroll > len(content)-1 {
		d.scroll = len(content) - 1
	}
	if d.scroll < 0 {
		d.scroll = 0
	}
	end := d.scroll + maxRows
	if end > len(content) {
		end = len(content)
	}
	out = append(out, content[d.scroll:end]...)
	if end < len(content) {
		out = append(out, "  "+th.FG256(th.Muted, fmt.Sprintf("\u2193 %d more lines (down/pgdn)", len(content)-end)))
	}
	out = append(out, frameRule(th, width))
	return out
}

// wrapDialogMutedRows folds one muted dialog line to the width available
// after the dialog's 2-space indent and returns ready-to-print rows.
func wrapDialogMutedRows(th tui.Theme, text string, width int) []string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	wrapped := tui.WrapANSILine(text, inner)
	out := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		out = append(out, "  "+th.FG256(th.Muted, line))
	}
	return out
}

func formatSkillRow(s *skills.Skill, maxWidth int) string {
	left := fmt.Sprintf("%-20s  ", truncateLineSafe(s.Name, 20))
	source := s.Source
	if s.DisableModelInvocation {
		source = "manual, " + source
	}
	src := "  " + truncateLineSafe(source, 16)
	room := maxWidth - len(left) - len(src)
	if room < 10 {
		room = 10
	}
	desc := s.Description
	if len(desc) > room {
		if room <= 3 {
			desc = strings.Repeat(".", room)
		} else {
			desc = desc[:room-3] + "..."
		}
	}
	return left + desc + src
}

// truncateLineSafe limits s to n runes (not bytes) so multibyte
// names + sources don't blow past the column.
func truncateLineSafe(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 3 {
		return strings.Repeat(".", n)
	}
	return string(r[:n-3]) + "..."
}

// visibleWindow centers cursor in a window of size n within total
// items. Returns [start, end) bounds.
func visibleWindow(cursor, total, n int) (start, end int) {
	if total <= n {
		return 0, total
	}
	start = cursor - n/2
	if start < 0 {
		start = 0
	}
	end = start + n
	if end > total {
		end = total
		start = end - n
	}
	return
}
