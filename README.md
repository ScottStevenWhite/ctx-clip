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
4. **Only current-directory `.ctx` is auto-loaded** unless `--ctx PATH` is provided.

## Build

```bash
go build -o ctx-clip .
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
- `.ctx` is configuration, not a nested `.gitignore` system.
- The tool preserves tree traversal order instead of re-sorting output after collection.

## Output behavior

- Default: copies payload to clipboard and prints only a one-line summary to stderr.
- `--no-clipboard`: prints payload to stdout (and does not touch clipboard).
- `--print`: copies to clipboard and prints payload to stdout.
- `--quiet`: suppresses payload output (summary still printed).
