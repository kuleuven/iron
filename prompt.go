package iron

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

type Prompt interface {
	Print(message string) error
	Ask(message string) (string, error)
	Password(message string) (string, error)
}

var StdPrompt Prompt = &prompt{
	r: os.Stdin,
	w: os.Stdout,
}

type prompt struct {
	r, w *os.File
}

func (p *prompt) Print(message string) error {
	_, err := fmt.Fprintf(p.w, "%s\n", message)

	return err
}

func (p *prompt) Ask(message string) (string, error) {
	_, err := fmt.Fprintf(p.w, "%s: ", message)
	if err != nil {
		return "", err
	}

	var value string

	// Read until newline
	for {
		r := make([]byte, 1)

		_, err := p.r.Read(r)
		if err != nil {
			return "", err
		}

		if r[0] == '\n' {
			break
		}

		value += string(r[0])
	}

	return value, err
}

func (p *prompt) Password(message string) (string, error) {
	_, err := fmt.Fprintf(p.w, "%s: ", message)
	if err != nil {
		return "", err
	}

	byteVal, err := term.ReadPassword(int(p.r.Fd()))
	if err != nil {
		return "", err
	}

	_, err = fmt.Fprintln(p.w)

	return string(byteVal), err
}

type Bot map[string]string

func (b Bot) Print(message string) error {
	return nil
}

func (b Bot) Ask(message string) (string, error) {
	value, ok := b[message]
	if !ok {
		return "", fmt.Errorf("no default value for %s", message)
	}

	return value, nil
}

func (b Bot) Password(message string) (string, error) {
	return b.Ask(message)
}
