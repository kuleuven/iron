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
- For `goconst` warnings in test files, add a file-level `//nolint:goconst` directive on the first line (before `package`).
- Do not blanket-disable linters in production code; address real issues, or use a narrowly scoped inline `//nolint:<linter>` only when justified.
