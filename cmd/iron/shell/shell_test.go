package shell

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/elk-language/go-prompt"
	"github.com/spf13/cobra"
)

func TestNew(t *testing.T) {
	root := &cobra.Command{
		Use: "testcli",
	}

	shellCmd := New(root)

	if shellCmd.Use != "shell" {
		t.Errorf("Expected shell command Use to be 'shell', got '%s'", shellCmd.Use)
	}

	if shellCmd.Short != "Start an interactive shell." {
		t.Errorf("Expected shell command Short to be 'Start an interactive shell.', got '%s'", shellCmd.Short)
	}

	if shellCmd.Run == nil {
		t.Error("Expected shell command to have a Run function")
	}
}

func TestEditCommandTree(t *testing.T) {
	root := &cobra.Command{
		Use: "testcli",
	}

	// Add a completion command to test hiding
	completionCmd := &cobra.Command{
		Use: "completion",
	}
	root.AddCommand(completionCmd)

	shell := &cobraShell{
		root:  root,
		cache: make(map[string][]prompt.Suggest),
	}

	shellCmd := &cobra.Command{
		Use: "shell",
	}
	root.AddCommand(shellCmd)

	shell.editCommandTree(shellCmd)

	// Check that shell command was removed
	found := false

	for _, cmd := range root.Commands() {
		if cmd.Use == "shell" {
			found = true
			break
		}
	}

	if found {
		t.Error("Expected shell command to be removed from root")
	}

	// Check that completion command is hidden
	if !completionCmd.Hidden {
		t.Error("Expected completion command to be hidden")
	}

	// Check that exit command was added
	exitFound := false

	for _, cmd := range root.Commands() {
		if cmd.Use == "exit" {
			exitFound = true
			break
		}
	}

	if !exitFound {
		t.Error("Expected exit command to be added")
	}
}

