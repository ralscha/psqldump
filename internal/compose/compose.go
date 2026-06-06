package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	ImageName    string
	DBName       string
	User         string
	Password     string
	ExternalPort int
	OutDir       string
	ComposeFile  string
}

func Generate(cfg Config) (string, error) {
	if cfg.ComposeFile == "" {
		cfg.ComposeFile = "docker-compose.yml"
	}
	if cfg.ExternalPort == 0 {
		cfg.ExternalPort = 5432
	}
	if cfg.User == "" {
		cfg.User = "postgres"
	}

	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	composePath := filepath.Join(cfg.OutDir, cfg.ComposeFile)

	content := fmt.Sprintf(`services:
  postgres:
    image: %s
    restart: unless-stopped
    environment:
      POSTGRES_DB: %s
      POSTGRES_USER: %s
      POSTGRES_PASSWORD: %s
    ports:
      - "%d:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
`, cfg.ImageName, quoteYAMLString(cfg.DBName), quoteYAMLString(cfg.User), quoteYAMLString(cfg.Password), cfg.ExternalPort)

	if err := os.WriteFile(composePath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}

	fmt.Printf("Generated %s\n", composePath)
	return composePath, nil
}

func quoteYAMLString(value string) string {
	return strconv.Quote(value)
}
