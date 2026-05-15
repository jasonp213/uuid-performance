package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"

	_ "github.com/go-sql-driver/mysql"
)

func runMySQL(ctx context.Context, cfg Config, scenarios []string) ([]InsertSample, []PaginationSample, error) {
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("mysql open: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	if err := db.PingContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("mysql ping: %w", err)
	}

	var inserts []InsertSample
	var pagings []PaginationSample

	for _, scenario := range scenarios {
		log.Printf("[%s] starting", scenario)
		if !cfg.SkipInsert {
			if cfg.Truncate {
				if _, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s", scenario)); err != nil {
					return nil, nil, fmt.Errorf("[%s] truncate: %w", scenario, err)
				}
			}
			samples, err := mysqlInsert(ctx, db, scenario, cfg)
			if err != nil {
				return nil, nil, fmt.Errorf("[%s] insert: %w", scenario, err)
			}
			inserts = append(inserts, samples...)
		}
		if !cfg.SkipPagination {
			samples, err := mysqlPaginate(ctx, db, scenario, cfg)
			if err != nil {
				return nil, nil, fmt.Errorf("[%s] paginate: %w", scenario, err)
			}
			pagings = append(pagings, samples...)
		}
	}
	return inserts, pagings, nil
}

func mysqlInsert(ctx context.Context, db *sql.DB, scenario string, cfg Config) ([]InsertSample, error) {
	chunks := cfg.Rows / cfg.Chunk
	batchesPerChunk := cfg.Chunk / cfg.Batch
	samples := make([]InsertSample, 0, chunks)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	cumRows := 0
	for c := 0; c < chunks; c++ {
		chunkStart := time.Now()
		for b := 0; b < batchesPerChunk; b++ {
			if err := mysqlInsertBatch(ctx, db, scenario, cfg.Batch, rng); err != nil {
				return nil, err
			}
		}
		chunkDur := time.Since(chunkStart)
		cumRows += cfg.Chunk
		total, idx, err := mysqlRelationSize(ctx, db, scenario)
		if err != nil {
			return nil, err
		}
		samples = append(samples, InsertSample{
			Scenario:   scenario,
			CumRows:    cumRows,
			ChunkRows:  cfg.Chunk,
			ChunkMS:    chunkDur.Milliseconds(),
			RowsPerSec: float64(cfg.Chunk) / chunkDur.Seconds(),
			TotalBytes: total,
			IndexBytes: idx,
		})
		log.Printf("[%s] cum=%d chunk=%dms (%.0f rows/s) total=%s",
			scenario, cumRows, chunkDur.Milliseconds(), float64(cfg.Chunk)/chunkDur.Seconds(), humanBytes(total))
	}
	return samples, nil
}

func mysqlInsertBatch(ctx context.Context, db *sql.DB, scenario string, n int, rng *rand.Rand) error {
	var sb strings.Builder
	args := make([]any, 0, n*4)

	switch scenario {
	case "mysql_autoinc":
		sb.WriteString("INSERT INTO mysql_autoinc (user_id, amount, status) VALUES ")
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?)")
			args = append(args, rng.Int63n(10_000_000), randAmount(rng), randStatus(rng))
		}
	case "mysql_uuidv4_app", "mysql_uuidv7_app":
		table := scenario
		sb.WriteString(fmt.Sprintf("INSERT INTO %s (id, user_id, amount, status) VALUES ", table))
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?)")
			var id uuid.UUID
			if scenario == "mysql_uuidv4_app" {
				id = uuid.New()
			} else {
				id = mustUUIDv7()
			}
			args = append(args, id[:], rng.Int63n(10_000_000), randAmount(rng), randStatus(rng))
		}
	default:
		return fmt.Errorf("unknown mysql scenario %q", scenario)
	}

	_, err := db.ExecContext(ctx, sb.String(), args...)
	return err
}

