package postgres

import (
	"fmt"
	"regexp"
	"strconv"
)

var versionPattern = regexp.MustCompile(`^[1-9][0-9]*(?:\.[0-9]+)?$`)

func ValidateVersion(version string) error {
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("invalid PostgreSQL version %q (expected a major version such as 16 or 9.6)", version)
	}
	return nil
}

func VersionFromServerNumber(raw string) (string, error) {
	versionNumber, err := strconv.Atoi(raw)
	if err != nil {
		return "", fmt.Errorf("parse version %q: %w", raw, err)
	}
	if versionNumber < 10000 {
		return "", fmt.Errorf("invalid server version number %q", raw)
	}

	major := versionNumber / 10000
	if versionNumber >= 100000 {
		return strconv.Itoa(major), nil
	}

	minor := (versionNumber / 100) % 100
	return fmt.Sprintf("%d.%d", major, minor), nil
}
