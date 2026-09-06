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
		Password:     "p@ss:$word",
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
		`build:`,
		`context: .`,
		`dockerfile: "psqldump.Dockerfile"`,
		`image: "psqldump-app:latest"`,
		`POSTGRES_DB: "app-db"`,
		`POSTGRES_USER: "read only"`,
		`POSTGRES_PASSWORD: "p@ss:$$word"`,
		`- "15432:5432"`,
		`test: ["CMD-SHELL", "pg_isready -U \"$$POSTGRES_USER\" -d \"$$POSTGRES_DB\""]`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("compose file does not contain %q:\n%s", want, content)
		}
	}
}

func TestGenerateRejectsIncompleteConfiguration(t *testing.T) {
	valid := Config{
		ImageName:    "psqldump-app:latest",
		DBName:       "app",
		User:         "postgres",
		Password:     "secret",
		ExternalPort: 5432,
		OutDir:       t.TempDir(),
	}

	tests := []struct {
		name   string
		change func(*Config)
	}{
		{name: "image", change: func(cfg *Config) { cfg.ImageName = "" }},
		{name: "database", change: func(cfg *Config) { cfg.DBName = "" }},
		{name: "password", change: func(cfg *Config) { cfg.Password = "" }},
		{name: "port", change: func(cfg *Config) { cfg.ExternalPort = 65536 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.change(&cfg)
			if _, err := Generate(cfg); err == nil {
				t.Fatal("Generate returned no error")
			}
		})
	}
}
