package main

import (
	"io"
	"os"
)

// isTerminalWriter reports whether w is a *os.File backed by a character
// device (a TTY), without any third-party dependency. Non-file writers (e.g.
// test buffers, pipes) are treated as non-terminals so color is disabled.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
