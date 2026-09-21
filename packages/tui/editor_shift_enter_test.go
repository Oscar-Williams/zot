package tui

import "testing"

func TestEditorShiftEnterInsertsNewline(t *testing.T) {
	e := NewEditor("> ")
	e.HandleKey(Key{Kind: KeyRune, Rune: 'a'})
	if submit := e.HandleKey(Key{Kind: KeyEnter, Shift: true}); submit {
		t.Fatal("Shift+Enter submitted; want newline")
	}
	e.HandleKey(Key{Kind: KeyRune, Rune: 'b'})

	if got, want := e.Value(), "a\nb"; got != want {
		t.Fatalf("Value() = %q, want %q", got, want)
	}
}

func TestEditorIgnoresCtrlAndSuperRunes(t *testing.T) {
	e := NewEditor("> ")
	e.HandleKey(Key{Kind: KeyRune, Rune: 'a'})
	for _, k := range []Key{
		{Kind: KeyRune, Rune: 's', Ctrl: true},
		{Kind: KeyRune, Rune: 'G', Ctrl: true, Shift: true},
		{Kind: KeyRune, Rune: 'k', Super: true},
	} {
		if submit := e.HandleKey(k); submit {
			t.Fatalf("%+v submitted", k)
		}
	}
	if got, want := e.Value(), "a"; got != want {
		t.Fatalf("Value() = %q, want %q (chords must not insert text)", got, want)
	}
}

func TestEditorPlainEnterSubmits(t *testing.T) {
	e := NewEditor("> ")
	e.HandleKey(Key{Kind: KeyRune, Rune: 'a'})
	if submit := e.HandleKey(Key{Kind: KeyEnter}); !submit {
		t.Fatal("Enter did not submit")
	}
}
