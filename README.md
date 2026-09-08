# ansi-normalize

`escfmt` normalizes terminal escape sequences in text so that
semantically identical output stops looking different on the wire.

## the problem

Capture terminal output from two different programs, or the same
program at two different times, and diff it. You'll get noise that has
nothing to do with the actual content:

- `\x1b[m` and `\x1b[0m` both reset all attributes, but they're
  different bytes.
- `\x1b[01;32m` and `\x1b[1;32m` both mean bold green, but one has a
  padded zero.
- Some tools drop the implicit `0` in a parameter list, others don't.

None of that matters to a terminal. All of it matters if you're diffing
logs, writing a snapshot test against colored CLI output, or grepping
through a captured session.

## what it does right now

`escfmt` reads a byte stream, walks it looking for `ESC` bytes, and
rewrites two families of sequences into canonical form:

- Select Graphic Rendition (SGR, the color/style) sequences: no
  leading zeros, no implicit empty reset.
- Cursor movement (`CUU`/`CUD`/`CUF`/`CUB`/`CNL`/`CPL`/`CHA`/`CUP`):
  a parameter equal to the command's default is dropped rather than
  spelled out, so `\x1b[1A`, `\x1b[0A`, and `\x1b[A` all collapse to
  `\x1b[A`, and `\x1b[1;1H` collapses to `\x1b[H`.
- OSC sequences (window titles, hyperlinks, and the like): some
  programs terminate these with BEL (`\x07`), others with the two-byte
  string terminator `ESC \`. Both mean the same thing, so the output
  always uses `ESC \`.

Other recognized string-type escape sequences (DCS/APC/PM) are
validated and passed through as-is.

## examples

Escape bytes below are written as `ESC` since a raw `0x1b` doesn't
render in a table. Each row is exercised by a test in
`internal/escfmt/format_test.go`.

| before | after |
| --- | --- |
| `ESC[m` | `ESC[0m` |
| `ESC[01;032m` | `ESC[1;32m` |
| `ESC[38;5;001m` | `ESC[38;5;1m` |
| `ESC[1;1H` | `ESC[H` |
| `ESC[05;03H` | `ESC[5;3H` |
| `ESC[1;5H` | `ESC[;5H` |
| `ESC]0;titleBEL` | `ESC]0;titleESC\` |
| `ESC]8;;http://example.comBELlink textESC]8;;BEL` | `ESC]8;;http://example.comESC\link textESC]8;;ESC\` |

The important part is what happens when it finds something it doesn't
recognize.

## strict by default

By default, any escape sequence `escfmt` can't fully parse - an
unknown introducer, a truncated CSI sequence cut off mid-stream, a
string sequence with no terminator - is a hard error. The tool refuses
to guess, because silently passing through something it doesn't
understand is how you end up with "normalized" output that still isn't
normalized.

```
$ printf 'hello \x1bZ world' | escfmt
escfmt: strict mode rejected input: offset 6: unrecognized escape 0x5a
escfmt: rerun with --lenient to pass unrecognized sequences through unchanged
```

If you're dealing with a capture from something unusual and you just
want the known sequences cleaned up while everything else is left
alone, use `--lenient`:

```
$ printf 'hello \x1bZ world' | escfmt --lenient
hello ESCZ world   # (shown here as ESCZ; the real output has a raw 0x1b byte)
```

## usage

```
escfmt [--lenient] [--strip] [-w] [file...]
```

Reads from stdin if no file is given, writes the normalized result to
stdout.

```
$ cat captured.log | escfmt > normalized.log
$ escfmt --lenient weird_capture.ans > clean.ans
```

Pass more than one file and each is read and normalized in turn, with
output written to stdout in order - handy for checking several captures
in one pass. Pass `-w` to write the normalized result back to each file
instead, in place, rather than printing it:

```
$ escfmt -w session-*.log
```

`-w` requires at least one file argument; there's no such thing as
writing stdin back in place.

If any one file fails to parse in strict mode, `escfmt` reports it and
keeps going on the rest, then exits nonzero once all files have been
processed.

If you don't want normalized escape sequences, you want none at all,
pass `--strip` to remove every escape sequence and keep only the plain
text:

```
$ printf 'hello \x1b[1;32mworld\x1b[0m' | escfmt --strip
hello world
```

`--strip` still parses strictly by default - an unrecognized or
truncated sequence is an error unless you also pass `--lenient`, in
which case it's dropped along with everything else `--strip` removes.

## building

Standard library only, no dependencies:

```
go build ./...
```

## status

Early. SGR, cursor-movement, and OSC terminator normalization work;
DCS/APC/PM string sequences are recognized and validated but passed
through unchanged.
