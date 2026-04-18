package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openai/ctx-clip/internal/clipboard"
	"github.com/openai/ctx-clip/internal/ctxconfig"
	"github.com/openai/ctx-clip/internal/matcher"
	"github.com/openai/ctx-clip/internal/scan"
)

const version = "0.1.0"

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("ctx-clip", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		includePatterns stringList
		excludePatterns stringList

		allFlag         = fs.Bool("a", false, "include hidden files and directories")
		maxDepthFlag    = fs.Int("L", -1, "maximum directory depth (-1 means unlimited)")
		followLinksFlag = fs.Bool("l", false, "follow symlinked directories")
		sameFSFlag      = fs.Bool("x", false, "stay on the same filesystem")
		fullPathsFlag   = fs.Bool("f", false, "print full paths in headers")

		printFlag       = fs.Bool("print", false, "also print the payload to stdout")
		noClipboardFlag = fs.Bool("no-clipboard", false, "do not copy to clipboard; print payload to stdout")
		quietFlag       = fs.Bool("quiet", false, "suppress payload output (summary still printed)")

		ctxPathFlag   = fs.String("ctx", "", "path to a .ctx config file")
		noCtxFlag     = fs.Bool("no-ctx", false, "ignore any .ctx file")
		expandCtxFlag = fs.Bool("expand-ctx", false, "expand matched files using directory-local .ctx mappings")

		versionFlag = fs.Bool("version", false, "print version and exit")
	)

	fs.Var(&includePatterns, "P", "include only files matching a smart pattern (repeatable, pipe-separated alternates allowed)")
	fs.Var(&excludePatterns, "I", "exclude files or directories matching a smart pattern (repeatable, pipe-separated alternates allowed)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `ctx-clip copies text files from a tree-like directory walk into one clipboard payload.

Usage:
  ctx-clip [options] [directory ...]

Default behavior:
  - copies payload to clipboard
  - prints ONLY a summary line to stderr

Examples:
  ctx-clip
  ctx-clip -L 1 -I 'node_modules|package-lock.json'
  ctx-clip -P 'server/src/**|web/src/**'
  ctx-clip --expand-ctx docs/adr_example.md
  ctx-clip --no-clipboard
  ctx-clip --print

Smart pattern rules:
  - split alternates with |
  - use globs like *.json, **/*.ts, node_modules/**
  - use re:<regex> for raw regular expressions
  - plain literals like node_modules match path segments

.ctx format (auto-loaded from ./.ctx unless --no-ctx is used):
  include server/src/**
  include web/src/**
  exclude node_modules
  context adr_example.md supporting.md ../shared/glossary.md
  exclude *.json
  max-depth 2
  hidden false
`)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *versionFlag {
		fmt.Println(version)
		return 0
	}

	visitedFlags := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visitedFlags[f.Name] = true })

	cfg, cfgSource, err := resolveConfig(*ctxPathFlag, *noCtxFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ctx-clip: %v\n", err)
		return 1
	}

	// Defaults: clipboard ON, payload printing OFF (summary always printed).
	settings := effectiveSettings{
		maxDepth:       -1,
		includeHidden:  false,
		followLinks:    false,
		sameFilesystem: false,
		fullPaths:      false,
		useClipboard:   true,
		printPayload:   false,
	}

	settings.applyConfig(cfg)
	settings.applyCLI(
		visitedFlags,
		*allFlag,
		*maxDepthFlag,
		*followLinksFlag,
		*sameFSFlag,
		*fullPathsFlag,
		*noClipboardFlag,
		*printFlag,
		*quietFlag,
	)

	patterns := patternBundle{
		include: append([]string{}, cfg.Include...),
		exclude: append([]string{}, cfg.Exclude...),
	}
	patterns.include = append(patterns.include, includePatterns...)
	patterns.exclude = append(patterns.exclude, excludePatterns...)
	patterns.exclude = addDefaultExcludes(patterns.exclude, visitedFlags, cfg)

	includeMatcher, err := matcher.Compile(patterns.include)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ctx-clip: invalid include pattern: %v\n", err)
		return 2
	}
	excludeMatcher, err := matcher.Compile(patterns.exclude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ctx-clip: invalid exclude pattern: %v\n", err)
		return 2
	}

	roots := fs.Args()
	files, stats, err := scan.Collect(scan.Options{
		Roots:          roots,
		MaxDepth:       settings.maxDepth,
		IncludeHidden:  settings.includeHidden,
		FollowSymlinks: settings.followLinks,
		SameFilesystem: settings.sameFilesystem,
		FullPaths:      settings.fullPaths,
		Include:        includeMatcher,
		Exclude:        excludeMatcher,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ctx-clip: %v\n", err)
		return 1
	}

	ctxWarnings := 0
	if *expandCtxFlag {
		expandedFiles, mappedStats, warnings, err := ctxconfig.ExpandMappedFiles(files, settings.fullPaths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ctx-clip: %v\n", err)
			return 1
		}
		files = expandedFiles
		stats = mergeStats(stats, mappedStats)
		ctxWarnings = len(warnings)
		for _, warning := range warnings {
			fmt.Fprintln(os.Stderr, warning)
		}
	}

	payload := buildPayload(files)

	// Print payload only when explicitly requested OR when no-clipboard was requested.
	if settings.printPayload && payload != "" {
		fmt.Print(payload)
	}

	clipboardName := ""
	exitCode := 0
	if settings.useClipboard && payload != "" {
		name, err := clipboard.Copy(payload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ctx-clip: %v\n", err)
			exitCode = 1
		} else {
			clipboardName = name
		}
	}

	printSummary(stats, len(files), len(payload), clipboardName, cfgSource, settings.useClipboard, payload == "", ctxWarnings)
	return exitCode
}

type effectiveSettings struct {
	maxDepth       int
	includeHidden  bool
	followLinks    bool
	sameFilesystem bool
	fullPaths      bool
	useClipboard   bool
	printPayload   bool
}

type patternBundle struct {
	include []string
	exclude []string
}

func (s *effectiveSettings) applyConfig(cfg ctxconfig.Config) {
	if cfg.MaxDepth != nil {
		s.maxDepth = *cfg.MaxDepth
	}
	if cfg.Hidden != nil {
		s.includeHidden = *cfg.Hidden
	}
	if cfg.FollowSymlinks != nil {
		s.followLinks = *cfg.FollowSymlinks
	}
	if cfg.SameFilesystem != nil {
		s.sameFilesystem = *cfg.SameFilesystem
	}
	if cfg.FullPaths != nil {
		s.fullPaths = *cfg.FullPaths
	}
	if cfg.Clipboard != nil {
		s.useClipboard = *cfg.Clipboard
	}
	if cfg.PrintPayload != nil {
		s.printPayload = *cfg.PrintPayload
	}
}

func (s *effectiveSettings) applyCLI(
	visited map[string]bool,
	allFlag bool,
	maxDepthFlag int,
	followLinksFlag, sameFSFlag, fullPathsFlag bool,
	noClipboardFlag, printFlag, quietFlag bool,
) {
	if visited["a"] {
		s.includeHidden = allFlag
	}
	if visited["L"] {
		s.maxDepth = maxDepthFlag
	}
	if visited["l"] {
		s.followLinks = followLinksFlag
	}
	if visited["x"] {
		s.sameFilesystem = sameFSFlag
	}
	if visited["f"] {
		s.fullPaths = fullPathsFlag
	}

	// no-clipboard => don't copy; DO print payload (otherwise command does nothing visible)
	if visited["no-clipboard"] && noClipboardFlag {
		s.useClipboard = false
		s.printPayload = true
	}

	// --print => copy AND print
	if visited["print"] && printFlag {
		s.printPayload = true
	}

	// --quiet => suppress payload printing no matter what
	if visited["quiet"] && quietFlag {
		s.printPayload = false
	}
}

func buildPayload(files []scan.File) string {
	var b strings.Builder
	for i, file := range files {
		if i > 0 {
			b.WriteString("\n")
		}
		// file.DisplayPath already carries relative paths like some_dir/file2
		b.WriteString(file.DisplayPath)
		b.WriteString(":\n")
		b.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func printSummary(stats scan.Stats, copiedFiles int, payloadBytes int, clipboardName, cfgSource string, clipboardWanted, empty bool, ctxWarnings int) {
	sourceMsg := ""
	if cfgSource != "" {
		sourceMsg = fmt.Sprintf(" | config: %s", cfgSource)
	}

	clipboardMsg := ""
	switch {
	case !clipboardWanted:
		clipboardMsg = " | clipboard: disabled"
	case empty:
		clipboardMsg = " | clipboard: unchanged (nothing matched)"
	case clipboardName == "":
		clipboardMsg = " | clipboard: failed"
	default:
		clipboardMsg = fmt.Sprintf(" | clipboard: %s", clipboardName)
	}

	fmt.Fprintf(
		os.Stderr,
		"ctx-clip: copied %d file(s), %d byte(s)%s%s | skipped hidden=%d ignored=%d binary=%d empty=%d nonregular=%d errors=%d symlink-dirs=%d ctx-warnings=%d\n",
		copiedFiles,
		payloadBytes,
		sourceMsg,
		clipboardMsg,
		stats.HiddenSkipped,
		stats.IgnoredSkipped,
		stats.BinarySkipped,
		stats.EmptySkipped,
		stats.NonRegularSkipped,
		stats.ErrorSkipped,
		stats.SymlinkDirSkipped,
		ctxWarnings,
	)
}

func resolveConfig(explicit string, disabled bool) (ctxconfig.Config, string, error) {
	if disabled {
		return ctxconfig.Config{}, "", nil
	}

	if explicit != "" {
		cfg, err := ctxconfig.Load(explicit)
		if err != nil {
			return ctxconfig.Config{}, "", err
		}
		return cfg, explicit, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ctxconfig.Config{}, "", err
	}
	path := filepath.Join(cwd, ".ctx")
	if _, err := os.Stat(path); err == nil {
		cfg, err := ctxconfig.Load(path)
		if err != nil {
			return ctxconfig.Config{}, "", err
		}
		return cfg, path, nil
	}
	return ctxconfig.Config{}, "", nil
}

func addDefaultExcludes(excludes []string, visited map[string]bool, cfg ctxconfig.Config) []string {
	// If user explicitly set excludes in ctx or CLI, don't second-guess them.
	userHasExcludes := len(cfg.Exclude) > 0 || visited["I"]
	if userHasExcludes {
		return excludes
	}
	// Default safety/UX excludes
	return append(excludes,
		"ctx-clip",     // common local binary name (segment match)
		".ctx",         // config file (especially relevant when -a is used)
		".ctx.example", // template file
	)
}

func mergeStats(a, b scan.Stats) scan.Stats {
	return scan.Stats{
		Roots:             a.Roots + b.Roots,
		FilesCopied:       a.FilesCopied + b.FilesCopied,
		DirsVisited:       a.DirsVisited + b.DirsVisited,
		HiddenSkipped:     a.HiddenSkipped + b.HiddenSkipped,
		IgnoredSkipped:    a.IgnoredSkipped + b.IgnoredSkipped,
		BinarySkipped:     a.BinarySkipped + b.BinarySkipped,
		EmptySkipped:      a.EmptySkipped + b.EmptySkipped,
		NonRegularSkipped: a.NonRegularSkipped + b.NonRegularSkipped,
		ErrorSkipped:      a.ErrorSkipped + b.ErrorSkipped,
		SymlinkDirSkipped: a.SymlinkDirSkipped + b.SymlinkDirSkipped,
	}
}
