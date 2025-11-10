## fzfclone

A tiny, fast, terminal fuzzy finder for files, built with Go and `tcell`. It scans a directory tree, ranks matches as you type, and lets you select a file to print to stdout (fzf-style), so you can pipe it into your editor or other commands.

### Features
- Incremental, non-blocking scan of the filesystem (start searching immediately)
- Ranked fuzzy matching with top-N results
- Arrow-key navigation; Enter to select; ESC to quit
- Respects `.gitignore` (best-effort) and common large directories by default
- Configurable via flags: root, max results, hidden files, and `.gitignore` handling

### Install

Option A: Install from source to your Go bin

```bash
go install github.com/kaleab49/fzfclone@latest
# Ensure $GOPATH/bin (or $GOBIN) is on PATH
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
exec zsh
```

Option B: Build locally

```bash
cd /home/Kaleab_Lionheart/Codes/go-codes/fzfclone
go build -o fzfclone .
# Optionally move into your PATH
install -m 0755 fzfclone ~/.local/bin/
```

### Usage

Basic

```bash
# Search the current directory (default --root ".")
fzfclone
```

Search anywhere

```bash
fzfclone --root "$HOME"
fzfclone --root /etc
fzfclone --root /path/to/project
```

Editor integration

```bash
# Open selected file in VS Code
code "$(fzfclone --root "$HOME")"

# Open selected file in Neovim
nvim "$(fzfclone --root /path/to/project)"
```

Convenience helpers

```bash
# Function: search the provided dir or current dir and print the path
ff() { local f; f="$(fzfclone --root "${1:-$PWD}")" && printf '%s\n' "$f"; }

# Aliases
alias fzfhome='fzfclone --root "$HOME"'
```

### Flags

- `--root string` (default `.`): Directory to scan and search
- `--max-results int` (default `15`): Limit results shown
- `--hidden` (default `false`): Include hidden files and directories
- `--ignore-git` (default `true`): Respect `.gitignore` at the root (best-effort)

Example

```bash
fzfclone --root "$HOME" --max-results 50 --hidden --ignore-git=false
```

### Keybindings

- Up/Down: move selection
- Enter: print the selected path to stdout and exit
- ESC: cancel and exit

### Ignore behavior

fzfclone merges two sources of ignore rules:

1) Built-in defaults (always on):
   - Directories: `.git`, `node_modules`, `dist`, `build`, `bin`, `vendor`
2) `.gitignore` at `--root` (when `--ignore-git=true`):
   - Best-effort parsing; directory entries and simple glob patterns are respected

Hidden files and directories are excluded by default. Pass `--hidden` to include them.

Note: The `.gitignore` support here is intentionally lightweight to avoid adding heavy dependencies. If you need exact parity with Git’s ignore semantics (including negations, anchored rules, etc.), open an issue—this can be added behind a flag.

### Performance tips

- Keep `--ignore-git` enabled and ensure your project’s `.gitignore` is tuned
- Narrow your scope with `--root` (e.g., your project folder instead of `$HOME`)
- Increase `--max-results` cautiously; it affects rendering work, not scanning
- Use `--hidden` only when you really need to search dotfiles

### Roadmap

- Optional right-side preview (first lines or via `bat`)
- Highlight matching characters in results
- Multi-select mode (print multiple paths)
- Full `.gitignore` parity
- File system watcher for live index updates
- CI builds and release artifacts via GoReleaser

### Development

Run locally

```bash
go run . --root .
```

Lint and tidy

```bash
go mod tidy
```

### License

MIT (proposed). If you prefer a different license, update `LICENSE` accordingly.


