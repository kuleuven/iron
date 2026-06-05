# Agent instructions

## Git

- Always commit changes to git after completing a task. Do not leave the working tree dirty unless the user explicitly asks for it.
- Add GitHub Copilot as a co-author on commits made with assistant help, using a trailer like:

  ```
  Co-authored-by: GitHub Copilot <copilot@github.com>
  ```
- Use a concise, descriptive commit subject and, when useful, a short body explaining the rationale.

## Linting

- Before committing, run `golangci-lint run` and ensure no new issues are introduced.
- For `goconst` warnings, prefer extracting the repeated literal into a named constant (in tests, a `testconsts_test.go` file is a convenient home). Reuse existing production constants when they already encode the same value.
- Use a narrowly scoped inline `//nolint:<linter>` only when a real fix is impractical or would harm readability, and never blanket-disable linters in production code.
