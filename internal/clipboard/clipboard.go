package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Backend struct {
	Name string
	Cmd  []string
}

func Detect() (*Backend, error) {
	candidates := orderedCandidates()
	for _, c := range candidates {
		if len(c.Cmd) == 0 {
			continue
		}
		if _, err := exec.LookPath(c.Cmd[0]); err == nil {
			return &c, nil
		}
	}
	return nil, fmt.Errorf("no clipboard backend found (tried wl-copy, xclip, xsel, pbcopy, clip.exe)")
}

func Copy(text string) (string, error) {
	backend, err := Detect()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(backend.Cmd[0], backend.Cmd[1:]...)
	cmd.Stdin = strings.NewReader(text)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return backend.Name, fmt.Errorf("clipboard copy failed with %s: %w", backend.Name, err)
	}
	return backend.Name, nil
}

func orderedCandidates() []Backend {
	wayland := os.Getenv("WAYLAND_DISPLAY") != "" || strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland")
	x11 := os.Getenv("DISPLAY") != ""

	var out []Backend
	if wayland {
		out = append(out, Backend{Name: "wl-copy", Cmd: []string{"wl-copy"}})
	}
	if x11 {
		out = append(out,
			Backend{Name: "xclip", Cmd: []string{"xclip", "-selection", "clipboard"}},
			Backend{Name: "xsel", Cmd: []string{"xsel", "--clipboard", "--input"}},
		)
	}

	// Keep fallbacks regardless of desktop session. Useful over SSH or in mixed environments.
	out = append(out,
		Backend{Name: "wl-copy", Cmd: []string{"wl-copy"}},
		Backend{Name: "xclip", Cmd: []string{"xclip", "-selection", "clipboard"}},
		Backend{Name: "xsel", Cmd: []string{"xsel", "--clipboard", "--input"}},
		Backend{Name: "pbcopy", Cmd: []string{"pbcopy"}},
		Backend{Name: "clip.exe", Cmd: []string{"clip.exe"}},
	)

	return dedupe(out)
}

func dedupe(in []Backend) []Backend {
	seen := map[string]bool{}
	out := make([]Backend, 0, len(in))
	for _, b := range in {
		key := strings.Join(b.Cmd, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, b)
	}
	return out
}
