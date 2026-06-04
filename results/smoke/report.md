# UUID Primary Key Benchmark

_Generated: 2026-06-04T17:25:33+08:00_

## Configuration

- Rows per scenario: **1000000**
- Batch (rows per INSERT statement): **1000**
- Chunk (rows per measurement checkpoint): **100000**
- Pagination limit: **100**
- Scenarios: pg_serial, pg_uuidv4_app, pg_uuidv7_app, mysql_autoinc, mysql_autoinc_uuidv4_uk, mysql_uuidv4_app, mysql_uuidv7_app

Engines: PostgreSQL 17, MySQL 8.0

Neither PG 17 nor MySQL 8 has a native `uuidv7()` generator, so UUIDv7 values are generated in the Go app for both engines.

## INSERT latency per chunk (ms)

| cum_rows | mysql_autoinc | mysql_uuidv4_app | mysql_uuidv7_app | pg_serial | pg_uuidv4_app | pg_uuidv7_app | pg_uuidv7_db |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 25000 | 203 | 211 | 211 | 87 | 110 | 97 | 108 |
| 50000 | 99 | 127 | 91 | 82 | 102 | 88 | 113 |

## INSERT throughput per chunk (rows/sec)

| cum_rows | mysql_autoinc | mysql_uuidv4_app | mysql_uuidv7_app | pg_serial | pg_uuidv4_app | pg_uuidv7_app | pg_uuidv7_db |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 25000 | 122775 | 118088 | 118479 | 285681 | 227168 | 256381 | 229865 |
| 50000 | 251139 | 196456 | 272226 | 302533 | 242758 | 282218 | 219968 |

## Pagination at deep offset (ms)

| scenario | method | offset | limit | millis |
|---|---|---:|---:|---:|
| mysql_autoinc | cursor_created_at | 49750 | 50 | 0.36 |
| mysql_autoinc | offset | 49750 | 50 | 4.79 |
| mysql_uuidv4_app | offset | 49750 | 50 | 4.86 |
| mysql_uuidv7_app | cursor_pk | 49750 | 50 | 0.22 |
| mysql_uuidv7_app | offset | 49750 | 50 | 4.92 |
| pg_serial | cursor_created_at | 49750 | 50 | 0.26 |
| pg_serial | offset | 49750 | 50 | 2.26 |
| pg_uuidv4_app | offset | 49750 | 50 | 8.16 |
| pg_uuidv7_app | cursor_pk | 49750 | 50 | 0.24 |
| pg_uuidv7_app | offset | 49750 | 50 | 2.33 |
| pg_uuidv7_db | cursor_pk | 49750 | 50 | 0.22 |
| pg_uuidv7_db | offset | 49750 | 50 | 4.35 |

## Storage after each chunk (MB)

| cum_rows | mysql_autoinc total | mysql_autoinc idx | mysql_uuidv4_app total | mysql_uuidv4_app idx | mysql_uuidv7_app total | mysql_uuidv7_app idx | pg_serial total | pg_serial idx | pg_uuidv4_app total | pg_uuidv4_app idx | pg_uuidv7_app total | pg_uuidv7_app idx | pg_uuidv7_db total | pg_uuidv7_db idx |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 25000 | 2.0 | 0.5 | 2.5 | 0.0 | 2.5 | 0.0 | 2.9 | 1.1 | 3.1 | 1.1 | 2.8 | 0.8 | 3.0 | 1.0 |
| 50000 | 5.0 | 1.5 | 5.5 | 0.0 | 3.5 | 0.0 | 5.7 | 2.2 | 6.1 | 2.1 | 5.5 | 1.5 | 5.9 | 2.0 |

## Quick takeaways

**Insert degradation (last chunk ms / first chunk ms):**

| scenario | first | last | ratio |
|---|---:|---:|---:|
| mysql_autoinc | 203 | 99 | 0.49x |
| mysql_uuidv4_app | 211 | 127 | 0.60x |
| mysql_uuidv7_app | 211 | 91 | 0.43x |
| pg_serial | 87 | 82 | 0.94x |
| pg_uuidv4_app | 110 | 102 | 0.93x |
| pg_uuidv7_app | 97 | 88 | 0.91x |
| pg_uuidv7_db | 108 | 113 | 1.05x |

**Best pagination method per scenario:**

| scenario | best method | millis |
|---|---|---:|
| mysql_autoinc | cursor_created_at | 0.36 |
| mysql_uuidv4_app | offset | 4.86 |
| mysql_uuidv7_app | cursor_pk | 0.22 |
| pg_serial | cursor_created_at | 0.26 |
| pg_uuidv4_app | offset | 8.16 |
| pg_uuidv7_app | cursor_pk | 0.24 |
| pg_uuidv7_db | cursor_pk | 0.22 |

