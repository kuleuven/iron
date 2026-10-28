package shell

import (
	"reflect"
	"testing"
)

func TestScriptTokens(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{`upload C:\new\table /target`, []string{"upload", `C:\new\table`, "/target"}},
		{`mkdir 'a;b|c&d'`, []string{"mkdir", "a;b|c&d"}},
		{`mv "with space" final`, []string{"mv", "with space", "final"}},
	} {
		t.Run(tc.line, func(t *testing.T) {
			got, err := SplitScriptTokens(tc.line)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SplitScriptTokens = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestScriptRejectsShellOperators(t *testing.T) {
	for _, line := range []string{"mkdir one;mkdir two", "mkdir one && mkdir two", "download one | cat", "upload <input output", "mkdir `pwd`"} {
		if _, err := SplitScriptTokens(line); err == nil {
			t.Errorf("accepted shell operator in %q", line)
		}
	}
}
