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
	case ']':
		return readOSC(buf, lenient)
	case 'P', '_', '^':
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
	switch final {
	case 'm':
		return csiResult(normalizeSGR(params), final), i + 1, nil
	case 'A', 'B', 'C', 'D', 'E', 'F', 'G':
		if normalized, ok := normalizeSingleMoveParam(params); ok {
			return csiResult(normalized, final), i + 1, nil
		}
	case 'H', 'f':
		return csiResult(normalizeCursorPosition(params), final), i + 1, nil
	}

	return buf[:i+1], i + 1, nil
}

func csiResult(params []byte, final byte) []byte {
	result := make([]byte, 0, len(params)+3)
	result = append(result, 0x1b, '[')
	result = append(result, params...)
	result = append(result, final)
	return result
}

// isMoveDefault reports whether a single cursor-movement parameter field
// carries the default value. Per ECMA-48, an omitted parameter and an
// explicit 0 both mean "1" for these commands, so all three are
// interchangeable and collapse to the same canonical (omitted) form.
func isMoveDefault(field string) bool {
	if field == "" {
		return true
	}
	stripped := strings.TrimLeft(field, "0")
	return stripped == "" || stripped == "1"
}

// stripLeadingZeros normalizes a numeric parameter field, e.g. "05" -> "5".
// An all-zero field normalizes to "0".
func stripLeadingZeros(field string) string {
	stripped := strings.TrimLeft(field, "0")
	if stripped == "" {
		return "0"
	}
	return stripped
}

// normalizeSingleMoveParam handles the single-parameter cursor movement
// commands (CUU, CUD, CUF, CUB, CNL, CPL, CHA), all of which default to a
// count of 1. It reports ok == false when the parameter list has more than
// one field, since that isn't a shape this function knows how to canonicalize.
func normalizeSingleMoveParam(params []byte) ([]byte, bool) {
	raw := string(params)
	if strings.Contains(raw, ";") {
		return nil, false
	}
	if isMoveDefault(raw) {
		return nil, true
	}
	return []byte(stripLeadingZeros(raw)), true
}

// normalizeCursorPosition handles CUP (final byte 'H' or 'f'), which takes
// an optional row and column, each defaulting to 1. A field left at its
// default is omitted from the output rather than written out explicitly.
func normalizeCursorPosition(params []byte) []byte {
	raw := string(params)
	if raw == "" {
		return nil
	}
	fields := strings.Split(raw, ";")
	if len(fields) > 2 {
		return params
	}

	row := fields[0]
	rowDefault := isMoveDefault(row)
	if len(fields) == 1 {
		if rowDefault {
			return nil
		}
		return []byte(stripLeadingZeros(row))
	}

	col := fields[1]
	colDefault := isMoveDefault(col)
	switch {
	case rowDefault && colDefault:
		return nil
	case rowDefault:
		return []byte(";" + stripLeadingZeros(col))
	case colDefault:
		return []byte(stripLeadingZeros(row))
	default:
		return []byte(stripLeadingZeros(row) + ";" + stripLeadingZeros(col))
	}
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
		fields[idx] = stripLeadingZeros(f)
	}
	return []byte(strings.Join(fields, ";"))
}

// readOSC parses "ESC ] payload terminator", where terminator is either
// BEL (0x07) or the two-byte string terminator ESC \. Both terminators are
// accepted by every terminal that implements OSC, so which one a program
// happened to write is exactly the kind of wire noise Format exists to
// remove: the output always uses ESC \.
func readOSC(buf []byte, lenient bool) ([]byte, int, error) {
	i := 2
	for i < len(buf) {
		if buf[i] == 0x07 {
			return oscResult(buf[2:i]), i + 1, nil
		}
		if buf[i] == 0x1b && i+1 < len(buf) && buf[i+1] == '\\' {
			return oscResult(buf[2:i]), i + 2, nil
		}
		i++
	}
	if lenient {
		return buf, len(buf), nil
	}
	return nil, 0, errors.New("unterminated OSC sequence")
}

func oscResult(payload []byte) []byte {
	result := make([]byte, 0, len(payload)+4)
	result = append(result, 0x1b, ']')
	result = append(result, payload...)
	result = append(result, 0x1b, '\\')
	return result
}

// readStringSeq parses DCS/APC/PM style sequences: ESC followed by a
// one-byte introducer, then arbitrary bytes, terminated by BEL (0x07) or
// the two-byte string terminator ESC \. Unlike OSC these are left as
// written; BEL as a terminator for these is rare enough in practice that
// rewriting it isn't worth the risk of touching payload bytes we don't
// otherwise parse.
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
