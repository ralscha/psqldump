package dump

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	args := []string{
		"run", "--rm",
		"-e", "PGPASSWORD=" + cfg.Password,
		pgClientImage,
		"psql",
		"-h", cfg.Host,
		"-p", fmt.Sprintf("%d", cfg.Port),
		"-U", cfg.User,
		"-d", cfg.DBName,
		"-t", "-A",
		"-c", "SELECT current_setting('server_version_num')",
	}

	cmd := exec.Command("docker", args...)
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

	verNum, err := strconv.Atoi(raw)
	if err != nil {
		return "", fmt.Errorf("parse version %q: %w", raw, err)
	}

	major := strconv.Itoa(verNum / 10000)
	fmt.Printf("Detected PostgreSQL server version: %s (raw: %s)\n", major, raw)
	return major, nil
}

func Run(cfg Config) (string, error) {
	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	pgVer := cfg.PgVersion
	if pgVer == "" {
		v, err := ServerVersion(cfg)
		if err != nil {
			return "", fmt.Errorf("auto-detect pg version for dump: %w", err)
		}
		pgVer = v
	}

	clientImage := fmt.Sprintf("postgres:%s-alpine", pgVer)

	dumpPath := filepath.Join(cfg.OutDir, cfg.DBName+".sql")

	args := []string{
		"run", "--rm",
		"-e", "PGPASSWORD=" + cfg.Password,
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

	cmd := exec.Command("docker", args...)

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

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}
