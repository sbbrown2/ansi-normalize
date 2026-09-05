package escfmt

import "testing"

// FuzzFormat hardens the strict-mode parser against panics and checks two
// invariants that are easy to break by accident when adding a new sequence
// type: lenient mode must accept literally anything, and whatever strict
// mode does accept must already be in canonical form (formatting it again
// should be a no-op).
func FuzzFormat(f *testing.F) {
	seeds := []string{
		"",
		"plain text, no escapes",
		"\x1b",
		"\x1bZ",
		"\x1b[m",
		"\x1b[01;032m",
		"\x1b[38;5;001m",
		"\x1b[1;5H",
		"\x1b[5;3;1H",
		"\x1b[",
		"\x1b[1;3",
		"\x1b]0;title\x07",
		"\x1b]8;;http://example.com\x07link\x1b]8;;\x07",
		"\x1b]0;title",
		"\x1bP1$q\x1b\\",
		"\x1b7\x1b8",
		"\x1b[m\x1b[1;5H\x1b]0;t\x1b\\",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, in string) {
		data := []byte(in)

		// Lenient mode is defined to accept any input, so it must never
		// error and must never panic (the fuzzer catches panics for us).
		if _, err := Format(data, true); err != nil {
			t.Fatalf("Format(%q, lenient) returned error: %v", in, err)
		}
		if _, err := Strip(data, true); err != nil {
			t.Fatalf("Strip(%q, lenient) returned error: %v", in, err)
		}

		strictOut, err := Format(data, false)
		if err != nil {
			// Strict mode is allowed to reject input; that's not a bug.
			return
		}

		again, err := Format(strictOut, false)
		if err != nil {
			t.Fatalf("Format(%q) succeeded but re-formatting its own output failed: %v", in, err)
		}
		if string(again) != string(strictOut) {
			t.Fatalf("Format is not idempotent: Format(%q) = %q, Format(that) = %q", in, strictOut, again)
		}
	})
}
