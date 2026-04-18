package scan

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/openai/ctx-clip/internal/matcher"
	"github.com/openai/ctx-clip/internal/textfile"
)

type Options struct {
	Roots          []string
	MaxDepth       int
	IncludeHidden  bool
	FollowSymlinks bool
	SameFilesystem bool
	FullPaths      bool
	Include        *matcher.Matcher
	Exclude        *matcher.Matcher
}

type File struct {
	DisplayPath string
	SourcePath  string
	Content     string
}

type Stats struct {
	Roots             int
	FilesCopied       int
	DirsVisited       int
	HiddenSkipped     int
	IgnoredSkipped    int
	BinarySkipped     int
	EmptySkipped      int
	NonRegularSkipped int
	ErrorSkipped      int
	SymlinkDirSkipped int
}

type Collector struct {
	opts    Options
	cwd     string
	visited map[string]bool
	files   []File
	stats   Stats
}

func Collect(opts Options) ([]File, Stats, error) {
	if len(opts.Roots) == 0 {
		opts.Roots = []string{"."}
	}
	if opts.MaxDepth < 0 {
		opts.MaxDepth = -1
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, Stats{}, err
	}

	c := &Collector{
		opts:    opts,
		cwd:     cwd,
		visited: map[string]bool{},
	}
	c.stats.Roots = len(opts.Roots)

	for _, root := range opts.Roots {
		if err := c.collectRoot(root); err != nil {
			return c.files, c.stats, err
		}
	}

	sort.SliceStable(c.files, func(i, j int) bool {
		return c.files[i].DisplayPath < c.files[j].DisplayPath
	})
	return c.files, c.stats, nil
}

func (c *Collector) collectRoot(root string) error {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("stat %s: %w", root, err)
	}

	displayRoot, err := c.rootDisplay(root)
	if err != nil {
		return err
	}

	var rootDev uint64
	if c.opts.SameFilesystem {
		rootDev, err = deviceID(root)
		if err != nil {
			return err
		}
	}

	if info.Mode()&os.ModeSymlink != 0 {
		stat, err := os.Stat(root)
		if err != nil {
			c.stats.ErrorSkipped++
			return nil
		}
		if stat.IsDir() {
			if !c.opts.FollowSymlinks {
				c.stats.SymlinkDirSkipped++
				return nil
			}
			real, err := filepath.EvalSymlinks(root)
			if err != nil {
				c.stats.ErrorSkipped++
				return nil
			}
			if c.seen(real) {
				return nil
			}
			return c.walkDir(real, displayRoot, 0, rootDev)
		}
		return c.processFile(root, displayRoot, rootDev, true)
	}

	if info.IsDir() {
		return c.walkDir(root, displayRoot, 0, rootDev)
	}
	return c.processFile(root, displayRoot, rootDev, true)
}

func (c *Collector) walkDir(fsPath, displayPrefix string, depth int, rootDev uint64) error {
	c.stats.DirsVisited++

	entries, err := os.ReadDir(fsPath)
	if err != nil {
		c.stats.ErrorSkipped++
		return nil
	}

	for _, entry := range entries {
		name := entry.Name()
		childFsPath := filepath.Join(fsPath, name)
		childDisplay := joinDisplay(displayPrefix, name)

		if !c.opts.IncludeHidden && isHiddenName(name) {
			c.stats.HiddenSkipped++
			continue
		}

		if c.opts.Exclude != nil && c.opts.Exclude.Match(childDisplay, entry.IsDir()) {
			c.stats.IgnoredSkipped++
			continue
		}

		if entry.Type()&os.ModeSymlink != 0 {
			stat, err := os.Stat(childFsPath)
			if err != nil {
				c.stats.ErrorSkipped++
				continue
			}
			if c.opts.SameFilesystem {
				dev, err := deviceID(childFsPath)
				if err == nil && dev != rootDev {
					c.stats.IgnoredSkipped++
					continue
				}
			}

			if stat.IsDir() {
				if !c.opts.FollowSymlinks {
					c.stats.SymlinkDirSkipped++
					continue
				}
				nextDepth := depth + 1
				if c.opts.MaxDepth >= 0 && nextDepth > c.opts.MaxDepth {
					continue
				}
				real, err := filepath.EvalSymlinks(childFsPath)
				if err != nil {
					c.stats.ErrorSkipped++
					continue
				}
				if c.seen(real) {
					continue
				}
				if err := c.walkDir(real, childDisplay, nextDepth, rootDev); err != nil {
					return err
				}
				continue
			}

			if err := c.processFile(childFsPath, childDisplay, rootDev, false); err != nil {
				return err
			}
			continue
		}

		if entry.IsDir() {
			nextDepth := depth + 1
			if c.opts.MaxDepth >= 0 && nextDepth > c.opts.MaxDepth {
				continue
			}
			if c.opts.SameFilesystem {
				dev, err := deviceID(childFsPath)
				if err == nil && dev != rootDev {
					c.stats.IgnoredSkipped++
					continue
				}
			}
			if err := c.walkDir(childFsPath, childDisplay, nextDepth, rootDev); err != nil {
				return err
			}
			continue
		}

		if err := c.processFile(childFsPath, childDisplay, rootDev, false); err != nil {
			return err
		}
	}

	return nil
}

