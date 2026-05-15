# UUID Primary Key Benchmark

_Generated: 2026-05-15T14:14:21+08:00_

## Configuration

- Rows per scenario: **1000000**
- Batch (rows per INSERT statement): **1000**
- Chunk (rows per measurement checkpoint): **100000**
- Pagination limit: **100**
- Scenarios: pg_serial, pg_uuidv4_app, pg_uuidv7_app, mysql_autoinc, mysql_uuidv4_app, mysql_uuidv7_app

Engines: PostgreSQL 17, MySQL 8.0

Neither PG 17 nor MySQL 8 has a native `uuidv7()` generator, so UUIDv7 values are generated in the Go app for both engines.

## INSERT latency per chunk (ms)

| cum_rows | mysql_autoinc | mysql_uuidv4_app | mysql_uuidv7_app | pg_serial | pg_uuidv4_app | pg_uuidv7_app |
|---:|---:|---:|---:|---:|---:|---:|
| 100000 | 728 | 714 | 561 | 470 | 584 | 545 |
| 200000 | 516 | 699 | 528 | 414 | 626 | 568 |
| 300000 | 529 | 667 | 514 | 407 | 660 | 563 |
| 400000 | 491 | 608 | 540 | 432 | 665 | 646 |
| 500000 | 458 | 640 | 489 | 418 | 642 | 542 |
| 600000 | 430 | 691 | 543 | 403 | 597 | 581 |
| 700000 | 399 | 723 | 523 | 376 | 706 | 597 |
| 800000 | 420 | 677 | 552 | 352 | 712 | 597 |
| 900000 | 398 | 683 | 465 | 303 | 718 | 600 |
| 1000000 | 395 | 690 | 518 | 343 | 729 | 558 |

## INSERT throughput per chunk (rows/sec)

| cum_rows | mysql_autoinc | mysql_uuidv4_app | mysql_uuidv7_app | pg_serial | pg_uuidv4_app | pg_uuidv7_app |
|---:|---:|---:|---:|---:|---:|---:|
| 100000 | 137281 | 140033 | 177944 | 212558 | 170986 | 183282 |
| 200000 | 193694 | 142900 | 189388 | 241426 | 159496 | 176032 |
| 300000 | 188714 | 149907 | 194534 | 245123 | 151326 | 177581 |
| 400000 | 203309 | 164364 | 184851 | 231374 | 150367 | 154770 |
| 500000 | 217899 | 156205 | 204479 | 239031 | 155560 | 184194 |
| 600000 | 232488 | 144607 | 184052 | 247545 | 167406 | 171895 |
| 700000 | 250370 | 138224 | 191084 | 265690 | 141522 | 167281 |
| 800000 | 237791 | 147596 | 180985 | 283748 | 140375 | 167417 |
| 900000 | 251137 | 146281 | 214902 | 329174 | 139248 | 166486 |
| 1000000 | 252582 | 144852 | 192851 | 290885 | 137118 | 179168 |

## Pagination at deep offset (ms)

| scenario | method | offset | limit | millis |
|---|---|---:|---:|---:|
| mysql_autoinc | cursor_created_at | 999500 | 100 | 0.70 |
| mysql_autoinc | offset | 999500 | 100 | 97.93 |
| mysql_uuidv4_app | offset | 999500 | 100 | 101.79 |
| mysql_uuidv7_app | cursor_pk | 999500 | 100 | 0.53 |
| mysql_uuidv7_app | offset | 999500 | 100 | 98.89 |
| pg_serial | cursor_created_at | 999500 | 100 | 0.49 |
| pg_serial | offset | 999500 | 100 | 47.27 |
| pg_uuidv4_app | offset | 999500 | 100 | 379.10 |
| pg_uuidv7_app | cursor_pk | 999500 | 100 | 2.53 |
| pg_uuidv7_app | offset | 999500 | 100 | 47.22 |

## Storage after each chunk (MB)

| cum_rows | mysql_autoinc total | mysql_autoinc idx | mysql_uuidv4_app total | mysql_uuidv4_app idx | mysql_uuidv7_app total | mysql_uuidv7_app idx | pg_serial total | pg_serial idx | pg_uuidv4_app total | pg_uuidv4_app idx | pg_uuidv7_app total | pg_uuidv7_app idx |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 100000 | 9.0 | 2.5 | 10.5 | 0.0 | 6.5 | 0.0 | 11.4 | 4.3 | 12.2 | 4.3 | 10.9 | 3.0 |
| 200000 | 16.0 | 4.5 | 20.5 | 0.0 | 13.5 | 0.0 | 22.8 | 8.5 | 24.2 | 8.5 | 21.8 | 6.0 |
| 300000 | 24.1 | 6.5 | 32.6 | 0.0 | 19.6 | 0.0 | 34.0 | 12.7 | 34.6 | 11.0 | 32.7 | 9.0 |
| 400000 | 31.1 | 8.5 | 41.6 | 0.0 | 26.6 | 0.0 | 45.3 | 16.9 | 48.2 | 16.7 | 43.6 | 12.1 |
| 500000 | 38.1 | 9.5 | 51.6 | 0.0 | 32.6 | 0.0 | 56.7 | 21.2 | 58.3 | 18.9 | 54.4 | 15.1 |
| 600000 | 46.1 | 11.5 | 63.6 | 0.0 | 39.6 | 0.0 | 67.9 | 25.4 | 69.3 | 22.1 | 65.3 | 18.1 |
| 700000 | 53.1 | 13.5 | 74.6 | 0.0 | 45.6 | 0.0 | 79.2 | 29.6 | 83.1 | 27.9 | 76.2 | 21.1 |
| 800000 | 61.1 | 15.5 | 82.6 | 0.0 | 51.6 | 0.0 | 90.5 | 33.7 | 96.2 | 33.2 | 87.1 | 24.1 |
| 900000 | 68.1 | 17.6 | 91.7 | 0.0 | 58.6 | 0.0 | 101.8 | 38.0 | 107.0 | 36.2 | 97.9 | 27.1 |
| 1000000 | 76.2 | 19.6 | 102.8 | 0.0 | 64.6 | 0.0 | 113.1 | 42.2 | 116.8 | 38.0 | 108.8 | 30.1 |

## Quick takeaways

**Insert degradation (last chunk ms / first chunk ms):**

| scenario | first | last | ratio |
|---|---:|---:|---:|
| mysql_autoinc | 728 | 395 | 0.54x |
| mysql_uuidv4_app | 714 | 690 | 0.97x |
| mysql_uuidv7_app | 561 | 518 | 0.92x |
| pg_serial | 470 | 343 | 0.73x |
| pg_uuidv4_app | 584 | 729 | 1.25x |
| pg_uuidv7_app | 545 | 558 | 1.02x |

**Best pagination method per scenario:**

| scenario | best method | millis |
|---|---|---:|
| mysql_autoinc | cursor_created_at | 0.70 |
| mysql_uuidv4_app | offset | 101.79 |
| mysql_uuidv7_app | cursor_pk | 0.53 |
| pg_serial | cursor_created_at | 0.49 |
| pg_uuidv4_app | offset | 379.10 |
| pg_uuidv7_app | cursor_pk | 2.53 |

