package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"psqldump/internal/compose"
	"psqldump/internal/docker"
	"psqldump/internal/dump"
	"psqldump/internal/postgres"
)

const defaultPostgresPort = 5432

type options struct {
	host         string
	port         int
	user         string
	password     string
	dbName       string
	outDir       string
	pgVer        string
	externalPort int
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runContext(ctx, args)
}

func runContext(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return errors.New("missing command")
	}

	command := args[0]
	if command == "-h" || command == "--help" || command == "help" {
		printUsage(os.Stdout)
		return nil
	}

	opts, err := parseOptions(command, args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	switch command {
	case "dump":
		return runDump(ctx, opts)
	case "build":
		return runBuild(ctx, opts)
	case "compose":
		return runCompose(opts)
	case "all":
		return runAll(ctx, opts)
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", command)
	}
}

func parseOptions(command string, args []string) (options, error) {
	opts := options{
		host:         "localhost",
		port:         defaultPostgresPort,
		user:         "postgres",
		password:     os.Getenv("PGPASSWORD"),
		outDir:       ".",
		externalPort: defaultPostgresPort,
	}

	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		printCommandUsage(os.Stderr, command)
	}
	addStringFlag(fs, &opts.host, "host", "H", opts.host, "PostgreSQL host")
	addIntFlag(fs, &opts.port, "port", "P", opts.port, "PostgreSQL port")
	addStringFlag(fs, &opts.user, "user", "U", opts.user, "PostgreSQL user")
	addStringFlag(fs, &opts.password, "password", "W", opts.password, "PostgreSQL password (or set PGPASSWORD)")
	addStringFlag(fs, &opts.dbName, "dbname", "d", "", "Database name (required)")
	addStringFlag(fs, &opts.outDir, "out", "o", opts.outDir, "Output directory")
	fs.StringVar(&opts.pgVer, "pg-version", "", "PostgreSQL major version (e.g. 16). Empty = auto-detect from server")
	addIntFlag(fs, &opts.externalPort, "external-port", "E", opts.externalPort, "Host port for the generated compose file")

	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	externalPortSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "external-port" || f.Name == "E" {
			externalPortSet = true
		}
	})
	if !externalPortSet {
		opts.externalPort = opts.port
	}
	if opts.dbName == "" {
		return opts, errors.New("missing required flag: -d/--dbname")
	}
	dumpFileName := opts.dbName + ".sql"
	if !filepath.IsLocal(dumpFileName) || filepath.Base(dumpFileName) != dumpFileName {
		return opts, fmt.Errorf("database name %q cannot be used as a portable dump filename", opts.dbName)
	}
	if opts.port <= 0 || opts.port > 65535 {
		return opts, fmt.Errorf("invalid PostgreSQL port: %d", opts.port)
	}
	if opts.externalPort <= 0 || opts.externalPort > 65535 {
		return opts, fmt.Errorf("invalid external port: %d", opts.externalPort)
	}
	if opts.pgVer != "" {
		if err := postgres.ValidateVersion(opts.pgVer); err != nil {
			return opts, err
		}
	}
	return opts, nil
}

func addStringFlag(fs *flag.FlagSet, target *string, long, short, value, usage string) {
	fs.StringVar(target, long, value, usage)
	fs.StringVar(target, short, value, usage)
}

func addIntFlag(fs *flag.FlagSet, target *int, long, short string, value int, usage string) {
	fs.IntVar(target, long, value, usage)
	fs.IntVar(target, short, value, usage)
}

