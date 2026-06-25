package shell

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/elk-language/go-prompt"
	istrings "github.com/elk-language/go-prompt/strings"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

type cobraShell struct {
	root  *cobra.Command
	cache map[string][]prompt.Suggest
	stdin *term.State
}

const shellCommandName = "shell"

// New creates a Cobra CLI command named "shell" which runs an interactive shell prompt for the root command.
func New(root *cobra.Command, opts ...prompt.Option) *cobra.Command {
	shell := &cobraShell{
		root:  root,
		cache: make(map[string][]prompt.Suggest),
	}

	prefix := fmt.Sprintf("> %s ", root.Name())

	opts = append(
		[]prompt.Option{
			prompt.WithCompleter(shell.completer),
			prompt.WithPrefix(prefix),
			prompt.WithCompletionOnDown(),
		},
		opts...,
	)

	return &cobra.Command{
		Use:   shellCommandName,
		Short: "Start an interactive shell.",
		Run: func(cmd *cobra.Command, _ []string) {
			shell.saveStdin()

			shell.editCommandTree(cmd)

			prompt.New(shell.executor, opts...).Run()

			shell.restoreStdin()
		},
	}
}

func (s *cobraShell) editCommandTree(shell *cobra.Command) {
	s.root.RemoveCommand(shell)

	// Hide the "completion" command
	if cmd, _, err := s.root.Find([]string{"completion"}); err == nil {
		cmd.Hidden = true
	}

	s.root.AddCommand(&cobra.Command{
		Use:   "exit",
		Short: "Exit the interactive shell.",
		Run: func(*cobra.Command, []string) {
			os.Exit(0)
		},
	})

	initDefaultHelpFlag(s.root)
}

func initDefaultHelpFlag(cmd *cobra.Command) {
	cmd.InitDefaultHelpFlag()

	for _, subcommand := range cmd.Commands() {
		initDefaultHelpFlag(subcommand)
	}
}

func (s *cobraShell) saveStdin() {
	state, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return
	}

	s.stdin = state
}

func (s *cobraShell) executor(line string) {
	// Allow command to read from stdin
	s.restoreStdin()

	args, err := splitTokens(line)
	if err != nil {
		fmt.Print(err)
	} else if len(args) > 0 {
		_ = execute(s.root, args) //nolint:errcheck
	}

	s.cache = make(map[string][]prompt.Suggest)
}

func (s *cobraShell) restoreStdin() {
	if s.stdin == nil {
		return
	}

	if err := term.Restore(int(os.Stdin.Fd()), s.stdin); err != nil {
		panic(err)
	}
}

func (s *cobraShell) completer(d prompt.Document) ([]prompt.Suggest, istrings.RuneNumber, istrings.RuneNumber) {
	line := d.TextBeforeCursor()
	if line == "" {
		return nil, 0, 0
	}

	// Check if the line starts with a comment (possibly after whitespace)
	if isComment(line) {
		return nil, 0, 0
	}

	args, err := buildCompletionArgs(line)
	if err != nil {
		return nil, 0, 0
	}

	key := strings.Join(args, "\x00")

	suggestions, ok := s.cache[key]
	if !ok {
		out, err := readCommandOutput(s.root, args)
		if err != nil {
			return nil, 0, 0
		}

		suggestions = parseSuggestions(out)
		s.cache[key] = suggestions
	}

	// Determine the token currently being completed in a quote-aware way. Using
	// GetWordBeforeCursor would split on a space inside a quoted path, corrupting
	// both the prefix filter and the range of text to replace.
	rawWord := line[startOfCurrentToken(line):]
	prefix := unquoteToken(rawWord)

	endIndex := d.CurrentRuneIndex()
	startIndex := endIndex - istrings.RuneCount([]byte(rawWord))

	return escapeSuggestions(prompt.FilterHasPrefix(suggestions, prefix, true)), startIndex, endIndex
}

// isComment checks if the input line is a shell comment (starts with #, ignoring leading whitespace).
func isComment(s string) bool {
	// Check if line starts with # (after skipping whitespace)
	trimmed := strings.TrimLeft(s, " \t\n")
	return strings.HasPrefix(trimmed, "#")
}

const completionCommandName = "__complete"

// buildCompletionArgs converts a partial command line into arguments for cobra's
// hidden __complete command. The final argument is always the unquoted token
// currently being completed, which may be empty. Splitting is quote-aware, so a
// token containing spaces inside quotes is treated as a single argument, even
// when the quote has not been closed yet because the user is still typing.
func buildCompletionArgs(input string) ([]string, error) {
	start := startOfCurrentToken(input)

	// The leading part consists of completed tokens only, so it is always
	// balanced and can be split with splitTokens.
	leading, err := splitTokens(input[:start])
	if err != nil {
		return nil, err
	}

	args := append([]string{completionCommandName}, leading...)
	args = append(args, unquoteToken(input[start:]))

	return args, nil
}

