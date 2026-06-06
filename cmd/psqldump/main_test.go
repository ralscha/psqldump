package main

import "testing"

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
