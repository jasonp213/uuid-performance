package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runPostgres(ctx context.Context, cfg Config, scenarios []string) ([]InsertSample, []PaginationSample, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.PgDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("pgx parse config: %w", err)
	}
	// Avoid prepared-statement cache (different batch sizes → many cached stmts)
	pcfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, nil, fmt.Errorf("pgx pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return nil, nil, fmt.Errorf("pgx ping: %w", err)
	}

	var inserts []InsertSample
	var pagings []PaginationSample

	for _, scenario := range scenarios {
		log.Printf("[%s] starting", scenario)
		if !cfg.SkipInsert {
			if cfg.Truncate {
				if _, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY", scenario)); err != nil {
					return nil, nil, fmt.Errorf("[%s] truncate: %w", scenario, err)
				}
			}
			samples, err := pgInsert(ctx, pool, scenario, cfg)
			if err != nil {
				return nil, nil, fmt.Errorf("[%s] insert: %w", scenario, err)
			}
			inserts = append(inserts, samples...)
		}
		if !cfg.SkipPagination {
			samples, err := pgPaginate(ctx, pool, scenario, cfg)
			if err != nil {
				return nil, nil, fmt.Errorf("[%s] paginate: %w", scenario, err)
			}
			pagings = append(pagings, samples...)
		}
	}

	return inserts, pagings, nil
}

func pgInsert(ctx context.Context, pool *pgxpool.Pool, scenario string, cfg Config) ([]InsertSample, error) {
	chunks := cfg.Rows / cfg.Chunk
	batchesPerChunk := cfg.Chunk / cfg.Batch
	samples := make([]InsertSample, 0, chunks)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	cumRows := 0
	for c := 0; c < chunks; c++ {
		chunkStart := time.Now()
		for b := 0; b < batchesPerChunk; b++ {
			if err := pgInsertBatch(ctx, pool, scenario, cfg.Batch, rng); err != nil {
				return nil, err
			}
		}
		chunkDur := time.Since(chunkStart)
		cumRows += cfg.Chunk
		total, idx, err := pgRelationSize(ctx, pool, scenario)
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

func pgInsertBatch(ctx context.Context, pool *pgxpool.Pool, scenario string, n int, rng *rand.Rand) error {
	var sb strings.Builder
	args := make([]any, 0, n*4)

	switch scenario {
	case "pg_serial":
		sb.WriteString("INSERT INTO pg_serial (user_id, amount, status) VALUES ")
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			off := i * 3
			fmt.Fprintf(&sb, "($%d,$%d,$%d)", off+1, off+2, off+3)
			args = append(args, rng.Int63n(10_000_000), randAmount(rng), randStatus(rng))
		}
	case "pg_uuidv4_app", "pg_uuidv7_app":
		table := scenario
		sb.WriteString(fmt.Sprintf("INSERT INTO %s (id, user_id, amount, status) VALUES ", table))
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			off := i * 4
			fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d)", off+1, off+2, off+3, off+4)
			var id uuid.UUID
			if scenario == "pg_uuidv4_app" {
				id = uuid.New()
			} else {
				id = mustUUIDv7()
			}
			args = append(args, id, rng.Int63n(10_000_000), randAmount(rng), randStatus(rng))
		}
	default:
		return fmt.Errorf("unknown postgres scenario %q", scenario)
	}

	_, err := pool.Exec(ctx, sb.String(), args...)
	return err
}

func pgRelationSize(ctx context.Context, pool *pgxpool.Pool, table string) (int64, int64, error) {
	var total, idx int64
	row := pool.QueryRow(ctx,
		"SELECT pg_total_relation_size($1), pg_indexes_size($1)", table)
	if err := row.Scan(&total, &idx); err != nil {
		return 0, 0, err
	}
	return total, idx, nil
}

func pgPaginate(ctx context.Context, pool *pgxpool.Pool, scenario string, cfg Config) ([]PaginationSample, error) {
	// pick a deep offset = (rows - limit*5)
	deepOffset := cfg.Rows - cfg.PaginationN*5
	if deepOffset < 0 {
		deepOffset = cfg.Rows / 2
	}
	out := []PaginationSample{}

	measure := func(method, sqlText string, args ...any) error {
		start := time.Now()
		rows, err := pool.Query(ctx, sqlText, args...)
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
	case "pg_serial":
		// OFFSET-based
		if err := measure("offset",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM pg_serial ORDER BY id LIMIT %d OFFSET %d", cfg.PaginationN, deepOffset)); err != nil {
			return nil, err
		}
		// cursor by created_at (uses created_at index)
		// pick a pivot row's created_at near the deepOffset
		var pivot time.Time
		row := pool.QueryRow(ctx,
			fmt.Sprintf("SELECT created_at FROM pg_serial ORDER BY created_at OFFSET %d LIMIT 1", deepOffset))
		if err := row.Scan(&pivot); err != nil {
			return nil, fmt.Errorf("pg_serial pivot: %w", err)
		}
		if err := measure("cursor_created_at",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM pg_serial WHERE created_at >= $1 ORDER BY created_at LIMIT %d", cfg.PaginationN),
			pivot); err != nil {
			return nil, err
		}
	case "pg_uuidv4_app":
		// only OFFSET makes sense (random PK isn't time-ordered)
		if err := measure("offset",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM pg_uuidv4_app ORDER BY id LIMIT %d OFFSET %d", cfg.PaginationN, deepOffset)); err != nil {
			return nil, err
		}
	case "pg_uuidv7_app":
		table := scenario
		// OFFSET-based comparison
		if err := measure("offset",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM %s ORDER BY id LIMIT %d OFFSET %d", table, cfg.PaginationN, deepOffset)); err != nil {
			return nil, err
		}
		// cursor by PK (UUIDv7 is time-ordered)
		var pivot uuid.UUID
		row := pool.QueryRow(ctx,
			fmt.Sprintf("SELECT id FROM %s ORDER BY id OFFSET %d LIMIT 1", table, deepOffset))
		if err := row.Scan(&pivot); err != nil {
			return nil, fmt.Errorf("%s pivot: %w", table, err)
		}
		if err := measure("cursor_pk",
			fmt.Sprintf("SELECT id, user_id, amount, status, created_at FROM %s WHERE id >= $1 ORDER BY id LIMIT %d", table, cfg.PaginationN),
			pivot); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func mustUUIDv7() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return id
}

func randAmount(rng *rand.Rand) string {
	return fmt.Sprintf("%d.%02d", rng.Intn(99999), rng.Intn(100))
}

var statuses = []string{"pending", "paid", "shipped", "cancelled", "refunded"}

func randStatus(rng *rand.Rand) string {
	return statuses[rng.Intn(len(statuses))]
}

func humanBytes(n int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.2fGB", float64(n)/GB)
	case n >= MB:
		return fmt.Sprintf("%.1fMB", float64(n)/MB)
	case n >= KB:
		return fmt.Sprintf("%.1fKB", float64(n)/KB)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
