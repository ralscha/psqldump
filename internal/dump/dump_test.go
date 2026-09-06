package dump

import (
	"context"
	"strings"
	"testing"
)

func TestEnvironmentWithPasswordReplacesExistingValue(t *testing.T) {
	t.Setenv("PGPASSWORD", "old-secret")
	environment := environmentWithPassword("new-secret")

	var passwordEntries []string
	for _, entry := range environment {
		if strings.HasPrefix(strings.ToUpper(entry), "PGPASSWORD=") {
			passwordEntries = append(passwordEntries, entry)
		}
	}

	if len(passwordEntries) != 1 {
		t.Fatalf("found %d PGPASSWORD entries, want 1: %v", len(passwordEntries), passwordEntries)
	}
	if passwordEntries[0] != "PGPASSWORD=new-secret" {
		t.Fatalf("PGPASSWORD entry = %q", passwordEntries[0])
	}
}

func TestCommandsRejectInvalidConnectionConfigBeforeRunningDocker(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "missing host", cfg: Config{Port: 5432, User: "postgres", DBName: "app"}},
		{name: "invalid port", cfg: Config{Host: "db.example.com", Port: 0, User: "postgres", DBName: "app"}},
		{name: "missing user", cfg: Config{Host: "db.example.com", Port: 5432, DBName: "app"}},
		{name: "missing database", cfg: Config{Host: "db.example.com", Port: 5432, User: "postgres"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ServerVersionContext(context.Background(), test.cfg); err == nil {
				t.Fatal("ServerVersionContext returned no error")
			}
			if _, err := RunContext(context.Background(), test.cfg); err == nil {
				t.Fatal("RunContext returned no error")
			}
		})
	}
}
