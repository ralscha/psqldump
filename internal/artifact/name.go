package artifact

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const maxSlugLength = 160

// DumpFileName returns a portable, collision-resistant filename for a database.
func DumpFileName(database string) string {
	return databaseSlug(database) + ".sql"
}

// ImageTag returns a valid local Docker image tag for a database.
func ImageTag(database string) string {
	return "psqldump-" + databaseSlug(database) + ":latest"
}

func databaseSlug(database string) string {
	lowerName := strings.ToLower(database)
	var slug strings.Builder
	var pendingSeparator byte

	for _, char := range lowerName {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			if pendingSeparator != 0 && slug.Len() > 0 {
				slug.WriteByte(pendingSeparator)
			}
			pendingSeparator = 0
			slug.WriteRune(char)
			continue
		}

		separator := byte('-')
		if char == '-' || char == '_' || char == '.' {
			separator = byte(char)
		}
		if pendingSeparator == 0 {
			pendingSeparator = separator
		} else {
			pendingSeparator = '-'
		}
	}

	normalized := slug.String()
	if normalized == "" {
		normalized = "database"
	}
	if len(normalized) > maxSlugLength {
		normalized = strings.TrimRight(normalized[:maxSlugLength], "-_.")
	}
	changed := normalized != database
	if isWindowsReservedName(normalized) {
		normalized = "database-" + normalized
		changed = true
	}
	if changed {
		hash := sha256.Sum256([]byte(database))
		normalized = fmt.Sprintf("%s-%x", normalized, hash[:8])
	}
	return normalized
}

func isWindowsReservedName(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	switch stem {
	case "con", "prn", "aux", "nul", "com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
		"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9":
		return true
	default:
		return false
	}
}
