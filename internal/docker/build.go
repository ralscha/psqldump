package docker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"

	"psqldump/internal/postgres"
)

const (
	DockerfileName   = "psqldump.Dockerfile"
	DockerignoreName = DockerfileName + ".dockerignore"
	initDumpPath     = "/docker-entrypoint-initdb.d/10-psqldump.sql"
)

const dockerignoreContent = "*\n!*.sql\n!" + DockerfileName + "\n"

type BuildConfig struct {
	DumpPath  string
	ImageTag  string
	PgVersion string
}

func BuildImage(ctx context.Context, cfg BuildConfig) error {
	if cfg.PgVersion == "" {
		cfg.PgVersion = "16"
	}
	if err := postgres.ValidateVersion(cfg.PgVersion); err != nil {
		return err
	}
	if cfg.ImageTag == "" {
		return errors.New("image tag is required")
	}

	dockerfile, err := writePortableBuildFiles(cfg)
	if err != nil {
		return err
	}

	cli, err := client.New(client.FromEnv)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	buildContext, waitForContext := streamBuildContext(cfg.DumpPath, dockerfile)
	defer func() { _ = buildContext.Close() }()

	dumpFileName := filepath.Base(cfg.DumpPath)
	fmt.Printf("Building image %s (postgres:%s + %s)...\n", cfg.ImageTag, cfg.PgVersion, dumpFileName)

	resp, err := cli.ImageBuild(ctx, buildContext, client.ImageBuildOptions{
		Dockerfile:  DockerfileName,
		Tags:        []string{cfg.ImageTag},
		Remove:      true,
		ForceRemove: true,
	})
	if err != nil {
		_ = buildContext.CloseWithError(err)
		_ = waitForContext()
		return fmt.Errorf("image build: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := waitForContext(); err != nil {
		return fmt.Errorf("create image build context: %w", err)
	}
	if err := jsonmessage.DisplayStream(resp.Body, os.Stdout); err != nil {
		return fmt.Errorf("image build failed: %w", err)
	}

	if _, err := cli.ImageInspect(ctx, cfg.ImageTag); err != nil {
		return fmt.Errorf("verify built image: %w", err)
	}

	fmt.Printf("Image built successfully: %s\n", cfg.ImageTag)
	return nil
}

func PullPostgres(ctx context.Context, version string) error {
	if version == "" {
		version = "16"
	}
	if err := postgres.ValidateVersion(version); err != nil {
		return err
	}

	cli, err := client.New(client.FromEnv)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	ref := fmt.Sprintf("postgres:%s", version)

	_, err = cli.ImageInspect(ctx, ref)
	if err == nil {
		fmt.Printf("Base image %s already present.\n", ref)
		return nil
	}
	if !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("inspect base image %s: %w", ref, err)
	}

	fmt.Printf("Pulling %s...\n", ref)
	resp, err := cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	if err := resp.Wait(ctx); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	fmt.Printf("Pulled %s\n", ref)
	return nil
}

func dockerfileContent(version, dumpFileName string) ([]byte, error) {
	if err := postgres.ValidateVersion(version); err != nil {
		return nil, err
	}
	copyArgs, err := json.Marshal([]string{dumpFileName, initDumpPath})
	if err != nil {
		return nil, fmt.Errorf("encode Dockerfile COPY arguments: %w", err)
	}
	return fmt.Appendf(nil, "FROM postgres:%s\nCOPY %s\n", version, copyArgs), nil
}

func writePortableBuildFiles(cfg BuildConfig) ([]byte, error) {
	info, err := os.Stat(cfg.DumpPath)
	if err != nil {
		return nil, fmt.Errorf("inspect dump file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("dump file is not a regular file: %s", cfg.DumpPath)
	}

	dumpFileName := filepath.Base(cfg.DumpPath)
	dockerfile, err := dockerfileContent(cfg.PgVersion, dumpFileName)
	if err != nil {
		return nil, err
	}

	outDir := filepath.Dir(cfg.DumpPath)
	path := filepath.Join(outDir, DockerfileName)
	// #nosec G306 -- the generated Dockerfile contains no credentials
	if err := os.WriteFile(path, dockerfile, 0o644); err != nil {
		return nil, fmt.Errorf("write portable Dockerfile: %w", err)
	}
	ignorePath := filepath.Join(outDir, DockerignoreName)
	// #nosec G306 -- the generated ignore file contains no credentials
	if err := os.WriteFile(ignorePath, []byte(dockerignoreContent), 0o644); err != nil {
		return nil, fmt.Errorf("write portable Docker ignore file: %w", err)
	}
	return dockerfile, nil
}

func streamBuildContext(dumpPath string, dockerfile []byte) (*io.PipeReader, func() error) {
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := writeBuildContext(writer, dumpPath, dockerfile)
		_ = writer.CloseWithError(err)
		done <- err
	}()
	return reader, func() error { return <-done }
}

func writeBuildContext(dst io.Writer, dumpPath string, dockerfile []byte) error {
	tw := tar.NewWriter(dst)
	if err := addBytesToTar(tw, DockerfileName, dockerfile); err != nil {
		_ = tw.Close()
		return fmt.Errorf("add Dockerfile to tar: %w", err)
	}
	if err := addFileToTar(tw, filepath.Base(dumpPath), dumpPath); err != nil {
		_ = tw.Close()
		return fmt.Errorf("add dump to tar: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}
	return nil
}

func addBytesToTar(tw *tar.Writer, name string, data []byte) error {
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

func addFileToTar(tw *tar.Writer, name, path string) error {
	// #nosec G304 -- path is the caller-selected dump file
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", path)
	}

	hdr := &tar.Header{
		Name: name,
		Mode: 0o600,
		Size: info.Size(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, file)
	return err
}
