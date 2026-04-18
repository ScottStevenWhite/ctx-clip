package clipboard

import "testing"

func TestOrderedCandidatesPreferSessionBackendsWithoutDuplicates(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("XDG_SESSION_TYPE", "wayland")

	candidates := orderedCandidates()
	if len(candidates) < 5 {
		t.Fatalf("len(candidates) = %d, want at least 5", len(candidates))
	}

	if candidates[0].Name != "wl-copy" {
		t.Fatalf("candidates[0].Name = %q, want wl-copy", candidates[0].Name)
	}
	if candidates[1].Name != "xclip" {
		t.Fatalf("candidates[1].Name = %q, want xclip", candidates[1].Name)
	}
	if candidates[2].Name != "xsel" {
		t.Fatalf("candidates[2].Name = %q, want xsel", candidates[2].Name)
	}

	seen := map[string]bool{}
	for _, candidate := range candidates {
		key := candidate.Name + "\x00" + joinCmd(candidate.Cmd)
		if seen[key] {
			t.Fatalf("duplicate candidate found: %#v", candidate)
		}
		seen[key] = true
	}
}

func TestOrderedCandidatesFallbacksWithoutDesktopSession(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_SESSION_TYPE", "")

	candidates := orderedCandidates()
	want := []string{"wl-copy", "xclip", "xsel", "pbcopy", "clip.exe"}
	if len(candidates) != len(want) {
		t.Fatalf("len(candidates) = %d, want %d", len(candidates), len(want))
	}
	for i, name := range want {
		if candidates[i].Name != name {
			t.Fatalf("candidates[%d].Name = %q, want %q", i, candidates[i].Name, name)
		}
	}
}

func joinCmd(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += "\x00"
		}
		out += part
	}
	return out
}
