# UUID Primary Key Benchmark — PostgreSQL 17 vs MySQL 8

Reproduces the lab from [Database Primary Keys: AUTO_INCREMENT, UUID, and UUIDv7](https://blog.kalan.dev/devnote/uuidv7-primary-key) and extends it with:

- Both PostgreSQL 17 and MySQL 8 (via `docker-compose`)
- A Go benchmark driver with tunable knobs (rows, batch size, chunk size, scenarios, pagination)
- Output: Markdown report + raw CSVs

Neither PG 17 nor MySQL 8 ships a native `uuidv7()` generator, so UUIDv7 values are produced in the Go app (`google/uuid` v1.6+).

## Scenarios

| name                    | engine     | PK strategy                                                      |
|-------------------------|------------|------------------------------------------------------------------|
| pg_serial               | PostgreSQL | `BIGSERIAL`                                                      |
| pg_uuidv4_app           | PostgreSQL | `UUID` v4 generated in Go                                        |
| pg_uuidv7_app           | PostgreSQL | `UUID` v7 generated in Go                                        |
| mysql_autoinc           | MySQL      | `BIGINT AUTO_INCREMENT`                                          |
| mysql_autoinc_uuidv4_uk | MySQL      | `BIGINT AUTO_INCREMENT` PK + `BINARY(16)` UUID v4 unique key     |
| mysql_uuidv4_app        | MySQL      | `BINARY(16)` UUID v4 generated in Go                             |
| mysql_uuidv7_app        | MySQL      | `BINARY(16)` UUID v7 generated in Go                             |

`mysql_autoinc_uuidv4_uk` is a common real-world pattern: keep InnoDB's clustered auto-increment PK for write locality, but expose a random UUID as a stable external identifier enforced by a secondary unique index. The benchmark isolates the write overhead of maintaining that extra index.

## Quick start

```bash
docker-compose up -d                # PG 17 on :55432, MySQL 8 on :53306
go build -o bin/bench .
./bin/bench                          # default: 1,000,000 rows × 7 scenarios
open results/report.md
```

## CLI flags

| flag                | default                                                                 | meaning |
|---------------------|-------------------------------------------------------------------------|---------|
| `-rows`             | `1000000`                                                               | total rows inserted per scenario |
| `-batch`            | `1000`                                                                  | rows per multi-value `INSERT` statement |
| `-chunk`            | `100000`                                                                | rows per measurement checkpoint (`rows` must be divisible by this) |
| `-scenarios`        | `all`                                                                   | `all` / `pg` / `mysql` / comma-list (e.g. `pg_serial,pg_uuidv7_app`) |
| `-pg-dsn`           | `postgres://bench:bench@127.0.0.1:55432/bench?sslmode=disable`          | PostgreSQL DSN |
| `-mysql-dsn`        | `bench:bench@tcp(127.0.0.1:53306)/bench?parseTime=true&multiStatements=true` | MySQL DSN |
| `-output-dir`       | `results`                                                               | where `report.md`, `inserts.csv`, `pagination.csv` are written |
| `-pagination-limit` | `100`                                                                   | `LIMIT` for the pagination probe (probe is at `rows - limit*5` offset) |
| `-skip-insert`      | `false`                                                                 | skip the insert phase (use with `-truncate=false`) |
| `-skip-pagination`  | `false`                                                                 | skip the pagination probe |
| `-truncate`         | `true`                                                                  | `TRUNCATE` each table before inserting |
| `-append`           | `false`                                                                 | merge new results into existing CSVs instead of overwriting |

## Reproducing article-scale runs

The original article runs 20M rows per scenario. That's slow (~hours on a laptop). A few presets:

```bash
# Smoke: ~5s end-to-end
./bin/bench -rows 50000 -chunk 25000 -output-dir results/smoke

# Quick lab: ~1-3 min end-to-end
./bin/bench -rows 1000000 -chunk 100000 -output-dir results/1m

# Article-scale: tens of minutes to hours; expects UUIDv4 insert latency to climb sharply.
./bin/bench -rows 20000000 -chunk 1000000 -output-dir results/20m
```

PostgreSQL-only / MySQL-only:

```bash
./bin/bench -scenarios pg    -rows 5000000 -chunk 500000 -output-dir results/pg-5m
./bin/bench -scenarios mysql -rows 5000000 -chunk 500000 -output-dir results/mysql-5m
```

### Adding a new scenario to an existing run

Use `-append` to inject a single scenario's results into an already-populated output directory without re-running the others:

```bash
# e.g. add mysql_autoinc_uuidv4_uk to a finished 20M run
./bin/bench -rows 20000000 -chunk 1000000 \
    -scenarios mysql_autoinc_uuidv4_uk \
    -append -output-dir results/20m
```

`-append` reads back the existing CSVs, drops any rows for the scenarios being re-run (to avoid duplicates), appends the new rows, and regenerates `report.md` with the full combined dataset.

> **Note:** if the table doesn't exist yet (e.g. after adding a new scenario to a running container), create it manually rather than doing a full `reset` which would destroy existing data:
> ```bash
> docker exec -i uuidlab-mysql8 mysql -ubench -pbench bench < sql/mysql-init.sql
> ```
> Replace `mysql-init.sql` with a one-off `CREATE TABLE` statement if you don't want to recreate all tables.

## What the report contains

- **INSERT latency / throughput per chunk** — shows degradation as the table grows. UUIDv4 should diverge upward at scale; serial and UUIDv7 stay roughly flat.
- **Storage after each chunk** — table + index size (`pg_total_relation_size` / `INNODB_TABLESPACES.FILE_SIZE`).
- **Pagination at deep offset** — three probes per applicable scenario:
  - `offset`: `ORDER BY id LIMIT N OFFSET (rows - 5N)` — scales O(offset)
  - `cursor_created_at`: `WHERE created_at >= $pivot ORDER BY created_at LIMIT N` (only for `*_serial` / `*_autoinc`)
  - `cursor_pk`: `WHERE id >= $pivot ORDER BY id LIMIT N` (only for UUIDv7 tables, since the PK is time-ordered)
- **Quick takeaways**: degradation ratio (`last chunk / first chunk`) and the fastest pagination method per scenario.

## Container tuning notes

`docker-compose.yml` sets a few non-default options for both engines so the benchmark isn't bottlenecked by fsync/commit settings:

- PostgreSQL: `synchronous_commit=off`, larger `shared_buffers`, `wal_compression=on`.
- MySQL: `innodb_flush_log_at_trx_commit=2`, larger `innodb_buffer_pool_size`, binlog sync off.

Both are realistic "tuned for throughput" defaults but not "force durability" — that's fine for relative comparisons, do not copy verbatim to production.

## Cleanup

```bash
docker-compose down -v   # removes volumes
```
