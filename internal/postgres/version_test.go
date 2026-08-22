package postgres

import "testing"

func TestVersionFromServerNumber(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "180002", want: "18"},
		{raw: "100000", want: "10"},
		{raw: "90624", want: "9.6"},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			got, err := VersionFromServerNumber(test.raw)
			if err != nil {
				t.Fatalf("VersionFromServerNumber returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("VersionFromServerNumber(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestVersionFromServerNumberRejectsInvalidInput(t *testing.T) {
	for _, raw := range []string{"", "not-a-version", "9999"} {
		if _, err := VersionFromServerNumber(raw); err == nil {
			t.Fatalf("VersionFromServerNumber(%q) returned no error", raw)
		}
	}
}

func TestValidateVersionRejectsDockerfileSyntax(t *testing.T) {
	if err := ValidateVersion("16\nRUN whoami"); err == nil {
		t.Fatal("ValidateVersion returned no error for injected Dockerfile syntax")
	}
}
