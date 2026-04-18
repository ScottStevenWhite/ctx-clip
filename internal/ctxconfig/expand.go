package ctxconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openai/ctx-clip/internal/matcher"
	"github.com/openai/ctx-clip/internal/scan"
)

type dirConfig struct {
	ctxPath  string
	rules    []compiledRule
	warnings []string
}

type compiledRule struct {
	matcher *matcher.Matcher
	related []string
}

func ExpandMappedFiles(files []scan.File, fullPaths bool) ([]scan.File, scan.Stats, []string, error) {
	if len(files) == 0 {
		return nil, scan.Stats{}, nil, nil
	}

	resolver := mappingResolver{
		baseDir:   inferBaseDir(files, fullPaths),
		fullPaths: fullPaths,
		cache:     map[string]dirConfig{},
	}
	return resolver.expand(files)
}

type mappingResolver struct {
	baseDir   string
	fullPaths bool
	cache     map[string]dirConfig
}

func (r *mappingResolver) expand(files []scan.File) ([]scan.File, scan.Stats, []string, error) {
	allFiles := append([]scan.File{}, files...)
	allStats := scan.Stats{}
	var warnings []string
	warningSeen := map[string]bool{}
	knownFiles := map[string]bool{}
	expandedTargets := map[string]bool{}

	var pending []scan.File
	for _, file := range files {
		key, err := fileKey(file.SourcePath)
		if err != nil || knownFiles[key] {
			continue
		}
		knownFiles[key] = true
		pending = append(pending, file)
	}

	for len(pending) > 0 {
		nextRoots := map[string]string{}

		for _, file := range pending {
			key, err := fileKey(file.SourcePath)
			if err != nil || expandedTargets[key] {
				continue
			}
			expandedTargets[key] = true

			sourcePath, err := absolutePath(file.SourcePath)
			if err != nil {
				continue
			}

			dir := filepath.Dir(sourcePath)
			cfg := r.loadDir(dir)
			for _, warning := range cfg.warnings {
				appendWarning(&warnings, warningSeen, warning)
			}

			relTarget, err := filepath.Rel(dir, sourcePath)
			if err != nil {
				continue
			}
			relTarget = normalizePath(relTarget)

			for _, rule := range cfg.rules {
				if !rule.matcher.Match(relTarget, false) {
					continue
				}

				for _, related := range rule.related {
					resolved := related
					if !filepath.IsAbs(related) {
						resolved = filepath.Join(dir, related)
					}
					resolved = filepath.Clean(resolved)

					info, err := os.Stat(resolved)
					if err != nil {
						if os.IsNotExist(err) {
							appendWarning(
								&warnings,
								warningSeen,
								fmt.Sprintf(
									"ctx-clip: mapped file not found for %s via %s: %s was not found. Please update .ctx file with new location or remove this mapping.",
									relTarget,
									cfg.ctxPath,
									related,
								),
							)
							continue
						}
						appendWarning(
							&warnings,
							warningSeen,
							fmt.Sprintf(
								"ctx-clip: could not inspect mapped file for %s via %s: %s (%v)",
								relTarget,
								cfg.ctxPath,
								related,
								err,
							),
						)
						continue
					}
					if info.IsDir() {
						appendWarning(
							&warnings,
							warningSeen,
							fmt.Sprintf(
								"ctx-clip: mapped path for %s via %s points to a directory: %s. Only files are supported in mappings.",
								relTarget,
								cfg.ctxPath,
								related,
							),
						)
						continue
					}

					relatedKey, err := fileKey(resolved)
					if err != nil || knownFiles[relatedKey] {
						continue
					}
					nextRoots[relatedKey] = resolved
				}
			}
		}

		if len(nextRoots) == 0 {
			break
		}

		roots := make([]string, 0, len(nextRoots))
		for _, root := range nextRoots {
			roots = append(roots, root)
		}
		sort.Strings(roots)

		batchFiles, batchStats, err := scan.Collect(scan.Options{
			Roots:          roots,
			MaxDepth:       -1,
			IncludeHidden:  true,
			FollowSymlinks: true,
			FullPaths:      r.fullPaths,
		})
		if err != nil {
			return nil, scan.Stats{}, warnings, err
		}
		if !r.fullPaths {
			for i := range batchFiles {
				if rel, ok := r.displayPathFor(batchFiles[i].SourcePath); ok {
					batchFiles[i].DisplayPath = rel
				}
			}
		}

		allStats = mergeStats(allStats, batchStats)
		pending = pending[:0]
		for _, file := range batchFiles {
			key, err := fileKey(file.SourcePath)
			if err != nil || knownFiles[key] {
				continue
			}
			knownFiles[key] = true
			allFiles = append(allFiles, file)
			pending = append(pending, file)
		}
	}

	sort.SliceStable(allFiles, func(i, j int) bool {
		return allFiles[i].DisplayPath < allFiles[j].DisplayPath
	})

	return allFiles, allStats, warnings, nil
}

