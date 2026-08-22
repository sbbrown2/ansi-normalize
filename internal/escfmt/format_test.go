package escfmt

import "testing"

func TestFormatSGRNormalization(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"\x1b[m", "\x1b[0m"},
		{"\x1b[01m", "\x1b[1m"},
		{"\x1b[01;032m", "\x1b[1;32m"},
		{"\x1b[38;5;001m", "\x1b[38;5;1m"},
		{"plain text, no escapes", "plain text, no escapes"},
	}
	for _, c := range cases {
		got, err := Format([]byte(c.in), false)
		if err != nil {
			t.Errorf("Format(%q) returned error: %v", c.in, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("Format(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatStrictRejectsUnknownEscape(t *testing.T) {
	_, err := Format([]byte("\x1bZ"), false)
	if err == nil {
		t.Fatal("expected an error for an unrecognized escape in strict mode")
	}
	if _, ok := err.(*MalformedError); !ok {
		t.Fatalf("expected *MalformedError, got %T", err)
	}
}

func TestFormatLenientPassesUnknownEscapeThrough(t *testing.T) {
	in := "\x1bZ"
	got, err := Format([]byte(in), true)
	if err != nil {
		t.Fatalf("unexpected error in lenient mode: %v", err)
	}
	if string(got) != in {
		t.Errorf("Format(%q, lenient) = %q, want unchanged input", in, got)
	}
}

func TestFormatCursorMovementNormalization(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// single-parameter movement commands default to a count of 1;
		// an explicit 1 or 0 is redundant and drops out.
		{"\x1b[0A", "\x1b[A"},
		{"\x1b[1A", "\x1b[A"},
		{"\x1b[01B", "\x1b[B"},
		{"\x1b[5C", "\x1b[5C"},
		{"\x1b[05D", "\x1b[5D"},
		{"\x1b[00E", "\x1b[E"},
		// extra fields on a single-param command aren't a shape we
		// know how to canonicalize, so they pass through unchanged.
		{"\x1b[1;2A", "\x1b[1;2A"},
		// cursor position (CUP) takes row;col, each defaulting to 1.
		{"\x1b[H", "\x1b[H"},
		{"\x1b[1;1H", "\x1b[H"},
		{"\x1b[0;0H", "\x1b[H"},
		{"\x1b[5H", "\x1b[5H"},
		{"\x1b[5;1H", "\x1b[5H"},
		{"\x1b[1;5H", "\x1b[;5H"},
		{"\x1b[05;03H", "\x1b[5;3H"},
		{"\x1b[3;5H", "\x1b[3;5H"},
		{"\x1b[1;1f", "\x1b[f"},
		{"\x1b[5;3;1H", "\x1b[5;3;1H"},
	}
	for _, c := range cases {
		got, err := Format([]byte(c.in), false)
		if err != nil {
			t.Errorf("Format(%q) returned error: %v", c.in, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("Format(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatStrictRejectsTruncatedCSI(t *testing.T) {
	_, err := Format([]byte("\x1b[1;3"), false)
	if err == nil {
		t.Fatal("expected an error for a truncated CSI sequence in strict mode")
	}
}

func TestFormatOSCTerminatorNormalization(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// window title (OSC 0/1/2): BEL and ST are interchangeable
		// terminators, so BEL collapses to the canonical ST form.
		{"\x1b]0;title\x07", "\x1b]0;title\x1b\\"},
		{"\x1b]2;window title\x07", "\x1b]2;window title\x1b\\"},
		{"\x1b]1;icon name\x1b\\", "\x1b]1;icon name\x1b\\"},
		// hyperlinks (OSC 8): open and close both get the same treatment.
		{"\x1b]8;;http://example.com\x07link text\x1b]8;;\x07",
			"\x1b]8;;http://example.com\x1b\\link text\x1b]8;;\x1b\\"},
		{"\x1b]8;id=1;http://example.com\x1b\\link\x1b]8;;\x1b\\",
			"\x1b]8;id=1;http://example.com\x1b\\link\x1b]8;;\x1b\\"},
	}
	for _, c := range cases {
		got, err := Format([]byte(c.in), false)
		if err != nil {
			t.Errorf("Format(%q) returned error: %v", c.in, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("Format(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatStrictRejectsUnterminatedOSC(t *testing.T) {
	_, err := Format([]byte("\x1b]0;title"), false)
	if err == nil {
		t.Fatal("expected an error for an unterminated OSC sequence in strict mode")
	}
	if _, ok := err.(*MalformedError); !ok {
		t.Fatalf("expected *MalformedError, got %T", err)
	}
}

func TestFormatLenientPassesUnterminatedOSCThrough(t *testing.T) {
	in := "\x1b]0;title"
	got, err := Format([]byte(in), true)
	if err != nil {
		t.Fatalf("unexpected error in lenient mode: %v", err)
	}
	if string(got) != in {
		t.Errorf("Format(%q, lenient) = %q, want unchanged input", in, got)
	}
}