func readCommandOutput(cmd *cobra.Command, args []string) (string, error) {
	buf := new(bytes.Buffer)

	stdout := cmd.OutOrStdout()
	stderr := os.Stderr

	cmd.SetOut(buf)

	var err error

	_, os.Stderr, err = os.Pipe()
	if err != nil {
		return "", err
	}

	err = execute(cmd, args)

	cmd.SetOut(stdout)

	os.Stderr = stderr

	return buf.String(), err
}

func assert(err error) {
	if err != nil {
		panic(err)
	}
}

func execute(cmd *cobra.Command, args []string) error {
	ResetArgs(cmd, args)

	cmd.SetArgs(args)

	// Provide a separate context to the command
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	defer stop()

	return cmd.ExecuteContext(ctx)
}

func ResetArgs(cmd *cobra.Command, args []string) {
	cmd, _, err := cmd.Find(args)
	if err != nil {
		return
	}

	// Reset flag values between runs due to a limitation in Cobra
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if val, ok := flag.Value.(pflag.SliceValue); ok {
			assert(val.Replace(explode(flag.DefValue)))

			if o, ok := flag.Value.(*overrideChangedSliceValue); ok {
				o.ResetChanged()
			} else {
				flag.Value = &overrideChangedSliceValue{
					Value:      flag.Value,
					SliceValue: val,
				}
			}
		} else {
			assert(flag.Value.Set(flag.DefValue))
		}

		assert(cmd.Flags().SetAnnotation(flag.Name, cobra.BashCompOneRequiredFlag, []string{"false"}))

		flag.Changed = false
	})

	cmd.InitDefaultHelpFlag()
	cmd.SetContext(nil) //nolint:staticcheck //NOSONAR
}

type overrideChangedSliceValue struct {
	pflag.Value
	pflag.SliceValue
	changed bool
}

func (o *overrideChangedSliceValue) Set(value string) error {
	if !o.changed {
		if err := o.SliceValue.Replace(nil); err != nil {
			return err
		}
	}

	return o.Value.Set(value)
}

func (o *overrideChangedSliceValue) ResetChanged() {
	o.changed = false
}

func explode(args string) []string {
	args, ok := strings.CutPrefix(args, "[")
	if !ok {
		return nil
	}

	args, ok = strings.CutSuffix(args, "]")
	if !ok {
		return nil
	}

	return strings.Split(args, ",")
}

func parseSuggestions(out string) []prompt.Suggest {
	var suggestions []prompt.Suggest

	x := strings.Split(out, "\n")
	if len(x) < 2 {
		return nil
	}

	for _, line := range x[:len(x)-2] {
		x := strings.SplitN(line, "\t", 2)

		if isShorthandFlag(x[0]) {
			continue
		}

		suggestion := prompt.Suggest{Text: x[0]}

		if len(x) > 1 {
			suggestion.Description = x[1]
		}

		suggestions = append(suggestions, suggestion)
	}

	sort.Slice(suggestions, func(i, j int) bool {
		it := suggestions[i].Text
		jt := suggestions[j].Text

		if isFlag(it) && isFlag(jt) {
			return it < jt
		}

		if isFlag(it) {
			return false
		}

		if isFlag(jt) {
			return true
		}

		return it < jt
	})

	return suggestions
}

