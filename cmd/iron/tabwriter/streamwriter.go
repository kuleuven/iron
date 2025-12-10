package tabwriter

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type StreamWriter struct {
	Writer io.Writer
	Cols   []int
	buffer bytes.Buffer
}

func (w *StreamWriter) Write(p []byte) (int, error) {
	n, err := w.buffer.Write(p)
	if err != nil {
		return n, err
	}

	for {
		line, err := w.buffer.ReadString('\n')
		if err != nil {
			w.buffer.WriteString(line)

			return n, nil //nolint:nilerr
		}

		line = line[:len(line)-1]

		if err := w.writeLine(line); err != nil {
			return n, err
		}
	}
}

func (w *StreamWriter) Flush() error {
	line, err := w.buffer.ReadString('\n')
	if err != io.EOF || line == "" {
		return err
	}

	return w.writeLine(line)
}

func (w *StreamWriter) writeLine(line string) error {
	buf := []byte(line)

	var (
		cell  string
		width int
		i     int
		out   bytes.Buffer
	)

	for len(buf) > 0 {
		cell, width, buf = findCell(buf)

		if cell[len(cell)-1] == '\t' {
			cell = cell[:len(cell)-1]
		}

		if i > 0 {
			fmt.Fprint(&out, "  ")
		}

		padding := w.Cols[i] - width

		if padding >= 0 {
			fmt.Fprintf(&out, "%s%s", cell, strings.Repeat(" ", padding))
		} else {
			fmt.Fprint(&out, abbreviate(cell, w.Cols[i]-1)+"…")
		}

		i++

		if i == len(w.Cols) {
			fmt.Fprintf(&out, "  %s", buf)

			buf = nil
		}
	}

	fmt.Fprint(&out, "\n")

	_, err := w.Writer.Write(out.Bytes())

	return err
}

func abbreviate(str string, width int) string {
	buf := []byte(str)

	var out bytes.Buffer

	for {
		// buffer empty
		if len(buf) == 0 {
			return out.String()
		}

		// escape character
		if escapeSequence, size := findControlEscapeSequence(buf); size > 0 {
			out.Write(escapeSequence)

			buf = buf[size:]

			continue
		}

		char, size := utf8.DecodeRune(buf)

		if width > 0 {
			out.WriteRune(char)

			width -= 1
		}

		buf = buf[size:]
	}
}
