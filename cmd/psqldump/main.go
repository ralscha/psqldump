package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"psqldump/internal/compose"
	"psqldump/internal/docker"
	"psqldump/internal/dump"
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
		return runDump(opts)
	case "build":
		return runBuild(opts)
	case "compose":
		return runCompose(opts)
	case "all":
		return runAll(opts)
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
	if opts.port <= 0 || opts.port > 65535 {
		return opts, fmt.Errorf("invalid PostgreSQL port: %d", opts.port)
	}
	if opts.externalPort <= 0 || opts.externalPort > 65535 {
		return opts, fmt.Errorf("invalid external port: %d", opts.externalPort)
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

func runDump(opts options) error {
	dumpPath, err := dump.Run(dump.Config{
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

func runBuild(opts options) error {
	ctx := context.Background()

	dumpPath := filepath.Join(opts.outDir, opts.dbName+".sql")
	if _, err := os.Stat(dumpPath); os.IsNotExist(err) {
		return fmt.Errorf("dump file not found: %s - run 'psqldump dump' first", dumpPath)
	}

	if opts.pgVer == "" {
		v, err := dump.ServerVersion(dump.Config{
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

	imageTag := fmt.Sprintf("psqldump-%s:latest", opts.dbName)

	if err := docker.PullPostgres(ctx, opts.pgVer); err != nil {
		return fmt.Errorf("pull postgres: %w", err)
	}

	if err := docker.BuildImage(ctx, docker.BuildConfig{
		DumpPath:  dumpPath,
		DBName:    opts.dbName,
		User:      opts.user,
		Password:  opts.password,
		ImageTag:  imageTag,
		PgVersion: opts.pgVer,
	}); err != nil {
		return fmt.Errorf("build image: %w", err)
	}

	fmt.Printf("Image %s built and ready.\n", imageTag)
	return nil
}

func runCompose(opts options) error {
	imageTag := fmt.Sprintf("psqldump-%s:latest", opts.dbName)
	composePath, err := compose.Generate(compose.Config{
		ImageName:    imageTag,
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
	fmt.Println("\nRun 'docker compose up -d' to start the restored database.")
	return nil
}

func runAll(opts options) error {
	ctx := context.Background()

	dumpCfg := dump.Config{
		Host:     opts.host,
		Port:     opts.port,
		User:     opts.user,
		Password: opts.password,
		DBName:   opts.dbName,
		OutDir:   opts.outDir,
	}

	if opts.pgVer == "" {
		v, err := dump.ServerVersion(dumpCfg)
		if err != nil {
			return fmt.Errorf("auto-detect pg version: %w", err)
		}
		opts.pgVer = v
	}

	dumpCfg.PgVersion = opts.pgVer
	dumpPath, err := dump.Run(dumpCfg)
	if err != nil {
		return fmt.Errorf("dump: %w", err)
	}

	imageTag := fmt.Sprintf("psqldump-%s:latest", opts.dbName)
	if err := docker.PullPostgres(ctx, opts.pgVer); err != nil {
		return fmt.Errorf("pull postgres: %w", err)
	}
	if err := docker.BuildImage(ctx, docker.BuildConfig{
		DumpPath:  dumpPath,
		DBName:    opts.dbName,
		User:      opts.user,
		Password:  opts.password,
		ImageTag:  imageTag,
		PgVersion: opts.pgVer,
	}); err != nil {
		return fmt.Errorf("build image: %w", err)
	}

	composePath, err := compose.Generate(compose.Config{
		ImageName:    imageTag,
		DBName:       opts.dbName,
		User:         opts.user,
		Password:     opts.password,
		ExternalPort: opts.externalPort,
		OutDir:       opts.outDir,
	})
	if err != nil {
		return fmt.Errorf("compose: %w", err)
	}

	fmt.Printf("\nAll done! Dump, image, and compose file at: %s\nRun 'docker compose -f %s up -d' to start.\n",
		opts.outDir, composePath)
	return nil
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
