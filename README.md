# ctx-clip

`ctx-clip` walks a directory tree, collects only sensible text files, formats them like:

```text
path/to/file.ts:
<file contents>
```

…and copies the combined payload to your clipboard.

It is intentionally **tree-inspired**, not a clone of `tree`. The parts implemented here are the ones that matter for prompt/context collection:

- recursive walking from the current directory or explicit roots
- `-L` max depth
- `-I` exclude patterns
- `-P` include patterns
- `-a` hidden files and directories
- `-l` follow symlinked directories
- `-x` stay on one filesystem
- `.ctx` config support
- `--expand-ctx` mapped context expansion for matched files
- text-only copying (skip binary / empty / non-regular files)

## Why this version is opinionated

A few decisions were made where your spec was vague or internally inconsistent:

1. **Hidden files are skipped by default.**
   Your final note says that explicitly, so that wins.
2. **Output is one combined clipboard payload**, not one clipboard write per file.
3. **Pattern matching is “smart patterns”**:
   - `*.json` is treated like a glob
   - `node_modules` is treated like a literal path segment match
   - `re:...` gives you raw regex when you really want regex
4. **Only current-directory `.ctx` is auto-loaded for global config** unless `--ctx PATH` is provided.
5. **Mapped context expansion is opt-in** and only runs when `--expand-ctx` is used.

## Build

```bash
go build -o ctx-clip .
```

Or use the repo targets:

```bash
make build
make install
```

## Usage

```bash
ctx-clip [options] [directory ...]
```

### Examples

Copy text files from the current directory only:

```bash
ctx-clip -L 0
```

Copy one level deep, excluding `node_modules` and any `package-lock.json`:

```bash
ctx-clip -L 1 -I 'node_modules|package-lock.json'
```

Only copy source trees:

```bash
ctx-clip -P 'server/src/**|web/src/**'
```

Show what would be copied without touching the clipboard:

```bash
ctx-clip --no-clipboard
```

Expand a target file using directory-local `.ctx` mappings:

```bash
ctx-clip --expand-ctx docs/adr_example.md
```

Include hidden files:

```bash
ctx-clip -a
```

## Pattern syntax

`-I` and `-P` accept repeatable **smart patterns**.

### 1. Pipe-separated alternates

```bash
-I 'node_modules|*.json|dist'
```

### 2. Glob-like patterns

Supported:

- `*` matches within one path segment
- `**` matches across directories
- `?` matches one character
- `[]` character classes work in a basic shell-style way

Examples:

```bash
-P 'src/**/*.ts'
-I '**/*.min.js'
-I 'web/public/**'
```

### 3. Plain literals

A plain literal with no slash matches a path segment:

```bash
-I 'node_modules'
```

A literal with a slash matches that path fragment:

```bash
-I 'web/public'
```

### 4. Raw regex

Prefix with `re:` when you want actual regex behavior:

```bash
-P 're:^(server|web)/src/.*\.(ts|tsx)$'
```

## `.ctx` support

If a `.ctx` file exists in the current directory, `ctx-clip` auto-loads it unless `--no-ctx` is used.

Example:

```text
include server/src/**
include web/src/**
exclude node_modules
context adr_example.md supporting.md ../shared/glossary.md
exclude *.json
max-depth 3
hidden false
follow-symlinks false
same-filesystem false
full-paths false
clipboard true
print true
```

### Supported `.ctx` keys

Repeated keys:

- `include`
- `exclude`
- `ignore` (alias for `exclude`)
- `context`
- `map`
- `related`

Single-value keys:

- `max-depth`
- `hidden`
- `follow-symlinks`
- `same-filesystem`
- `full-paths`
- `clipboard`
- `print`

Accepted separators:

```text
include server/src/**
include = server/src/**
include: server/src/**
```

### Mapped context directives

`--expand-ctx` tells `ctx-clip` to inspect the `.ctx` file in the same directory as each matched file and pull in any related files declared there.

Directive format:

```text
context <target-pattern> <related-file> [more-related-files...]
context <target-pattern> => <related-file> [more-related-files...]
```

Examples:

```text
context adr_example.md supporting.md ../shared/glossary.md
context "adr with spaces.md" => "../shared/decision records.md"
context re:^adr_.*\.md$ templates/adr_template.md
```

Notes:

- target patterns use the same smart pattern rules as `-P` and `-I`
- related paths are resolved relative to the directory that contains the `.ctx`
- mappings are recursive: if a pulled-in file also has a mapping in its own directory, that mapping is expanded too
- missing mapped files do not abort the run; `ctx-clip` prints a warning and copies everything else it can
- mapped files are treated as explicit includes, so they bypass include/exclude filters

## Clipboard backends

The program tries these backends in order until one exists:

- `wl-copy`
- `xclip`
- `xsel`
- `pbcopy`
- `clip.exe`

On Kubuntu / Fedora you will usually want one of:

```bash
sudo dnf install wl-clipboard
sudo dnf install xclip

sudo apt install wl-clipboard
sudo apt install xclip
```

## What counts as “text”?

`ctx-clip` copies:

- UTF-8 text files
- UTF-8 BOM files
- UTF-16 files with BOM
- text-like assets such as JSON, SVG, CSS, JS, TS, Markdown, HTML, etc.

It skips:

- files containing NUL bytes
- obvious binary data
- empty files
- non-regular files

## Notes

- Excludes win over includes.
- `.ctx` is configuration plus optional file-to-file context mappings, not a nested `.gitignore` system.
- The tool sorts the final payload by display path for stable output.

## Output behavior

- Default: copies payload to clipboard and prints only a one-line summary to stderr.
- `--no-clipboard`: prints payload to stdout (and does not touch clipboard).
- `--print`: copies to clipboard and prints payload to stdout.
- `--quiet`: suppresses payload output (summary still printed).

## Development

The repo ships with a small `Makefile` so the common local workflow stays consistent:

```bash
make fmt
make vet
make test
make test-race
make build
make install
make check
```

Defaults:

- `make build` writes `./ctx-clip`
- `make install` installs to `~/.local/bin/ctx-clip`
- override the install location with `make install BINDIR=/some/path`
