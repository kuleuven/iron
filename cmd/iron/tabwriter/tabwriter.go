package tabwriter

import (
	"bytes"
	"io"
	"slices"
	"unicode/utf8"
)

type TabWriter struct {
	Writer      io.Writer
	HideColumns []int
	buffer      bytes.Buffer
}

func (tw *TabWriter) Write(p []byte) (int, error) {
	return tw.buffer.Write(p)
}

func (tw *TabWriter) Flush() error {
	// collect cells
	var (
		rows   [][]string
		widths []int
		row    []string
		cell   string
		width  int
	)

	buf := tw.buffer.Bytes()

	for len(buf) > 0 {
		cell, width, buf = findCell(buf)

		// Save widths
		if len(widths) < len(row)+1 {
			widths = append(widths, width)
		} else {
			widths[len(row)] = max(widths[len(row)], width)
		}

		// Save cell in row and row in rows
		if cell == "" || cell[len(cell)-1] != '\t' {
			row = append(row, cell)
			rows = append(rows, row)
			row = nil
		} else {
			row = append(row, cell[:len(cell)-1])
		}
	}

	return tw.outputRows(rows, widths)
}

func (tw *TabWriter) outputRows(rows [][]string, widths []int) error {
	var output bytes.Buffer

	// Do actual output
	for _, row := range rows {
		var rowStarted bool

		for j, cell := range row {
			if slices.Contains(tw.HideColumns, j) {
				output.WriteString(emptyCell(cell))

				continue
			}

			if rowStarted {
				output.WriteString("  ")
			}

			output.WriteString(cell)

			_, width, _ := findCell([]byte(cell))

			if j < len(row)-1 {
				output.Write(bytes.Repeat([]byte(" "), widths[j]-width))
			}

			rowStarted = true
		}

		// Eat all spaces at the end of the line
		if _, err := tw.Writer.Write(eatSpaces(output.Bytes())); err != nil {
			return err
		}

		output.Reset()
	}

	return nil
}

func emptyCell(cell string) string {
	if cell == "" {
		return ""
	}

	abbr := abbreviate(cell, 0)

	if cell[len(cell)-1] == '\n' || cell[len(cell)-1] == '\f' {
		abbr += cell[len(cell)-1:]
	}

	return abbr
}

func findCell(buf []byte) (string, int, []byte) {
	var (
		cell  string
		width int
	)

	for {
		// buffer empty
		if len(buf) == 0 {
			return cell, width, nil
		}

		// tab or newline terminates the cell
		if buf[0] == '\t' || buf[0] == '\n' || buf[0] == '\f' {
			cell += string(buf[0])

			return cell, width, buf[1:]
		}

		// escape character
		if escapeSequence, size := findControlEscapeSequence(buf); size > 0 {
			cell += string(escapeSequence)
			buf = buf[size:]

			continue
		}

		char, size := utf8.DecodeRune(buf)

		cell += string(char)
		width += 1
		buf = buf[size:]
	}
}

func findControlEscapeSequence(buf []byte) ([]byte, int) {
	if len(buf) < 2 || buf[0] != 0x1B || buf[1] != '[' {
		return buf, 0
	}

	for i := 2; i < len(buf); i++ {
		if 0x20 <= buf[i] && buf[i] < 0x40 {
			continue
		}

		if 0x40 <= buf[i] && buf[i] < 0x7F {
			return buf[:i+1], i + 1
		}

		return buf, 0
	}

	return buf, 0
}

func eatSpaces(buf []byte) []byte {
	if len(buf) == 0 {
		return buf
	}

	last := buf[len(buf)-1]

	if last != '\n' && last != '\f' {
		return buf
	}

	for len(buf) > 1 && (buf[len(buf)-2] == ' ' || buf[len(buf)-2] == '\t') {
		buf[len(buf)-2] = last
		buf = buf[:len(buf)-1]
	}

	return buf
}
