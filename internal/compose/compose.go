package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"psqldump/internal/fileutil"
)

type Config struct {
	ImageName    string
	Dockerfile   string
	DBName       string
	User         string
	Password     string
	ExternalPort int
	OutDir       string
	ComposeFile  string
}

func Generate(cfg Config) (string, error) {
	if cfg.ImageName == "" {
		return "", fmt.Errorf("image name is required")
	}
	if cfg.DBName == "" {
		return "", fmt.Errorf("database name is required")
	}
	if cfg.Password == "" {
		return "", fmt.Errorf("target database password is required")
	}
	if cfg.ComposeFile == "" {
		cfg.ComposeFile = "docker-compose.yml"
	}
	if cfg.ExternalPort == 0 {
		cfg.ExternalPort = 5432
	}
	if cfg.ExternalPort < 1 || cfg.ExternalPort > 65535 {
		return "", fmt.Errorf("invalid external port: %d", cfg.ExternalPort)
	}
	if cfg.User == "" {
		cfg.User = "postgres"
	}
	if cfg.OutDir == "" {
		cfg.OutDir = "."
	}
	if cfg.Dockerfile == "" {
		cfg.Dockerfile = "psqldump.Dockerfile"
	}

	if err := os.MkdirAll(cfg.OutDir, 0o750); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	composePath := filepath.Join(cfg.OutDir, cfg.ComposeFile)

	content := fmt.Sprintf(`services:
  postgres:
    build:
      context: .
      dockerfile: %s
    image: %s
    restart: unless-stopped
    environment:
      POSTGRES_DB: %s
      POSTGRES_USER: %s
      POSTGRES_PASSWORD: %s
    ports:
      - "%d:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U \"$$POSTGRES_USER\" -d \"$$POSTGRES_DB\""]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 10s
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
`, quoteComposeString(cfg.Dockerfile), quoteComposeString(cfg.ImageName), quoteComposeString(cfg.DBName), quoteComposeString(cfg.User), quoteComposeString(cfg.Password), cfg.ExternalPort)

	if err := fileutil.WriteFileAtomic(composePath, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}

	fmt.Printf("Generated %s\n", composePath)
	return composePath, nil
}

func quoteComposeString(value string) string {
	return strconv.Quote(strings.ReplaceAll(value, "$", "$$"))
}
