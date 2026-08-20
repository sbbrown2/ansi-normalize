// Package escfmt normalizes terminal escape sequences in a byte stream.
//
// Different tools write semantically identical escape sequences in
// different ways: "\x1b[m" and "\x1b[0m" both reset all attributes,
// "\x1b[01m" and "\x1b[1m" both set bold. That noise makes captured
// terminal output annoying to diff or grep. Format rewrites sequences
// into a single canonical form and, by default, refuses to guess at
// anything it does not fully recognize.
package escfmt

import (
	"errors"
	"fmt"
	"strings"
)

// MalformedError reports an escape sequence that Format could not
// normalize while running in strict mode.
type MalformedError struct {
	Offset int // byte offset into the input where the sequence starts
	Reason string
}

func (e *MalformedError) Error() string {
	return fmt.Sprintf("offset %d: %s", e.Offset, e.Reason)
}

// Format returns a normalized copy of input.
//
// In strict mode (lenient == false), any escape sequence Format does not
// recognize, or that is truncated at end of input, produces a
// *MalformedError and no output. In lenient mode those sequences are
// copied through byte for byte instead.
func Format(input []byte, lenient bool) ([]byte, error) {
	out := make([]byte, 0, len(input))
	i := 0
	for i < len(input) {
		if input[i] != 0x1b {
			out = append(out, input[i])
			i++
			continue
		}
		seq, n, err := readEscape(input[i:], lenient)
		if err != nil {
			return nil, &MalformedError{Offset: i, Reason: err.Error()}
		}
		out = append(out, seq...)
		i += n
	}
	return out, nil
}

// singleCharEscapes are the classic VT100 two-byte escapes: ESC followed
// by exactly one more byte, with no parameters and no terminator to hunt
// for. Save/restore cursor, index, next line, and similar.
var singleCharEscapes = map[byte]bool{
	'7': true, '8': true, // save / restore cursor
	'D': true, 'E': true, 'H': true, 'M': true, // IND, NEL, HTS, RI
	'c': true, // full reset
	'=': true, '>': true, // keypad modes
	'N': true, 'O': true, // SS2, SS3
}

func readEscape(buf []byte, lenient bool) ([]byte, int, error) {
	if len(buf) < 2 {
		if lenient {
			return buf, len(buf), nil
		}
		return nil, 0, errors.New("bare escape byte at end of input")
	}

	switch buf[1] {
	case '[':
		return readCSI(buf, lenient)
	case ']', 'P', '_', '^':
		return readStringSeq(buf, lenient)
	default:
		if singleCharEscapes[buf[1]] {
			return buf[:2], 2, nil
		}
		if lenient {
			return buf[:2], 2, nil
		}
		return nil, 0, fmt.Errorf("unrecognized escape 0x%02x", buf[1])
	}
}

func isParamByte(b byte) bool {
	return (b >= '0' && b <= '9') || b == ';' || b == ':'
}

func isCSIFinal(b byte) bool {
	return b >= 0x40 && b <= 0x7e
}

// readCSI parses "ESC [ params final" starting at buf[0] == 0x1b.
func readCSI(buf []byte, lenient bool) ([]byte, int, error) {
	i := 2
	for i < len(buf) && isParamByte(buf[i]) {
		i++
	}
	if i >= len(buf) {
		if lenient {
			return buf, len(buf), nil
		}
		return nil, 0, errors.New("unterminated CSI sequence")
	}

	final := buf[i]
	if !isCSIFinal(final) {
		if lenient {
			return buf[:i+1], i + 1, nil
		}
		return nil, 0, fmt.Errorf("invalid CSI byte 0x%02x", final)
	}

	params := buf[2:i]
	if final == 'm' {
		normalized := normalizeSGR(params)
		result := make([]byte, 0, len(normalized)+3)
		result = append(result, 0x1b, '[')
		result = append(result, normalized...)
		result = append(result, 'm')
		return result, i + 1, nil
	}

	return buf[:i+1], i + 1, nil
}

// normalizeSGR rewrites Select Graphic Rendition parameters into a
// canonical form: an empty parameter list becomes "0" (reset), and each
// field has its leading zeros stripped ("01" -> "1"). Field order is
// preserved because later multi-part codes such as "38;5;N" depend on it.
func normalizeSGR(params []byte) []byte {
	if len(params) == 0 {
		return []byte("0")
	}
	fields := strings.Split(string(params), ";")
	for idx, f := range fields {
		if f == "" {
			fields[idx] = "0"
			continue
		}
		trimmed := strings.TrimLeft(f, "0")
		if trimmed == "" {
			trimmed = "0"
		}
		fields[idx] = trimmed
	}
	return []byte(strings.Join(fields, ";"))
}

// readStringSeq parses OSC/DCS/APC/PM style sequences: ESC followed by a
// one-byte introducer, then arbitrary bytes, terminated by BEL (0x07) or
// the two-byte string terminator ESC \.
func readStringSeq(buf []byte, lenient bool) ([]byte, int, error) {
	i := 2
	for i < len(buf) {
		if buf[i] == 0x07 {
			return buf[:i+1], i + 1, nil
		}
		if buf[i] == 0x1b && i+1 < len(buf) && buf[i+1] == '\\' {
			return buf[:i+2], i + 2, nil
		}
		i++
	}
	if lenient {
		return buf, len(buf), nil
	}
	return nil, 0, errors.New("unterminated OSC/DCS sequence")
}
