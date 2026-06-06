# psqldump

Dump a remote PostgreSQL database and create a self-restoring Docker Compose setup without local PostgreSQL client tools.

## How it works

```mermaid
flowchart LR
    A[Remote PostgreSQL] -->|docker run pg_dump| B[dump.sql]
    B -->|Docker Go SDK| C[Custom Image<br/>postgres + dump.sql]
    C --> D[docker-compose.yml]
    D -->|docker compose up| E[Local PostgreSQL<br/>auto-restored]
```

1. **Dump**: runs `pg_dump` inside a version-matched `postgres:<major>-alpine` container.
2. **Build**: uses the Docker Go SDK to build a PostgreSQL image with the dump copied into `/docker-entrypoint-initdb.d/`.
3. **Compose**: generates a `docker-compose.yml` that starts the image and auto-restores the database on first startup.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with the daemon running
- Network access to the remote PostgreSQL server

## Install

Download the latest release for your platform from the [Releases](https://github.com/ralscha/psqldump/releases) page. 

## Usage

### Quick start

```bash
psqldump all \
  --host my-db.example.com \
  --dbname mydatabase \
  --user postgres \
  --password s3cret \
  --out ./out
```

This auto-detects the remote PostgreSQL version, dumps the database, builds a Docker image, and generates a `docker-compose.yml`.

### Step by step

```bash
# 1. Dump only
psqldump dump -H my-db.example.com -d mydatabase -U postgres -W s3cret -o ./out

# 2. Build Docker image from ./out/mydatabase.sql
psqldump build -H my-db.example.com -d mydatabase -U postgres -W s3cret -o ./out

# 3. Generate compose file
psqldump compose -d mydatabase -U postgres -W s3cret -o ./out
```

### Start the restored database

```bash
docker compose -f ./out/docker-compose.yml up -d
```

The database will be available on the external port from the generated compose file.

## Flags

The command line uses Go's built-in `flag` package. Short flags use one dash and long flags may use either one or two dashes, so `-host` and `--host` both work.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--host` | `-H` | `localhost` | Remote PostgreSQL host |
| `--port` | `-P` | `5432` | Remote PostgreSQL port |
| `--user` | `-U` | `postgres` | PostgreSQL user |
| `--password` | `-W` | `PGPASSWORD` env or empty | PostgreSQL password |
| `--dbname` | `-d` | required | Database name |
| `--out` | `-o` | `.` | Output directory for dump and compose file |
| `--external-port` | `-E` | value of `--port` | Host port in the generated compose file |
| `--pg-version` | | auto-detect | PostgreSQL major version for the dump client and Docker image |

If `--external-port` is omitted, the compose file uses the value from `--port`. If both are omitted, the compose file exposes `5432:5432`.

## Commands

| Command | Description |
|---|---|
| `dump` | Dump the remote database to a `.sql` file |
| `build` | Build a Docker image with the dump baked in |
| `compose` | Generate a `docker-compose.yml` |
| `all` | Run dump, build, and compose in sequence |


## License

MIT License
