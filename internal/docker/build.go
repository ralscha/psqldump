package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/moby/moby/client"
)

type BuildConfig struct {
	DumpPath  string
	DBName    string
	User      string
	Password  string
	ImageTag  string
	PgVersion string
}

func BuildImage(ctx context.Context, cfg BuildConfig) error {
	if cfg.PgVersion == "" {
		cfg.PgVersion = "16"
	}
	if cfg.User == "" {
		cfg.User = "postgres"
	}

	cli, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	dumpData, err := os.ReadFile(cfg.DumpPath)
	if err != nil {
		return fmt.Errorf("read dump file: %w", err)
	}

	dumpFileName := filepath.Base(cfg.DumpPath)

	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	dockerfile := fmt.Sprintf(`FROM postgres:%s
COPY %s /docker-entrypoint-initdb.d/
`, cfg.PgVersion, dumpFileName)

	if err := addFileToTar(tw, "Dockerfile", []byte(dockerfile)); err != nil {
		return fmt.Errorf("add Dockerfile to tar: %w", err)
	}

	if err := addFileToTar(tw, dumpFileName, dumpData); err != nil {
		return fmt.Errorf("add dump to tar: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}

	fmt.Printf("Building image %s (postgres:%s + %s)...\n", cfg.ImageTag, cfg.PgVersion, dumpFileName)

	resp, err := cli.ImageBuild(ctx, tarBuf, client.ImageBuildOptions{
		Tags:        []string{cfg.ImageTag},
		Remove:      true,
		ForceRemove: true,
	})
	if err != nil {
		return fmt.Errorf("image build: %w", err)
	}
	defer resp.Body.Close()

	decodedOutput := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(decodedOutput)
		if n > 0 {
			os.Stdout.Write(decodedOutput[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read build output: %w", err)
		}
	}

	_, err = cli.ImageInspect(ctx, cfg.ImageTag)
	if err != nil {
		return fmt.Errorf("verify built image: %w", err)
	}

	fmt.Printf("Image built successfully: %s\n", cfg.ImageTag)
	return nil
}

func PullPostgres(ctx context.Context, version string) error {
	if version == "" {
		version = "16"
	}

	cli, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	ref := fmt.Sprintf("postgres:%s", version)

	_, err = cli.ImageInspect(ctx, ref)
	if err == nil {
		fmt.Printf("Base image %s already present.\n", ref)
		return nil
	}

	fmt.Printf("Pulling %s...\n", ref)
	resp, err := cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	defer resp.Close()

	io.Copy(io.Discard, resp)
	fmt.Printf("Pulled %s\n", ref)
	return nil
}

func addFileToTar(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
