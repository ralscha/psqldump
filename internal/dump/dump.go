package dump

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"psqldump/internal/postgres"
)

const pgClientImage = "postgres:alpine"

type Config struct {
	Host      string
	Port      int
	User      string
	Password  string
	DBName    string
	OutDir    string
	PgVersion string
}

func ServerVersion(cfg Config) (string, error) {
	return ServerVersionContext(context.Background(), cfg)
}

func ServerVersionContext(ctx context.Context, cfg Config) (string, error) {
	args := []string{
		"run", "--rm",
		"-e", "PGPASSWORD",
		pgClientImage,
		"psql",
		"-h", cfg.Host,
		"-p", fmt.Sprintf("%d", cfg.Port),
		"-U", cfg.User,
		"-d", cfg.DBName,
		"-t", "-A",
		"-c", "SELECT current_setting('server_version_num')",
	}

	// #nosec G204 -- args are built from config values; this is a Docker CLI wrapper
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = environmentWithPassword(cfg.Password)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker psql version query failed: %w", err)
	}

	raw := strings.TrimSpace(out.String())
	if raw == "" {
		return "", fmt.Errorf("empty version response from server")
	}

	major, err := postgres.VersionFromServerNumber(raw)
	if err != nil {
		return "", err
	}

	fmt.Printf("Detected PostgreSQL server version: %s (raw: %s)\n", major, raw)
	return major, nil
}

func Run(cfg Config) (string, error) {
	return RunContext(context.Background(), cfg)
}

func RunContext(ctx context.Context, cfg Config) (string, error) {
	if cfg.OutDir == "" {
		cfg.OutDir = "."
	}
	if err := os.MkdirAll(cfg.OutDir, 0o750); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	pgVer := cfg.PgVersion
	if pgVer == "" {
		v, err := ServerVersionContext(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("auto-detect pg version for dump: %w", err)
		}
		pgVer = v
	}
	if err := postgres.ValidateVersion(pgVer); err != nil {
		return "", err
	}

	clientImage := fmt.Sprintf("postgres:%s-alpine", pgVer)

	dumpFileName := cfg.DBName + ".sql"
	if !filepath.IsLocal(dumpFileName) || filepath.Base(dumpFileName) != dumpFileName {
		return "", fmt.Errorf("database name %q cannot be used as a portable dump filename", cfg.DBName)
	}
	dumpPath := filepath.Join(cfg.OutDir, dumpFileName)

	args := []string{
		"run", "--rm",
		"-e", "PGPASSWORD",
		clientImage,
		"pg_dump",
		"-h", cfg.Host,
		"-p", fmt.Sprintf("%d", cfg.Port),
		"-U", cfg.User,
		"-d", cfg.DBName,
		"--no-owner",
		"--no-acl",
	}

	fmt.Printf("Dumping %s@%s:%d/%s -> %s (via %s)\n", cfg.User, cfg.Host, cfg.Port, cfg.DBName, dumpPath, clientImage)

	// #nosec G204 -- args are built from config values; this is a Docker CLI wrapper
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = environmentWithPassword(cfg.Password)

	// #nosec G304 -- dumpPath is constructed from cfg.OutDir and cfg.DBName
	f, err := os.Create(dumpPath)
	if err != nil {
		return "", fmt.Errorf("create dump file: %w", err)
	}
	defer func() { _ = f.Close() }()

	cmd.Stdout = f
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		_ = os.Remove(dumpPath)
		return "", fmt.Errorf("docker pg_dump failed: %w", err)
	}

	fmt.Printf("Dump complete: %s (%d bytes)\n", dumpPath, fileSize(dumpPath))
	return dumpPath, nil
}

func environmentWithPassword(password string) []string {
	environment := os.Environ()
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if found && strings.EqualFold(name, "PGPASSWORD") {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "PGPASSWORD="+password)
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}