func escapeSpecialCharacters(val string) string {
	var b strings.Builder

	runes := []rune(val)

	for i, r := range runes {
		switch r {
		case '\\':
			// A backslash only needs escaping when the next character would
			// otherwise be consumed as an escape. A bare backslash (a Windows
			// path separator) is kept as a single character.
			if i+1 < len(runes) && isEscapable(runes[i+1]) {
				b.WriteByte('\\')
			}

			b.WriteRune(r)
		case '"', '$', '`', '!':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}

	escaped := b.String()

	if strings.ContainsAny(val, " #&*;<>?[]|~") {
		// A trailing literal backslash would otherwise escape the closing quote,
		// so double it before wrapping the value in quotes.
		if strings.HasSuffix(escaped, `\`) && !strings.HasSuffix(escaped, `\\`) {
			escaped += `\`
		}

		escaped = fmt.Sprintf(`"%s"`, escaped)
	}

	return escaped
}

// escapeSuggestions escapes each suggestion's text so it can be inserted on the
// command line as a single token, while leaving the descriptions untouched.
func escapeSuggestions(suggestions []prompt.Suggest) []prompt.Suggest {
	escaped := make([]prompt.Suggest, len(suggestions))

	for i, s := range suggestions {
		escaped[i] = prompt.Suggest{
			Text:        escapeSpecialCharacters(s.Text),
			Description: s.Description,
		}
	}

	return escaped
}

// escapableRunes are the only characters a backslash escapes. Keeping the set
// this small means a backslash followed by anything else (for example the "n"
// in C:\new) is preserved literally, so Windows paths survive splitting and
// unescaping unchanged.
func isEscapable(r rune) bool {
	switch r {
	case '\\', '"', '$', '`', '!':
		return true
	default:
		return false
	}
}

// appendEscaped writes the character escaped by the backslash at runes[i] and
// returns the index of the last rune it consumed. Only characters in isEscapable
// are unescaped; for anything else the backslash itself is written verbatim so a
// Windows path such as C:\new is preserved.
func appendEscaped(b *strings.Builder, runes []rune, i int) int {
	if i+1 < len(runes) && isEscapable(runes[i+1]) {
		b.WriteRune(runes[i+1])

		return i + 1
	}

	b.WriteRune(runes[i])

	return i
}

// closeQuoteOrWrite processes rune r while inside a quote of kind quote. It
// returns the new quote state, which is 0 once the matching closing quote is
// seen; any other rune is written to b unchanged.
func closeQuoteOrWrite(b *strings.Builder, r, quote rune) rune {
	if r == quote {
		return 0
	}

	b.WriteRune(r)

	return quote
}

// splitTokens splits a command line into shell-style tokens. It is a minimal
// state machine that understands single quotes, double quotes and backslash
// escapes, but unlike a full shell parser it only unescapes the limited set of
// characters in isEscapable. A backslash before any other character is kept
// verbatim, so a Windows path such as C:\new\table is not mangled into control
// characters the way a general-purpose lexer would. An unterminated quote is
// reported as an error.
func splitTokens(s string) ([]string, error) {
	var (
		tokens  []string
		b       strings.Builder
		started bool
		quote   rune
	)

	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch {
		case quote != 0 && (quote == '\'' || r != '\\'):
			// Inside a quote: write the rune (or close the quote). A backslash
			// is literal inside single quotes but escapes inside double quotes,
			// so double-quote backslashes fall through to the next case.
			started = true
			quote = closeQuoteOrWrite(&b, r, quote)
		case r == '\\':
			started = true
			i = appendEscaped(&b, runes, i)
		case r == '"' || r == '\'':
			started = true
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if started {
				tokens = append(tokens, b.String())
				b.Reset()

				started = false
			}
		default:
			started = true

			b.WriteRune(r)
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote: %q", string(quote))
	}

	if started {
		tokens = append(tokens, b.String())
	}

	return tokens, nil
}

// startOfCurrentToken returns the byte offset in s at which the final shell
// token begins. Whitespace that is inside quotes does not start a new token, so
// a quoted path containing spaces is treated as a single token. A backslash only
// escapes the characters in isEscapable, so a bare backslash (as in a Windows
// path) keeps its literal meaning and does not protect a following space. An
// unterminated quote is tolerated so completion keeps working while the user is
// still typing.
func startOfCurrentToken(s string) int {
	start := 0

	var quote rune

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])

		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			}
		case r == '\\' && quote != '\'':
			// Skip the next rune only when the backslash actually escapes it;
			// otherwise the backslash is literal and the following rune keeps
			// its normal meaning (for example a separating space).
			if next, nsize := utf8.DecodeRuneInString(s[i+size:]); isEscapable(next) {
				i += size + nsize
				continue
			}
		case quote == '"':
			if r == '"' {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			start = i + size
		}

		i += size
	}

	return start
}

// unquoteToken removes one level of shell quoting and escaping from a single
// token. Only the characters in isEscapable are unescaped; a backslash before
// anything else is kept verbatim so Windows paths like C:\new survive intact. It
// tolerates an unterminated quote or a trailing backslash so it can be applied
// to the token the user is currently typing.
func unquoteToken(s string) string {
	var b strings.Builder

	var quote rune

	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch {
		case quote == '\'':
			quote = closeQuoteOrWrite(&b, r, quote)
		case r == '\\':
			i = appendEscaped(&b, runes, i)
		case quote == '"':
			quote = closeQuoteOrWrite(&b, r, quote)
		case r == '"' || r == '\'':
			quote = r
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func isFlag(arg string) bool {
	return strings.HasPrefix(arg, "-")
}

func isShorthandFlag(arg string) bool {
	return isFlag(arg) && !strings.HasPrefix(arg, "--")
}
