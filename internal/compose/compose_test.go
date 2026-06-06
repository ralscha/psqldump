package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWritesExternalPortAndQuotedEnvironment(t *testing.T) {
	outDir := t.TempDir()

	composePath, err := Generate(Config{
		ImageName:    "psqldump-app:latest",
		DBName:       "app-db",
		User:         "read only",
		Password:     "p@ss:word",
		ExternalPort: 15432,
		OutDir:       outDir,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if composePath != filepath.Join(outDir, "docker-compose.yml") {
		t.Fatalf("composePath = %q, want docker-compose.yml in temp dir", composePath)
	}

	// #nosec G304 -- composePath is constructed from t.TempDir() and a fixed filename
	contentBytes, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	content := string(contentBytes)

	for _, want := range []string{
		`POSTGRES_DB: "app-db"`,
		`POSTGRES_USER: "read only"`,
		`POSTGRES_PASSWORD: "p@ss:word"`,
		`- "15432:5432"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("compose file does not contain %q:\n%s", want, content)
		}
	}
}
