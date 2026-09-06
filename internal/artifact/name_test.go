package artifact

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNamesPreserveSimpleDatabaseName(t *testing.T) {
	if got := DumpFileName("app-db"); got != "app-db.sql" {
		t.Fatalf("DumpFileName(app-db) = %q", got)
	}
	if got := ImageTag("app-db"); got != "psqldump-app-db:latest" {
		t.Fatalf("ImageTag(app-db) = %q", got)
	}
}

func TestNamesSafelyEncodeArbitraryDatabaseName(t *testing.T) {
	database := "../My Database/production"
	dumpName := DumpFileName(database)
	imageTag := ImageTag(database)

	if !filepath.IsLocal(dumpName) || filepath.Base(dumpName) != dumpName {
		t.Fatalf("DumpFileName(%q) is not a local filename: %q", database, dumpName)
	}
	if strings.ContainsAny(dumpName, `/\\`) {
		t.Fatalf("dump filename contains a path separator: %q", dumpName)
	}
	if imageTag != strings.ToLower(imageTag) || strings.ContainsAny(imageTag, ` /\\`) {
		t.Fatalf("image tag contains non-portable characters: %q", imageTag)
	}
	if DumpFileName(database) != dumpName || ImageTag(database) != imageTag {
		t.Fatal("generated names are not deterministic")
	}
}

func TestNamesDoNotCollideAfterNormalization(t *testing.T) {
	if DumpFileName("My Database") == DumpFileName("my-database") {
		t.Fatal("normalized dump filenames collide")
	}
	if ImageTag("My Database") == ImageTag("my-database") {
		t.Fatal("normalized image tags collide")
	}
}

func TestDumpFileNameAvoidsWindowsDeviceNames(t *testing.T) {
	for _, database := range []string{"con", "CON.backup", "NUL", "com1", "LPT9.log"} {
		if got := strings.TrimSuffix(DumpFileName(database), ".sql"); isWindowsReservedName(got) {
			t.Fatalf("DumpFileName(%q) uses reserved name %q", database, got)
		}
	}
}
