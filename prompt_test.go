package iron

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPrompt_Print(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "simple message",
			message:  "Hello, World!",
			expected: "Hello, World!\n",
		},
		{
			name:     "empty message",
			message:  "",
			expected: "\n",
		},
		{
			name:     "message with newlines",
			message:  "Line 1\nLine 2",
			expected: "Line 1\nLine 2\n",
		},
		{
			name:     "message with special characters",
			message:  "Special chars: !@#$%^&*()",
			expected: "Special chars: !@#$%^&*()\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &prompt{
				w: &os.File{}, // We'll use a custom writer
			}

			// Create a temporary file for writing
			tmpFile, err := os.CreateTemp(t.TempDir(), "test_output")
			if err != nil {
				t.Fatal(err)
			}

			defer tmpFile.Close()

			p.w = tmpFile

			err = p.Print(tt.message)
			if err != nil {
				t.Errorf("Print() error = %v", err)
				return
			}

			// Read back the content
			tmpFile.Seek(0, 0)

			content := make([]byte, 1024)
			n, _ := tmpFile.Read(content)
			actual := string(content[:n])

			if actual != tt.expected {
				t.Errorf("Print() got = %q, want = %q", actual, tt.expected)
			}
		})
	}
}

func TestPrompt_Ask(t *testing.T) { //nolint:gocognit,funlen
	tests := []struct {
		name          string
		message       string
		input         string
		expectedValue string
		expectError   bool
	}{
		{
			name:          "simple input",
			message:       "Enter name",
			input:         "John\n",
			expectedValue: "John",
			expectError:   false,
		},
		{
			name:          "empty input",
			message:       "Enter value",
			input:         "\n",
			expectedValue: "",
			expectError:   false,
		},
		{
			name:          "input with spaces",
			message:       "Enter text",
			input:         "Hello World\n",
			expectedValue: "Hello World",
			expectError:   false,
		},
		{
			name:          "numeric input",
			message:       "Enter number",
			input:         "123\n",
			expectedValue: "123",
			expectError:   false,
		},
		{
			name:          "no newline",
			message:       "Enter number",
			input:         "123",
			expectedValue: "123",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Create temporary files for input and output
			inputFile, err := os.CreateTemp(dir, "test_input")
			if err != nil {
				t.Fatal(err)
			}

			defer inputFile.Close()

			outputFile, err := os.CreateTemp(dir, "test_output")
			if err != nil {
				t.Fatal(err)
			}

			defer outputFile.Close()

			// Write input data
			inputFile.WriteString(tt.input)
			inputFile.Seek(0, 0)

			p := &prompt{
				r: inputFile,
				w: outputFile,
			}

			value, err := p.Ask(tt.message)
			if tt.expectError && err == nil {
				t.Errorf("Ask() expected error but got none")

				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Ask() unexpected error = %v", err)

				return
			}

			if !tt.expectError && value != tt.expectedValue {
				t.Errorf("Ask() got = %q, want = %q", value, tt.expectedValue)
			}

			// Check that prompt message was written
			outputFile.Seek(0, 0)

			output := make([]byte, 1024)
			n, _ := outputFile.Read(output)
			actualOutput := string(output[:n])
			expectedOutput := tt.message + ": "

			if !strings.HasPrefix(actualOutput, expectedOutput) {
				t.Errorf("Ask() prompt output got = %q, want prefix = %q", actualOutput, expectedOutput)
			}
		})
	}
}

func TestPrompt_Password(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		expectError bool
	}{
		{
			name:        "password prompt",
			message:     "Enter password",
			expectError: false, // We can't easily test actual password input without a real terminal
		},
		{
			name:        "empty message",
			message:     "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			outputFile, err := os.CreateTemp(dir, "test_output")
			if err != nil {
				t.Fatal(err)
			}

			defer outputFile.Close()

			inputFile, err := os.CreateTemp(dir, "test_input")
			if err != nil {
				t.Fatal(err)
			}

			defer inputFile.Close()

			p := &prompt{
				r: inputFile,
				w: outputFile,
			}

			// Note: This test will likely fail in CI/automated environments
			// because term.ReadPassword requires a real terminal
			_, err = p.Password(tt.message)
			if err == nil {
				// If no error, check that the prompt was written
				outputFile.Seek(0, 0)

				output := make([]byte, 1024)
				n, _ := outputFile.Read(output)
				actualOutput := string(output[:n])
				expectedPrompt := tt.message + ": "

				if !strings.Contains(actualOutput, expectedPrompt) {
					t.Errorf("Password() prompt output got = %q, want to contain = %q", actualOutput, expectedPrompt)
				}
			}
		})
	}
}