func (c *Collector) processFile(fsPath, displayPath string, rootDev uint64, explicit bool) error {
	if c.opts.SameFilesystem {
		dev, err := deviceID(fsPath)
		if err == nil && dev != rootDev {
			c.stats.IgnoredSkipped++
			return nil
		}
	}

	info, err := os.Stat(fsPath)
	if err != nil {
		c.stats.ErrorSkipped++
		return nil
	}
	if !info.Mode().IsRegular() {
		c.stats.NonRegularSkipped++
		return nil
	}

	if !explicit && !c.opts.IncludeHidden && isHiddenPath(displayPath) {
		c.stats.HiddenSkipped++
		return nil
	}

	if c.opts.Exclude != nil && c.opts.Exclude.Match(displayPath, false) {
		c.stats.IgnoredSkipped++
		return nil
	}

	if c.opts.Include != nil && !c.opts.Include.Empty() && !c.opts.Include.Match(displayPath, false) {
		c.stats.IgnoredSkipped++
		return nil
	}

	content, err := textfile.Read(fsPath)
	if err != nil {
		switch textfile.Reason(err) {
		case "binary":
			c.stats.BinarySkipped++
		case "empty":
			c.stats.EmptySkipped++
		default:
			c.stats.ErrorSkipped++
		}
		return nil
	}

	display := displayPath
	source := fsPath
	if abs, err := filepath.Abs(fsPath); err == nil {
		source = abs
	}
	if c.opts.FullPaths {
		abs, err := filepath.Abs(fsPath)
		if err == nil {
			display = normalizeDisplay(abs)
		}
	}

	c.files = append(c.files, File{
		DisplayPath: display,
		SourcePath:  source,
		Content:     content,
	})
	c.stats.FilesCopied++
	return nil
}

func (c *Collector) rootDisplay(root string) (string, error) {
	if c.opts.FullPaths {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		return normalizeDisplay(abs), nil
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(c.cwd, abs)
	if err != nil {
		return normalizeDisplay(root), nil
	}
	rel = normalizeDisplay(rel)
	if rel == "." {
		return "", nil
	}
	return rel, nil
}

func (c *Collector) seen(realPath string) bool {
	realPath = filepath.Clean(realPath)
	if c.visited[realPath] {
		return true
	}
	c.visited[realPath] = true
	return false
}

func joinDisplay(prefix, name string) string {
	name = normalizeDisplay(name)
	if prefix == "" || prefix == "." {
		return name
	}
	return path.Join(normalizeDisplay(prefix), name)
}

func normalizeDisplay(p string) string {
	p = filepath.Clean(p)
	p = strings.ReplaceAll(p, "\\", "/")
	if p == "." {
		return ""
	}
	return p
}

func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}

func isHiddenPath(displayPath string) bool {
	parts := strings.Split(normalizeDisplay(displayPath), "/")
	for _, part := range parts {
		if isHiddenName(part) {
			return true
		}
	}
	return false
}

func deviceID(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("unsupported stat type: %T", info.Sys())
	}
	return uint64(stat.Dev), nil
}
