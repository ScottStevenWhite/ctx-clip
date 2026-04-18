package matcher

import "testing"

func TestMatcherNodeModulesAndJSON(t *testing.T) {
	m, err := Compile([]string{"node_modules|*.json"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	cases := []struct {
		path string
		dir  bool
		want bool
	}{
		{path: "node_modules", dir: true, want: true},
		{path: "web/node_modules", dir: true, want: true},
		{path: "web/src/file.json", dir: false, want: true},
		{path: "web/src/file.ts", dir: false, want: false},
		{path: "server/package-lock.json", dir: false, want: true},
	}

	for _, tc := range cases {
		got := m.Match(tc.path, tc.dir)
		if got != tc.want {
			t.Fatalf("Match(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestMatcherRawRegex(t *testing.T) {
	m, err := Compile([]string{`re:^(server|web)/src/.*\.(ts|tsx)$`})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !m.Match("server/src/server.ts", false) {
		t.Fatal("expected regex matcher to match server/src/server.ts")
	}
	if m.Match("server/package.json", false) {
		t.Fatal("did not expect regex matcher to match package.json")
	}
}

func TestMatcherGlobstarMatchesRootAndNestedPaths(t *testing.T) {
	m, err := Compile([]string{"**/*.md"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !m.Match("README.md", false) {
		t.Fatal("expected **/*.md to match README.md")
	}
	if !m.Match("docs/adr/example.md", false) {
		t.Fatal("expected **/*.md to match docs/adr/example.md")
	}
}

func TestMatcherGlobstarMiddleSegmentMayBeEmpty(t *testing.T) {
	m, err := Compile([]string{"docs/**/adr.md"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !m.Match("docs/adr.md", false) {
		t.Fatal("expected docs/**/adr.md to match docs/adr.md")
	}
	if !m.Match("docs/reference/adr.md", false) {
		t.Fatal("expected docs/**/adr.md to match docs/reference/adr.md")
	}
}

func TestMatcherLiteralPathFragmentMatchesNestedPath(t *testing.T) {
	m, err := Compile([]string{"web/public"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !m.Match("apps/web/public/index.html", false) {
		t.Fatal("expected literal path fragment to match nested path")
	}
	if m.Match("apps/web/private/index.html", false) {
		t.Fatal("did not expect literal path fragment to match wrong path")
	}
}

func TestMatcherSplitAlternatesKeepsPipesInsideCharacterClasses(t *testing.T) {
	m, err := Compile([]string{`file[|].txt|*.md`})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !m.Match("file|.txt", false) {
		t.Fatal("expected character class alternate to match file|.txt")
	}
	if !m.Match("README.md", false) {
		t.Fatal("expected *.md alternate to match README.md")
	}
}