func TestBuildCompletionArgs(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "command arg1",
			expected: []string{"__complete", "command", "arg1"},
		},
		{
			input:    "command arg1 ",
			expected: []string{"__complete", "command", "arg1", ""},
		},
		{
			input:    "",
			expected: []string{"__complete", ""},
		},
		{
			input:    "single",
			expected: []string{"__complete", "single"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := buildCompletionArgs(tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuildCompletionArgsWithQuotes(t *testing.T) {
	input := `command "arg with spaces"`

	result, err := buildCompletionArgs(input)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{"__complete", "command", "arg with spaces"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseSuggestions(t *testing.T) {
	output := "suggestion1\tdescription1\nsuggestion2\tdescription2\nCompletion ended with directive: ShellCompDirectiveNoFileComp\n"

	suggestions := parseSuggestions(output)

	expected := []prompt.Suggest{
		{Text: "suggestion1", Description: "description1"},
		{Text: "suggestion2", Description: "description2"},
	}

	if !reflect.DeepEqual(suggestions, expected) {
		t.Errorf("Expected %v, got %v", expected, suggestions)
	}
}

func TestParseSuggestionsWithFlags(t *testing.T) {
	output := "--flag1\tFlag description\ncommand1\tCommand description\n-f\tShort flag\nCompletion ended\n"

	suggestions := parseSuggestions(output)

	// Should filter out shorthand flags and sort flags after commands
	expected := []prompt.Suggest{
		{Text: "command1", Description: "Command description"},
		{Text: "--flag1", Description: "Flag description"},
	}

	if !reflect.DeepEqual(suggestions, expected) {
		t.Errorf("Expected %v, got %v", expected, suggestions)
	}
}

func TestParseSuggestionsEmpty(t *testing.T) {
	output := ""
	suggestions := parseSuggestions(output)

	if suggestions != nil {
		t.Errorf("Expected nil suggestions for empty output, got %v", suggestions)
	}
}

func TestParseSuggestionsInsufficientLines(t *testing.T) {
	output := "single line"
	suggestions := parseSuggestions(output)

	if suggestions != nil {
		t.Errorf("Expected nil suggestions for single line output, got %v", suggestions)
	}
}

func TestEscapeSpecialCharacters(t *testing.T) { //nolint:funlen
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "simple",
			expected: "simple",
		},
		{
			input:    "with space",
			expected: `"with space"`,
		},
		{
			input:    "with$dollar",
			expected: `with\$dollar`,
		},
		{
			input:    `with\backslash`,
			expected: `with\\backslash`,
		},
		{
			input:    `with"quote`,
			expected: `with\"quote`,
		},
		{
			input:    "with`backtick",
			expected: "with\\`backtick",
		},
		{
			input:    "with!exclamation",
			expected: `with\!exclamation`,
		},
		{
			input:    "with&ampersand",
			expected: `"with&ampersand"`,
		},
		{
			input:    "with*asterisk",
			expected: `"with*asterisk"`,
		},
		{
			input:    "with;semicolon",
			expected: `"with;semicolon"`,
		},
		{
			input:    "with<less",
			expected: `"with<less"`,
		},
		{
			input:    "with>greater",
			expected: `"with>greater"`,
		},
		{
			input:    "with?question",
			expected: `"with?question"`,
		},
		{
			input:    "with[bracket",
			expected: `"with[bracket"`,
		},
		{
			input:    "with]bracket",
			expected: `"with]bracket"`,
		},
		{
			input:    "with|pipe",
			expected: `"with|pipe"`,
		},
		{
			input:    "with~tilde",
			expected: `"with~tilde"`,
		},
		{
			input:    "with#hash",
			expected: `"with#hash"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeSpecialCharacters(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIsFlag(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"--flag", true},
		{"-f", true},
		{"command", false},
		{"", false},
		{"-", true},
		{"--", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isFlag(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %t for '%s', got %t", tt.expected, tt.input, result)
			}
		})
	}
}

func TestIsShorthandFlag(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"-f", true},
		{"--flag", false},
		{"command", false},
		{"", false},
		{"-", true},
		{"--", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isShorthandFlag(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %t for '%s', got %t", tt.expected, tt.input, result)
			}
		})
	}
}

func TestExecute(t *testing.T) {
	// Create a test command
	var executed bool

	var receivedArgs []string

	testCmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {
			executed = true
			receivedArgs = args
		},
	}

	root := &cobra.Command{Use: "root"}
	root.AddCommand(testCmd)

	args := []string{"test", "arg1", "arg2"}

	err := execute(root, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !executed {
		t.Error("Expected command to be executed")
	}

	expectedArgs := []string{"arg1", "arg2"}
	if !reflect.DeepEqual(receivedArgs, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, receivedArgs)
	}
}

func TestExecuteWithFlags(t *testing.T) {
	var flagValue string

	var executed bool

	testCmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {
			executed = true
		},
	}
	testCmd.Flags().StringVar(&flagValue, "flag", "default", "test flag")

	root := &cobra.Command{Use: "root"}
	root.AddCommand(testCmd)

	// First execution with flag value
	args := []string{"test", "--flag", "value1"}

	err := execute(root, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !executed {
		t.Error("Expected command to be executed")
	}

	if flagValue != "value1" {
		t.Errorf("Expected flag value 'value1', got '%s'", flagValue)
	}

	// Second execution should reset flag to default
	executed = false
	args = []string{"test"}

	err = execute(root, args)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !executed {
		t.Error("Expected command to be executed")
	}

	if flagValue != "default" {
		t.Errorf("Expected flag value to be reset to 'default', got '%s'", flagValue)
	}
}

func TestReadCommandOutput(t *testing.T) {
	testCmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Print("test output")
		},
	}

	root := &cobra.Command{Use: "root"}
	root.AddCommand(testCmd)

	output, err := readCommandOutput(root, []string{"test"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if output != "test output" {
		t.Errorf("Expected 'test output', got '%s'", output)
	}
}

func TestCobraShellCompleter(t *testing.T) {
	// Create a simple command structure for testing
	root := &cobra.Command{Use: "root"}

	subCmd := &cobra.Command{
		Use: "subcmd",
		Run: func(cmd *cobra.Command, args []string) {},
	}
	subCmd.Flags().String("flag", "", "test flag")
	root.AddCommand(subCmd)

	shell := &cobraShell{
		root:  root,
		cache: make(map[string][]prompt.Suggest),
	}

	// Test empty line
	doc := prompt.Document{Text: ""}

	suggestions, _, _ := shell.completer(doc)
	if suggestions != nil {
		t.Errorf("Expected nil suggestions for empty line, got %v", suggestions)
	}

	// Note: Testing actual completion requires the cobra completion system to be working,
	// which is complex to set up in unit tests. This test verifies the basic structure.
}

func TestCobraShellExecutor(t *testing.T) {
	var executed bool

	var receivedArgs []string

	testCmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {
			executed = true
			receivedArgs = args
		},
	}

	root := &cobra.Command{Use: "root"}
	root.AddCommand(testCmd)

	shell := &cobraShell{
		root:  root,
		cache: make(map[string][]prompt.Suggest),
	}

	shell.executor("test arg1 arg2")

	if !executed {
		t.Error("Expected command to be executed")
	}

	expectedArgs := []string{"arg1", "arg2"}
	if !reflect.DeepEqual(receivedArgs, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, receivedArgs)
	}
}

func TestCobraShellExecutorInvalidCommand(t *testing.T) {
	// Capture output to avoid printing to stdout during tests
	var buf bytes.Buffer

	root := &cobra.Command{Use: "root"}
	root.SetOut(&buf)
	root.SetErr(&buf)

	shell := &cobraShell{
		root:  root,
		cache: make(map[string][]prompt.Suggest),
	}

	// This should not panic, even with invalid input
	shell.executor("invalid command")
	// The command should handle the error gracefully
	// (exact behavior depends on cobra's error handling)
}

func TestInitDefaultHelpFlag(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	subCmd := &cobra.Command{Use: "sub"}
	root.AddCommand(subCmd)

	initDefaultHelpFlag(root)

	// Check that help flag exists on root
	helpFlag := root.Flags().Lookup("help")
	if helpFlag == nil {
		t.Error("Expected help flag to be initialized on root command")
	}

	// Check that help flag exists on subcommand
	helpFlag = subCmd.Flags().Lookup("help")
	if helpFlag == nil {
		t.Error("Expected help flag to be initialized on subcommand")
	}
}
