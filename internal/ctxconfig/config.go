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
	MaxDepth       *int
	Hidden         *bool
	FollowSymlinks *bool
	SameFilesystem *bool
	FullPaths      *bool
	Clipboard      *bool
	PrintPayload   *bool
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
