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
