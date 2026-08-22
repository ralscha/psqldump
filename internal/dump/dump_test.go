package dump

import (
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