func mysqlRelationSize(ctx context.Context, db *sql.DB, table string) (int64, int64, error) {
	// InnoDB stats are sampled lazily; force a refresh.
	if _, err := db.ExecContext(ctx, "ANALYZE NO_WRITE_TO_BINLOG TABLE "+table); err != nil {
		return 0, 0, err
	}
	var dataLen, idxLen sql.NullInt64
	row := db.QueryRowContext(ctx,
		"SELECT data_length, index_length FROM information_schema.TABLES WHERE table_schema=DATABASE() AND table_name=?", table)
	if err := row.Scan(&dataLen, &idxLen); err != nil {
		return 0, 0, err
	}
	total := dataLen.Int64 + idxLen.Int64
	// Fall back to actual tablespace file size (more accurate than sampled stats).
	var fileSize sql.NullInt64
	row = db.QueryRowContext(ctx,
		"SELECT file_size FROM information_schema.INNODB_TABLESPACES WHERE name = CONCAT(DATABASE(), '/', ?)", table)
	_ = row.Scan(&fileSize)
	if fileSize.Valid && fileSize.Int64 > total {
		total = fileSize.Int64
	}
	return total, idxLen.Int64, nil
}

func mysqlPaginate(ctx context.Context, db *sql.DB, scenario string, cfg Config) ([]PaginationSample, error) {
	deepOffset := cfg.Rows - cfg.PaginationN*5
	if deepOffset < 0 {
		deepOffset = cfg.Rows / 2
	}
	out := []PaginationSample{}

	measure := func(method, sqlText string, args ...any) error {
		start := time.Now()
		rows, err := db.QueryContext(ctx, sqlText, args...)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			count++
		}
		rows.Close()
		if rows.Err() != nil {
			return rows.Err()
		}
		ms := float64(time.Since(start).Microseconds()) / 1000.0
		out = append(out, PaginationSample{
			Scenario: scenario,
			Method:   method,
			Offset:   deepOffset,
			Limit:    cfg.PaginationN,
			Millis:   ms,
		})
		log.Printf("[%s] paginate %s offset=%d limit=%d -> %.2fms (%d rows)", scenario, method, deepOffset, cfg.PaginationN, ms, count)
		return nil
	}

	switch scenario {
	case "mysql_autoinc":
		if err := measure("offset",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM mysql_autoinc ORDER BY id LIMIT %d OFFSET %d", cfg.PaginationN, deepOffset)); err != nil {
			return nil, err
		}
		var pivot time.Time
		row := db.QueryRowContext(ctx,
			fmt.Sprintf("SELECT created_at FROM mysql_autoinc ORDER BY created_at LIMIT 1 OFFSET %d", deepOffset))
		if err := row.Scan(&pivot); err != nil {
			return nil, fmt.Errorf("mysql_autoinc pivot: %w", err)
		}
		if err := measure("cursor_created_at",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM mysql_autoinc WHERE created_at >= ? ORDER BY created_at LIMIT %d", cfg.PaginationN),
			pivot); err != nil {
			return nil, err
		}
	case "mysql_uuidv4_app":
		if err := measure("offset",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM mysql_uuidv4_app ORDER BY id LIMIT %d OFFSET %d", cfg.PaginationN, deepOffset)); err != nil {
			return nil, err
		}
	case "mysql_uuidv7_app":
		if err := measure("offset",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM mysql_uuidv7_app ORDER BY id LIMIT %d OFFSET %d", cfg.PaginationN, deepOffset)); err != nil {
			return nil, err
		}
		var pivot []byte
		row := db.QueryRowContext(ctx,
			fmt.Sprintf("SELECT id FROM mysql_uuidv7_app ORDER BY id LIMIT 1 OFFSET %d", deepOffset))
		if err := row.Scan(&pivot); err != nil {
			return nil, fmt.Errorf("mysql_uuidv7_app pivot: %w", err)
		}
		if err := measure("cursor_pk",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM mysql_uuidv7_app WHERE id >= ? ORDER BY id LIMIT %d", cfg.PaginationN),
			pivot); err != nil {
			return nil, err
		}
	}
	return out, nil
}
