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
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: escfmt [--lenient] [--strip] [file]")
		fmt.Fprintln(os.Stderr, "reads from stdin if no file is given")
	}
	flag.Parse()

	var in io.Reader = os.Stdin
	args := flag.Args()
	if len(args) > 1 {
		flag.Usage()
		os.Exit(2)
	}
	if len(args) == 1 {
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "escfmt:", err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
	}

	data, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "escfmt:", err)
		os.Exit(1)
	}

	var out []byte
	if *strip {
		out, err = escfmt.Strip(data, *lenient)
	} else {
		out, err = escfmt.Format(data, *lenient)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "escfmt: strict mode rejected input:", err)
		fmt.Fprintln(os.Stderr, "escfmt: rerun with --lenient to pass unrecognized sequences through unchanged")
		os.Exit(1)
	}

	os.Stdout.Write(out)
}