func (r *mappingResolver) loadDir(dir string) dirConfig {
	dir = filepath.Clean(dir)
	if cached, ok := r.cache[dir]; ok {
		return cached
	}

	cfg := dirConfig{
		ctxPath: filepath.Join(dir, ".ctx"),
	}

	if _, err := os.Stat(cfg.ctxPath); err != nil {
		if !os.IsNotExist(err) {
			cfg.warnings = append(cfg.warnings, fmt.Sprintf("ctx-clip: could not inspect %s: %v", cfg.ctxPath, err))
		}
		r.cache[dir] = cfg
		return cfg
	}

	parsed, err := Load(cfg.ctxPath)
	if err != nil {
		cfg.warnings = append(cfg.warnings, fmt.Sprintf("ctx-clip: could not load %s: %v", cfg.ctxPath, err))
		r.cache[dir] = cfg
		return cfg
	}

	for _, mapping := range parsed.Mappings {
		compiled, err := matcher.Compile([]string{mapping.Target})
		if err != nil {
			cfg.warnings = append(
				cfg.warnings,
				fmt.Sprintf("ctx-clip: invalid mapped target pattern %q in %s: %v", mapping.Target, cfg.ctxPath, err),
			)
			continue
		}
		cfg.rules = append(cfg.rules, compiledRule{
			matcher: compiled,
			related: append([]string{}, mapping.Related...),
		})
	}

	r.cache[dir] = cfg
	return cfg
}

func (r *mappingResolver) displayPathFor(sourcePath string) (string, bool) {
	if r.baseDir == "" {
		return "", false
	}
	rel, err := filepath.Rel(r.baseDir, sourcePath)
	if err != nil {
		return "", false
	}
	return normalizePath(rel), true
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

func appendWarning(dst *[]string, seen map[string]bool, warning string) {
	if warning == "" || seen[warning] {
		return
	}
	seen[warning] = true
	*dst = append(*dst, warning)
}

func fileKey(path string) (string, error) {
	abs, err := absolutePath(path)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(real), nil
	}
	return filepath.Clean(abs), nil
}

func absolutePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func normalizePath(path string) string {
	path = filepath.Clean(path)
	path = strings.ReplaceAll(path, "\\", "/")
	if path == "." {
		return ""
	}
	return path
}

func inferBaseDir(files []scan.File, fullPaths bool) string {
	if fullPaths || len(files) == 0 {
		return ""
	}

	sourcePath := filepath.Clean(files[0].SourcePath)
	displayPath := normalizePath(files[0].DisplayPath)
	if displayPath == "" {
		return filepath.Dir(sourcePath)
	}

	displayParts := strings.Split(displayPath, "/")
	base := sourcePath
	for range displayParts {
		base = filepath.Dir(base)
	}
	return filepath.Clean(base)
}
