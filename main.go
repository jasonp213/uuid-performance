package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Config struct {
	Rows           int
	Batch          int
	Chunk          int
	Scenarios      []string
	PgDSN          string
	MySQLDSN       string
	OutputDir      string
	PaginationN    int
	SkipInsert     bool
	SkipPagination bool
	Truncate       bool
}

type InsertSample struct {
	Scenario   string
	CumRows    int
	ChunkRows  int
	ChunkMS    int64
	RowsPerSec float64
	TotalBytes int64
	IndexBytes int64
}

type PaginationSample struct {
	Scenario string
	Method   string
	Offset   int
	Limit    int
	Millis   float64
}

var allScenarios = []string{
	"pg_serial",
	"pg_uuidv4_app",
	"pg_uuidv7_app",
	"mysql_autoinc",
	"mysql_uuidv4_app",
	"mysql_uuidv7_app",
}

func main() {
	cfg := Config{}
	var scenariosFlag string

	flag.IntVar(&cfg.Rows, "rows", 1_000_000, "total rows to insert per scenario")
	flag.IntVar(&cfg.Batch, "batch", 1000, "rows per multi-value INSERT statement")
	flag.IntVar(&cfg.Chunk, "chunk", 100_000, "rows per measurement checkpoint")
	flag.StringVar(&scenariosFlag, "scenarios", "all", "comma-separated scenario names, or 'all' / 'pg' / 'mysql'")
	flag.StringVar(&cfg.PgDSN, "pg-dsn", "postgres://bench:bench@127.0.0.1:55432/bench?sslmode=disable", "PostgreSQL DSN")
	flag.StringVar(&cfg.MySQLDSN, "mysql-dsn", "bench:bench@tcp(127.0.0.1:53306)/bench?parseTime=true&multiStatements=true", "MySQL DSN")
	flag.StringVar(&cfg.OutputDir, "output-dir", "results", "directory to write report and CSV files")
	flag.IntVar(&cfg.PaginationN, "pagination-limit", 100, "rows returned per pagination query")
	flag.BoolVar(&cfg.SkipInsert, "skip-insert", false, "skip insert phase (assumes tables already populated)")
	flag.BoolVar(&cfg.SkipPagination, "skip-pagination", false, "skip pagination phase")
	flag.BoolVar(&cfg.Truncate, "truncate", true, "truncate tables before insert phase")

	flag.Parse()

	cfg.Scenarios = expandScenarios(scenariosFlag)
	if cfg.Chunk > cfg.Rows {
		cfg.Chunk = cfg.Rows
	}
	if cfg.Rows%cfg.Chunk != 0 {
		log.Fatalf("rows (%d) must be divisible by chunk (%d)", cfg.Rows, cfg.Chunk)
	}
	if cfg.Chunk%cfg.Batch != 0 {
		log.Fatalf("chunk (%d) must be divisible by batch (%d)", cfg.Chunk, cfg.Batch)
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		log.Fatalf("mkdir output: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("config: rows=%d batch=%d chunk=%d scenarios=%v output=%s", cfg.Rows, cfg.Batch, cfg.Chunk, cfg.Scenarios, cfg.OutputDir)

	var inserts []InsertSample
	var pagings []PaginationSample

	pgScenarios := filter(cfg.Scenarios, func(s string) bool { return strings.HasPrefix(s, "pg_") })
	mysqlScenarios := filter(cfg.Scenarios, func(s string) bool { return strings.HasPrefix(s, "mysql_") })

	if len(pgScenarios) > 0 {
		log.Printf("== PostgreSQL ==")
		pgIns, pgPag, err := runPostgres(ctx, cfg, pgScenarios)
		if err != nil {
			log.Fatalf("postgres bench: %v", err)
		}
		inserts = append(inserts, pgIns...)
		pagings = append(pagings, pgPag...)
	}

	if len(mysqlScenarios) > 0 {
		log.Printf("== MySQL ==")
		myIns, myPag, err := runMySQL(ctx, cfg, mysqlScenarios)
		if err != nil {
			log.Fatalf("mysql bench: %v", err)
		}
		inserts = append(inserts, myIns...)
		pagings = append(pagings, myPag...)
	}

	if err := writeInsertCSV(filepath.Join(cfg.OutputDir, "inserts.csv"), inserts); err != nil {
		log.Fatalf("write inserts.csv: %v", err)
	}
	if err := writePaginationCSV(filepath.Join(cfg.OutputDir, "pagination.csv"), pagings); err != nil {
		log.Fatalf("write pagination.csv: %v", err)
	}

	reportPath := filepath.Join(cfg.OutputDir, "report.md")
	if err := writeReport(reportPath, cfg, inserts, pagings); err != nil {
		log.Fatalf("write report: %v", err)
	}
	log.Printf("done. report: %s", reportPath)
}

func expandScenarios(flag string) []string {
	flag = strings.TrimSpace(flag)
	switch flag {
	case "", "all":
		return append([]string{}, allScenarios...)
	case "pg":
		return filter(allScenarios, func(s string) bool { return strings.HasPrefix(s, "pg_") })
	case "mysql":
		return filter(allScenarios, func(s string) bool { return strings.HasPrefix(s, "mysql_") })
	}
	parts := strings.Split(flag, ",")
	out := make([]string, 0, len(parts))
	known := make(map[string]bool, len(allScenarios))
	for _, s := range allScenarios {
		known[s] = true
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !known[p] {
			fmt.Fprintf(os.Stderr, "unknown scenario %q (known: %s)\n", p, strings.Join(allScenarios, ","))
			os.Exit(2)
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func filter(in []string, pred func(string) bool) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if pred(s) {
			out = append(out, s)
		}
	}
	return out
}