func runDump(ctx context.Context, opts options) error {
	dumpPath, err := dump.RunContext(ctx, dump.Config{
		Host:      opts.host,
		Port:      opts.port,
		User:      opts.user,
		Password:  opts.password,
		DBName:    opts.dbName,
		OutDir:    opts.outDir,
		PgVersion: opts.pgVer,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Dump saved to: %s\n", dumpPath)
	return nil
}

func runBuild(ctx context.Context, opts options) error {
	dumpPath := filepath.Join(opts.outDir, opts.dbName+".sql")
	if _, err := os.Stat(dumpPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("dump file not found: %s - run 'psqldump dump' first", dumpPath)
		}
		return fmt.Errorf("inspect dump file %s: %w", dumpPath, err)
	}

	if opts.pgVer == "" {
		v, err := dump.ServerVersionContext(ctx, dump.Config{
			Host:     opts.host,
			Port:     opts.port,
			User:     opts.user,
			Password: opts.password,
			DBName:   opts.dbName,
			OutDir:   opts.outDir,
		})
		if err != nil {
			return fmt.Errorf("auto-detect pg version: %w", err)
		}
		opts.pgVer = v
	}

	imageTag := imageTagForDatabase(opts.dbName)

	if err := docker.PullPostgres(ctx, opts.pgVer); err != nil {
		return fmt.Errorf("pull postgres: %w", err)
	}

	if err := docker.BuildImage(ctx, docker.BuildConfig{
		DumpPath:  dumpPath,
		ImageTag:  imageTag,
		PgVersion: opts.pgVer,
	}); err != nil {
		return fmt.Errorf("build image: %w", err)
	}

	fmt.Printf("Image %s built and ready.\n", imageTag)
	return nil
}

func runCompose(opts options) error {
	imageTag := imageTagForDatabase(opts.dbName)
	composePath, err := compose.Generate(compose.Config{
		ImageName:    imageTag,
		Dockerfile:   docker.DockerfileName,
		DBName:       opts.dbName,
		User:         opts.user,
		Password:     opts.password,
		ExternalPort: opts.externalPort,
		OutDir:       opts.outDir,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Compose file: %s\n", composePath)
	fmt.Println("\nRun 'docker compose up -d --build' to start the restored database.")
	return nil
}

func runAll(ctx context.Context, opts options) error {
	dumpCfg := dump.Config{
		Host:     opts.host,
		Port:     opts.port,
		User:     opts.user,
		Password: opts.password,
		DBName:   opts.dbName,
		OutDir:   opts.outDir,
	}

	if opts.pgVer == "" {
		v, err := dump.ServerVersionContext(ctx, dumpCfg)
		if err != nil {
			return fmt.Errorf("auto-detect pg version: %w", err)
		}
		opts.pgVer = v
	}

	dumpCfg.PgVersion = opts.pgVer
	dumpPath, err := dump.RunContext(ctx, dumpCfg)
	if err != nil {
		return fmt.Errorf("dump: %w", err)
	}

	imageTag := imageTagForDatabase(opts.dbName)
	if err := docker.PullPostgres(ctx, opts.pgVer); err != nil {
		return fmt.Errorf("pull postgres: %w", err)
	}
	if err := docker.BuildImage(ctx, docker.BuildConfig{
		DumpPath:  dumpPath,
		ImageTag:  imageTag,
		PgVersion: opts.pgVer,
	}); err != nil {
		return fmt.Errorf("build image: %w", err)
	}

	composePath, err := compose.Generate(compose.Config{
		ImageName:    imageTag,
		Dockerfile:   docker.DockerfileName,
		DBName:       opts.dbName,
		User:         opts.user,
		Password:     opts.password,
		ExternalPort: opts.externalPort,
		OutDir:       opts.outDir,
	})
	if err != nil {
		return fmt.Errorf("compose: %w", err)
	}

	fmt.Printf("\nAll done! Portable dump, build recipe, and compose file are at: %s\nRun 'docker compose -f %s up -d --build' to start.\n",
		opts.outDir, composePath)
	return nil
}

func imageTagForDatabase(dbName string) string {
	lowerName := strings.ToLower(dbName)
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
	if len(normalized) > 160 {
		normalized = strings.TrimRight(normalized[:160], "-_.")
	}
	if normalized != dbName {
		hash := sha256.Sum256([]byte(dbName))
		normalized = fmt.Sprintf("%s-%x", normalized, hash[:4])
	}
	return "psqldump-" + normalized + ":latest"
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "psqldump dumps a remote PostgreSQL DB and creates a self-restoring Docker Compose setup.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  psqldump <command> [flags]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  dump      Dump the remote database to a local SQL file")
	_, _ = fmt.Fprintln(w, "  build     Build a Docker image with the dump baked in")
	_, _ = fmt.Fprintln(w, "  compose   Generate a docker-compose.yml for the restored database")
	_, _ = fmt.Fprintln(w, "  all       Run dump, build, and compose in sequence")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Run 'psqldump <command> --help' for command flags.")
}

func printCommandUsage(w io.Writer, command string) {
	_, _ = fmt.Fprintf(w, "Usage: psqldump %s [flags]\n\n", command)
	_, _ = fmt.Fprintln(w, "Flags:")
	_, _ = fmt.Fprintln(w, "  -H, --host string             PostgreSQL host (default \"localhost\")")
	_, _ = fmt.Fprintln(w, "  -P, --port int                PostgreSQL port (default 5432)")
	_, _ = fmt.Fprintln(w, "  -U, --user string             PostgreSQL user (default \"postgres\")")
	_, _ = fmt.Fprintln(w, "  -W, --password string         PostgreSQL password (or set PGPASSWORD)")
	_, _ = fmt.Fprintln(w, "  -d, --dbname string           Database name (required)")
	_, _ = fmt.Fprintln(w, "  -o, --out string              Output directory (default \".\")")
	_, _ = fmt.Fprintln(w, "  -E, --external-port int       Host port for the generated compose file (default 5432)")
	_, _ = fmt.Fprintln(w, "      --pg-version string       PostgreSQL major version (e.g. 16). Empty = auto-detect from server")
}
