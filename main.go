// Command escfmt reads text containing terminal escape sequences and writes
// a normalized version of it.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sbbrown2/ansi-normalize/internal/escfmt"
)

func main() {
	lenient := flag.Bool("lenient", false, "pass unrecognized or malformed escape sequences through unchanged instead of failing")
	strip := flag.Bool("strip", false, "remove all escape sequences instead of normalizing them")
	write := flag.Bool("w", false, "write the result back to each input file instead of stdout (requires at least one file argument)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: escfmt [--lenient] [--strip] [-w] [file...]")
		fmt.Fprintln(os.Stderr, "reads from stdin if no file is given")
	}
	flag.Parse()

	args := flag.Args()
	if *write && len(args) == 0 {
		fmt.Fprintln(os.Stderr, "escfmt: -w requires at least one file argument")
		os.Exit(2)
	}

	if len(args) == 0 {
		if err := process(os.Stdin, "", *lenient, *strip, false); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	failed := false
	for _, name := range args {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "escfmt:", err)
			failed = true
			continue
		}
		err = process(f, name, *lenient, *strip, *write)
		f.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

// process reads all of in, formats or strips it, and writes the result
// either to stdout or, if write is true, back to the file at name.
func process(in *os.File, name string, lenient, strip, write bool) error {
	data, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("escfmt: %w", err)
	}

	var out []byte
	if strip {
		out, err = escfmt.Strip(data, lenient)
	} else {
		out, err = escfmt.Format(data, lenient)
	}
	if err != nil {
		prefix := "escfmt"
		if name != "" {
			prefix = "escfmt: " + name
		}
		return fmt.Errorf("%s: strict mode rejected input: %w\nescfmt: rerun with --lenient to pass unrecognized sequences through unchanged", prefix, err)
	}

	if !write {
		os.Stdout.Write(out)
		return nil
	}

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("escfmt: %s: %w", name, err)
	}
	if err := os.WriteFile(name, out, info.Mode().Perm()); err != nil {
		return fmt.Errorf("escfmt: %s: %w", name, err)
	}
	return nil
}
