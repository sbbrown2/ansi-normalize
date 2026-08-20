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

func TestFormatStrictRejectsTruncatedCSI(t *testing.T) {
	_, err := Format([]byte("\x1b[1;3"), false)
	if err == nil {
		t.Fatal("expected an error for a truncated CSI sequence in strict mode")
	}
}
