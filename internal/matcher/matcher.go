package matcher

import (
	"fmt"
	"regexp"
	"strings"
)

// Matcher supports a tree-like pattern language with pipe-separated alternates.
//
// Each alternate may be:
//   - re:<expr> for raw regular expressions
//   - a glob-like pattern using *, **, ?, and []
//   - a plain literal, which matches path segments or exact basenames
//
// Examples:
//
//	node_modules|*.json
//	re:^(src|web)/.*\.tsx$
//	server/src/**|README.md
//
// Matching is done against slash-normalized relative paths.
// Plain literals without slashes match any path segment.
// Glob patterns without slashes match the basename.
// Patterns with slashes match the full relative path.
type Matcher struct {
	alts []altMatcher
}

type altMatcher interface {
	Match(path string, isDir bool) bool
	String() string
}

type regexAlt struct {
	raw string
	re  *regexp.Regexp
}

type globAlt struct {
	raw      string
	hasSlash bool
	basename bool
	re       *regexp.Regexp
}

type literalAlt struct {
	raw      string
	hasSlash bool
}

func Compile(patterns []string) (*Matcher, error) {
	var alts []altMatcher
	for _, rawPattern := range patterns {
		rawPattern = strings.TrimSpace(rawPattern)
		var pieces []string
		if strings.HasPrefix(rawPattern, "re:") {
			pieces = []string{rawPattern}
		} else {
			pieces = splitAlternates(rawPattern)
		}
		for _, piece := range pieces {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}

			if strings.HasPrefix(piece, "re:") {
				expr := strings.TrimSpace(strings.TrimPrefix(piece, "re:"))
				re, err := regexp.Compile(expr)
				if err != nil {
					return nil, fmt.Errorf("compile regex %q: %w", piece, err)
				}
				alts = append(alts, regexAlt{raw: piece, re: re})
				continue
			}

			if looksLikeGlob(piece) {
				re, hasSlash, basename, err := compileGlob(piece)
				if err != nil {
					return nil, err
				}
				alts = append(alts, globAlt{raw: piece, hasSlash: hasSlash, basename: basename, re: re})
				continue
			}

			alts = append(alts, literalAlt{raw: normalizePath(piece), hasSlash: strings.Contains(piece, "/") || strings.Contains(piece, "\\")})
		}
	}
	return &Matcher{alts: alts}, nil
}

func (m *Matcher) Empty() bool {
	return m == nil || len(m.alts) == 0
}

func (m *Matcher) Match(path string, isDir bool) bool {
	if m == nil {
		return false
	}
	path = normalizePath(path)
	for _, alt := range m.alts {
		if alt.Match(path, isDir) {
			return true
		}
	}
	return false
}

func (m *Matcher) Patterns() []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.alts))
	for _, alt := range m.alts {
		out = append(out, alt.String())
	}
	return out
}

func (a regexAlt) Match(path string, isDir bool) bool {
	return a.re.MatchString(path)
}

func (a regexAlt) String() string {
	return a.raw
}

func (a globAlt) Match(path string, isDir bool) bool {
	if a.basename {
		return a.re.MatchString(basename(path))
	}
	return a.re.MatchString(path)
}

func (a globAlt) String() string {
	return a.raw
}

func (a literalAlt) Match(path string, isDir bool) bool {
	if a.hasSlash {
		return pathEqualsOrContains(path, a.raw)
	}

	for _, seg := range strings.Split(path, "/") {
		if seg == a.raw {
			return true
		}
	}
	return basename(path) == a.raw
}

func (a literalAlt) String() string {
	return a.raw
}

func splitAlternates(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var parts []string
	var b strings.Builder
	escaped := false
	inClass := false

	for _, r := range raw {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
			b.WriteRune(r)
		case r == '[':
			inClass = true
			b.WriteRune(r)
		case r == ']':
			inClass = false
			b.WriteRune(r)
		case r == '|' && !inClass:
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	parts = append(parts, b.String())
	return parts
}

func looksLikeGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func compileGlob(pattern string) (*regexp.Regexp, bool, bool, error) {
	normalized := normalizePath(pattern)
	hasSlash := strings.Contains(normalized, "/")
	basenameOnly := !hasSlash

	var b strings.Builder
	b.WriteString("^")

	runes := []rune(normalized)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '*':
			if i+1 < len(runes) && runes[i+1] == '*' {
				if i+2 < len(runes) && runes[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
				} else {
					b.WriteString(".*")
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			j := i + 1
			for ; j < len(runes) && runes[j] != ']'; j++ {
			}
			if j >= len(runes) {
				b.WriteString(regexp.QuoteMeta(string(r)))
				continue
			}
			inner := string(runes[i+1 : j])
			if strings.HasPrefix(inner, "!") {
				inner = "^" + inner[1:]
			}
			b.WriteString("[")
			b.WriteString(inner)
			b.WriteString("]")
			i = j
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}

	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, false, false, fmt.Errorf("compile glob %q: %w", pattern, err)
	}
	return re, hasSlash, basenameOnly, nil
}

func normalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	return path
}

func basename(path string) string {
	path = normalizePath(path)
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func pathEqualsOrContains(path, needle string) bool {
	path = normalizePath(path)
	needle = normalizePath(needle)
	if path == needle {
		return true
	}
	if strings.HasPrefix(path, needle+"/") {
		return true
	}
	if strings.Contains(path, "/"+needle+"/") {
		return true
	}
	if strings.HasSuffix(path, "/"+needle) {
		return true
	}
	return false
}
