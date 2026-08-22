package main

import (
	"strings"
	"testing"
)

func TestParseOptionsUsesRemotePortAsExternalPortDefault(t *testing.T) {
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
	_, err := parseOptions("compose", []string{"-d", "app", "extra"})
	if err == nil || !strings.Contains(err.Error(), "unexpected arguments") {
		t.Fatalf("parseOptions error = %v, want unexpected arguments error", err)
	}
}

func TestParseOptionsRejectsNonPortableDatabaseFilename(t *testing.T) {
	_, err := parseOptions("compose", []string{"-d", "../app"})
	if err == nil || !strings.Contains(err.Error(), "portable dump filename") {
		t.Fatalf("parseOptions error = %v, want portable filename error", err)
	}
}

func TestParseOptionsRejectsInvalidPostgresVersion(t *testing.T) {
	_, err := parseOptions("compose", []string{"-d", "app", "--pg-version", "16-alpine"})
	if err == nil || !strings.Contains(err.Error(), "invalid PostgreSQL version") {
		t.Fatalf("parseOptions error = %v, want invalid version error", err)
	}
}

func TestImageTagForDatabase(t *testing.T) {
	if got := imageTagForDatabase("app-db"); got != "psqldump-app-db:latest" {
		t.Fatalf("imageTagForDatabase(app-db) = %q", got)
	}

	first := imageTagForDatabase("My Database")
	second := imageTagForDatabase("My Database")
	if first != second {
		t.Fatalf("image tag is not deterministic: %q != %q", first, second)
	}
	if first != strings.ToLower(first) || strings.Contains(first, " ") {
		t.Fatalf("image tag contains non-portable characters: %q", first)
	}
	if first == imageTagForDatabase("my-database") {
		t.Fatalf("normalized image tags collide: %q", first)
	}
}
