package modes

import (
	"strings"
	"testing"

	"github.com/patriceckhart/zot/packages/agent/skills"
	"github.com/patriceckhart/zot/packages/tui"
)

// openSkillBody opens the /skills picker on one skill and enters its body
// view, the part of the dialog that renders a description, a source path,
// and prose that routinely exceed one terminal row.
func openSkillBody(t *testing.T, s *skills.Skill) *skillsDialog {
	t.Helper()
	d := newSkillsDialog()
	d.Open([]*skills.Skill{s})
	d.HandleKey(tui.Key{Kind: tui.KeyEnter})
	if d.viewing == nil {
		t.Fatal("enter did not open the body view")
	}
	return d
}

func TestSkillsDialogBodyWrapsLongLines(t *testing.T) {
	const width = 64
	description := "A deliberately long skill description that cannot fit on a single row of a narrow terminal window."
	path := "/home/someone/.local/state/zot/skills/engineering/deliberately-long-skill-name/SKILL.md"
	body := "# Deliberately long\n\n" +
		strings.Repeat("skill body prose that has to fold instead of running off the edge. ", 16) +
		"and one final tail sentence."

	d := openSkillBody(t, &skills.Skill{
		Name:        "wide",
		Description: description,
		Body:        body,
		Path:        path,
		Source:      "global",
	})

	rows := d.Render(tui.Theme{}, width)
	assertRowsFitWidth(t, rows, width)

	plain := strings.Join(rows, "\n")
	for _, word := range strings.Fields(description) {
		if !strings.Contains(plain, word) {
			t.Fatalf("wrapped description lost %q:\n%s", word, stripANSIBytes(plain))
		}
	}
	// The source path has no spaces, so it survives only when the dialog
	// folds it rather than handing an over-wide row to the renderer. A
	// path longer than one row is split mid-token, so compare it with the
	// whitespace the fold introduces removed.
	got := strings.NewReplacer(" ", "", "\n", "").Replace(stripANSIBytes(plain))
	if !strings.Contains(got, path) {
		t.Fatalf("wrapped source line lost its path:\n%s", stripANSIBytes(plain))
	}

	// Folding makes the body taller than the 16-row viewport, so the tail
	// must still be reachable by scrolling.
	for range 20 {
		d.HandleKey(tui.Key{Kind: tui.KeyPageDown})
	}
	scrolled := d.Render(tui.Theme{}, width)
	assertRowsFitWidth(t, scrolled, width)
	if got := stripANSIBytes(strings.Join(scrolled, "\n")); !strings.Contains(got, "final tail sentence.") {
		t.Fatalf("scrolled body lost its tail:\n%s", got)
	}
}

func TestSkillsDialogShortTerminalKeepsMetadataAndBodyReachable(t *testing.T) {
	const width, height, otherRows = 44, 24, 7
	d := openSkillBody(t, &skills.Skill{
		Name:        "wide",
		Description: strings.Repeat("description that wraps repeatedly ", 18),
		Source:      "global",
		Path:        "/long/skill/path/SKILL.md",
		Body:        "first body line\nsecond body line\nlast body line",
	})
	d.fitBodyRows(height, otherRows)
	render := func() string {
		rows := padDialogFrame(d.Render(tui.Theme{}, width))
		// Account for the editor/status band and the renderer's bottom margin.
		if len(rows)+otherRows+1 > height {
			t.Fatalf("dialog has %d rows with %d reserved, terminal has %d", len(rows), otherRows+1, height)
		}
		assertRowsFitWidth(t, rows, width)
		return stripANSIBytes(strings.Join(rows, "\n"))
	}
	if got := render(); !strings.Contains(got, "description that wraps") {
		t.Fatalf("metadata start not visible: %s", got)
	}
	foundPath, foundBody := false, false
	for range 40 {
		visible := render()
		foundPath = foundPath || strings.Contains(visible, "/long/skill/path/SKILL.md")
		foundBody = foundBody || strings.Contains(visible, "first body line")
		if foundPath && foundBody {
			break
		}
		d.HandleKey(tui.Key{Kind: tui.KeyDown})
	}
	if !foundPath || !foundBody {
		t.Fatalf("source and first body line must both be reachable, source=%t body=%t", foundPath, foundBody)
	}
	for range 40 {
		d.HandleKey(tui.Key{Kind: tui.KeyUp})
	}
	if got := render(); !strings.Contains(got, "description that wraps") {
		t.Fatalf("metadata start unreachable after scrolling: %s", got)
	}
}

func TestSkillsDialogWrapsHintLines(t *testing.T) {
	const width = 44
	d := newSkillsDialog()
	d.canPin = true
	d.Open(nil)

	rows := d.Render(tui.Theme{}, width)
	assertRowsFitWidth(t, rows, width)

	plain := stripANSIBytes(strings.Join(rows, "\n"))
	for _, want := range []string{"project pin", "toggle", "no user skills loaded", "$ZOT_HOME/skills", ".agents/skills"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("wrapped hint lost %q:\n%s", want, plain)
		}
	}
}
