package ctxconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Include        []string
	Exclude        []string
	Mappings       []Mapping
	MaxDepth       *int
	Hidden         *bool
	FollowSymlinks *bool
	SameFilesystem *bool
	FullPaths      *bool
	Clipboard      *bool
	PrintPayload   *bool
}

type Mapping struct {
	Target  string
	Related []string
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	cfg := Config{}
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
			continue
		}

		if mapping, ok, err := parseMapping(raw); ok {
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: %w", filepath.Base(path), lineNo, err)
			}
			cfg.Mappings = append(cfg.Mappings, mapping)
			continue
		}

		key, value, ok := splitKV(raw)
		if !ok {
			return Config{}, fmt.Errorf("%s:%d: expected <key> <value>", filepath.Base(path), lineNo)
		}

		value = stripQuotes(value)
		switch normalizeKey(key) {
		case "include", "pattern", "patterns":
			cfg.Include = append(cfg.Include, value)
		case "exclude", "ignore", "ignores":
			cfg.Exclude = append(cfg.Exclude, value)
		case "maxdepth", "depth", "level", "l":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return Config{}, fmt.Errorf("%s:%d: invalid max depth %q", filepath.Base(path), lineNo, value)
			}
			cfg.MaxDepth = &n
		case "hidden", "all":
			b, err := parseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: invalid boolean %q", filepath.Base(path), lineNo, value)
			}
			cfg.Hidden = &b
		case "followsymlinks", "symlinks", "followlinks", "links":
			b, err := parseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: invalid boolean %q", filepath.Base(path), lineNo, value)
			}
			cfg.FollowSymlinks = &b
		case "samefilesystem", "onefilesystem", "xdev", "x":
			b, err := parseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: invalid boolean %q", filepath.Base(path), lineNo, value)
			}
			cfg.SameFilesystem = &b
		case "fullpaths", "fullpath", "abspath", "f":
			b, err := parseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: invalid boolean %q", filepath.Base(path), lineNo, value)
			}
			cfg.FullPaths = &b
		case "clipboard":
			b, err := parseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: invalid boolean %q", filepath.Base(path), lineNo, value)
			}
			cfg.Clipboard = &b
		case "print", "stdout", "output":
			b, err := parseBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: invalid boolean %q", filepath.Base(path), lineNo, value)
			}
			cfg.PrintPayload = &b
		default:
			return Config{}, fmt.Errorf("%s:%d: unknown key %q", filepath.Base(path), lineNo, key)
		}
	}

	if err := scanner.Err(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func splitKV(line string) (key, value string, ok bool) {
	if idx := strings.Index(line, "="); idx >= 0 {
		return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
	}
	if idx := strings.Index(line, ":"); idx >= 0 {
		return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[0], strings.TrimSpace(strings.TrimPrefix(line, parts[0])), true
}

func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, "_", "")
	return key
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on", "y":
		return true, nil
	case "0", "false", "no", "off", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean")
	}
}

func parseMapping(line string) (Mapping, bool, error) {
	tokens, err := splitTokens(line)
	if err != nil {
		return Mapping{}, true, err
	}
	if len(tokens) == 0 {
		return Mapping{}, false, nil
	}

	switch normalizeKey(tokens[0]) {
	case "context", "contexts", "map", "mapping", "related":
	default:
		return Mapping{}, false, nil
	}

	if len(tokens) < 3 {
		return Mapping{}, true, fmt.Errorf("expected <directive> <target> <related...>")
	}

	target := tokens[1]
	related := tokens[2:]
	if related[0] == "=>" || related[0] == "->" {
		related = related[1:]
	}
	if len(related) == 0 {
		return Mapping{}, true, fmt.Errorf("expected at least one related path")
	}

	return Mapping{
		Target:  target,
		Related: related,
	}, true, nil
}

func splitTokens(line string) ([]string, error) {
	var tokens []string
	var b strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if b.Len() == 0 {
			return
		}
		tokens = append(tokens, b.String())
		b.Reset()
	}

	for _, r := range line {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case quote != 0:
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
				continue
			}
			b.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			b.WriteRune(r)
		}
	}

	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quoted string")
	}

	flush()
	return tokens, nil
}
