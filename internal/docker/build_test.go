package docker

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePortableBuildFilesAndStreamContext(t *testing.T) {
	outDir := t.TempDir()
	dumpPath := filepath.Join(outDir, "app database.sql")
	dump := []byte("SELECT 1;\n")
	if err := os.WriteFile(dumpPath, dump, 0o600); err != nil {
		t.Fatalf("write dump: %v", err)
	}

	dockerfile, err := writePortableBuildFiles(BuildConfig{
		DumpPath:  dumpPath,
		PgVersion: "16",
	})
	if err != nil {
		t.Fatalf("writePortableBuildFiles returned error: %v", err)
	}

	wantDockerfile := "FROM postgres:16\n" +
		"COPY [\"app database.sql\",\"/docker-entrypoint-initdb.d/10-psqldump.sql\"]\n"
	if string(dockerfile) != wantDockerfile {
		t.Fatalf("Dockerfile = %q, want %q", dockerfile, wantDockerfile)
	}

	// #nosec G304 -- the path is under t.TempDir and uses a fixed filename
	written, err := os.ReadFile(filepath.Join(outDir, DockerfileName))
	if err != nil {
		t.Fatalf("read portable Dockerfile: %v", err)
	}
	if !bytes.Equal(written, dockerfile) {
		t.Fatalf("written Dockerfile = %q, want %q", written, dockerfile)
	}
	// #nosec G304 -- the path is under t.TempDir and uses a fixed filename
	ignore, err := os.ReadFile(filepath.Join(outDir, DockerignoreName))
	if err != nil {
		t.Fatalf("read portable Docker ignore file: %v", err)
	}
	if string(ignore) != dockerignoreContent {
		t.Fatalf("Docker ignore file = %q, want %q", ignore, dockerignoreContent)
	}

	var contextData bytes.Buffer
	if err := writeBuildContext(&contextData, dumpPath, dockerfile); err != nil {
		t.Fatalf("writeBuildContext returned error: %v", err)
	}

	files := make(map[string][]byte)
	tarReader := tar.NewReader(&contextData)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read tar header: %v", err)
		}
		contents, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read %s from tar: %v", header.Name, err)
		}
		files[header.Name] = contents
	}

	if !bytes.Equal(files[DockerfileName], dockerfile) {
		t.Fatalf("tar Dockerfile = %q, want %q", files[DockerfileName], dockerfile)
	}
	if !bytes.Equal(files[filepath.Base(dumpPath)], dump) {
		t.Fatalf("tar dump = %q, want %q", files[filepath.Base(dumpPath)], dump)
	}
	if len(files) != 2 {
		t.Fatalf("tar contains %d files, want 2", len(files))
	}
}

func TestDockerfileContentRejectsVersionInjection(t *testing.T) {
	_, err := dockerfileContent("16\nRUN whoami", "dump.sql")
	if err == nil || !strings.Contains(err.Error(), "invalid PostgreSQL version") {
		t.Fatalf("dockerfileContent error = %v, want invalid version error", err)
	}
}