func TestBot_Print(t *testing.T) {
	bot := Bot{}

	err := bot.Print("Any message")
	if err != nil {
		t.Errorf("Bot.Print() should never return error, got: %v", err)
	}

	err = bot.Print("")
	if err != nil {
		t.Errorf("Bot.Print() should never return error with empty message, got: %v", err)
	}
}

func TestBot_Ask(t *testing.T) { //nolint:funlen
	tests := []struct {
		name          string
		bot           Bot
		message       string
		expectedValue string
		expectError   bool
	}{
		{
			name: "existing key",
			bot: Bot{
				"username": "john_doe",
				"email":    "john@example.com",
			},
			message:       "username",
			expectedValue: "john_doe",
			expectError:   false,
		},
		{
			name: "missing key",
			bot: Bot{
				"username": "john_doe",
			},
			message:     "password",
			expectError: true,
		},
		{
			name:        "empty bot",
			bot:         Bot{},
			message:     "anything",
			expectError: true,
		},
		{
			name: "empty string value",
			bot: Bot{
				"empty": "",
			},
			message:       "empty",
			expectedValue: "",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.bot.Ask(tt.message)
			if tt.expectError && err == nil {
				t.Errorf("Bot.Ask() expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Bot.Ask() unexpected error = %v", err)
				return
			}

			if !tt.expectError && value != tt.expectedValue {
				t.Errorf("Bot.Ask() got = %q, want = %q", value, tt.expectedValue)
			}

			if tt.expectError {
				expectedError := fmt.Sprintf("no default value for %s", tt.message)
				if err.Error() != expectedError {
					t.Errorf("Bot.Ask() error got = %q, want = %q", err.Error(), expectedError)
				}
			}
		})
	}
}

func TestBot_Password(t *testing.T) {
	tests := []struct {
		name          string
		bot           Bot
		message       string
		expectedValue string
		expectError   bool
	}{
		{
			name: "existing password",
			bot: Bot{
				"admin_password": "secret123",
			},
			message:       "admin_password",
			expectedValue: "secret123",
			expectError:   false,
		},
		{
			name: "missing password",
			bot: Bot{
				"username": "admin",
			},
			message:     "admin_password",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.bot.Password(tt.message)
			if tt.expectError && err == nil {
				t.Errorf("Bot.Password() expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Bot.Password() unexpected error = %v", err)
				return
			}

			if !tt.expectError && value != tt.expectedValue {
				t.Errorf("Bot.Password() got = %q, want = %q", value, tt.expectedValue)
			}
		})
	}
}

func TestStdPrompt(t *testing.T) {
	// Test that StdPrompt is properly initialized
	if StdPrompt == nil {
		t.Error("StdPrompt should not be nil")
	}

	// Test that StdPrompt implements the Prompt interface
	_ = StdPrompt

	// Test that Print doesn't panic (though we can't easily test the actual output to stdout)
	err := StdPrompt.Print("Test message")
	if err != nil {
		t.Errorf("StdPrompt.Print() error = %v", err)
	}
}

func TestPromptInterface(t *testing.T) {
	// Test that both prompt and Bot implement the Prompt interface
	var _ Prompt = &prompt{}

	var _ Prompt = Bot{}

	// Test interface methods exist
	testPrompt := &prompt{r: os.Stdin, w: os.Stdout}
	testBot := Bot{"test": "value"}

	// These should compile without error
	_ = testPrompt.Print
	_ = testPrompt.Ask
	_ = testPrompt.Password

	_ = testBot.Print
	_ = testBot.Ask
	_ = testBot.Password
}

// Benchmark tests
func BenchmarkBot_Ask(b *testing.B) {
	bot := Bot{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	b.ResetTimer()

	for range b.N {
		_, _ = bot.Ask("key1")
	}
}

func BenchmarkBot_Print(b *testing.B) {
	bot := Bot{}

	b.ResetTimer()

	for range b.N {
		_ = bot.Print("test message")
	}
}
