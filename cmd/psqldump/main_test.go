package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOptionsUsesRemotePortAsExternalPortDefault(t *testing.T) {
	t.Setenv("PSQLDUMP_TARGET_PASSWORD", "local-secret")
	opts, err := parseOptions("compose", []string{
		"--dbname", "app",
		"--port", "6543",
	})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}

	if opts.port != 6543 {
		t.Fatalf("port = %d, want 6543", opts.port)
	}
	if opts.externalPort != 6543 {
		t.Fatalf("externalPort = %d, want 6543", opts.externalPort)
	}
}

func TestParseOptionsExternalPortOverride(t *testing.T) {
	t.Setenv("PSQLDUMP_TARGET_PASSWORD", "local-secret")
	opts, err := parseOptions("compose", []string{
		"-d", "app",
		"-P", "6543",
		"-E", "15432",
	})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}

	if opts.externalPort != 15432 {
		t.Fatalf("externalPort = %d, want 15432", opts.externalPort)
	}
}

func TestParseOptionsRejectsUnexpectedArguments(t *testing.T) {
	t.Setenv("PSQLDUMP_TARGET_PASSWORD", "local-secret")
	_, err := parseOptions("compose", []string{"-d", "app", "extra"})
	if err == nil || !strings.Contains(err.Error(), "unexpected arguments") {
		t.Fatalf("parseOptions error = %v, want unexpected arguments error", err)
	}
}

func TestParseOptionsAcceptsDatabaseNameRequiringSafeArtifactName(t *testing.T) {
	t.Setenv("PSQLDUMP_TARGET_PASSWORD", "local-secret")
	opts, err := parseOptions("compose", []string{"-d", "../My Database"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.dbName != "../My Database" {
		t.Fatalf("dbName = %q", opts.dbName)
	}
}

func TestParseOptionsRejectsInvalidPostgresVersion(t *testing.T) {
	t.Setenv("PSQLDUMP_TARGET_PASSWORD", "local-secret")
	_, err := parseOptions("compose", []string{"-d", "app", "--pg-version", "16-alpine"})
	if err == nil || !strings.Contains(err.Error(), "invalid PostgreSQL version") {
		t.Fatalf("parseOptions error = %v, want invalid version error", err)
	}
}

func TestParseOptionsKeepsSourceAndTargetPasswordsSeparate(t *testing.T) {
	t.Setenv("PGPASSWORD", "source-secret")
	t.Setenv("PSQLDUMP_TARGET_PASSWORD", "target-secret")

	opts, err := parseOptions("all", []string{"-d", "app"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.password != "source-secret" || opts.targetPass != "target-secret" {
		t.Fatalf("passwords = source %q, target %q", opts.password, opts.targetPass)
	}
}

func TestParseOptionsRequiresTargetPasswordForPortableBundle(t *testing.T) {
	t.Setenv("PSQLDUMP_TARGET_PASSWORD", "")

	for _, command := range []string{"compose", "all"} {
		_, err := parseOptions(command, []string{"-d", "app"})
		if err == nil || !strings.Contains(err.Error(), "--target-password") {
			t.Fatalf("parseOptions(%q) error = %v, want target password error", command, err)
		}
	}
}

func TestRunContextRejectsUnknownCommandBeforeParsingFlags(t *testing.T) {
	err := runContext(context.Background(), []string{"unknown"})
	if err == nil || !strings.Contains(err.Error(), `unknown command "unknown"`) {
		t.Fatalf("runContext error = %v, want unknown command error", err)
	}
}

func TestRunBuildRejectsNonRegularDumpBeforeUsingDocker(t *testing.T) {
	outDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(outDir, "app.sql"), 0o750); err != nil {
		t.Fatalf("create directory in place of dump: %v", err)
	}

	err := runBuild(context.Background(), options{
		dbName: "app",
		outDir: outDir,
		pgVer:  "16",
	})
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("runBuild error = %v, want non-regular file error", err)
	}
}
