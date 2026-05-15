package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func writeInsertCSV(path string, samples []InsertSample) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"scenario", "cum_rows", "chunk_rows", "chunk_ms", "rows_per_sec", "total_bytes", "index_bytes"}); err != nil {
		return err
	}
	for _, s := range samples {
		if err := w.Write([]string{
			s.Scenario,
			strconv.Itoa(s.CumRows),
			strconv.Itoa(s.ChunkRows),
			strconv.FormatInt(s.ChunkMS, 10),
			strconv.FormatFloat(s.RowsPerSec, 'f', 1, 64),
			strconv.FormatInt(s.TotalBytes, 10),
			strconv.FormatInt(s.IndexBytes, 10),
		}); err != nil {
			return err
		}
	}
	return nil
}

func writePaginationCSV(path string, samples []PaginationSample) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"scenario", "method", "offset", "limit", "millis"}); err != nil {
		return err
	}
	for _, s := range samples {
		if err := w.Write([]string{
			s.Scenario,
			s.Method,
			strconv.Itoa(s.Offset),
			strconv.Itoa(s.Limit),
			strconv.FormatFloat(s.Millis, 'f', 2, 64),
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeReport(path string, cfg Config, inserts []InsertSample, pagings []PaginationSample) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	fmt.Fprintf(w, "# UUID Primary Key Benchmark\n\n")
	fmt.Fprintf(w, "_Generated: %s_\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(w, "## Configuration\n\n")
	fmt.Fprintf(w, "- Rows per scenario: **%d**\n", cfg.Rows)
	fmt.Fprintf(w, "- Batch (rows per INSERT statement): **%d**\n", cfg.Batch)
	fmt.Fprintf(w, "- Chunk (rows per measurement checkpoint): **%d**\n", cfg.Chunk)
	fmt.Fprintf(w, "- Pagination limit: **%d**\n", cfg.PaginationN)
	fmt.Fprintf(w, "- Scenarios: %s\n\n", strings.Join(cfg.Scenarios, ", "))

	fmt.Fprintf(w, "Engines: PostgreSQL 17, MySQL 8.0\n\n")
	fmt.Fprintf(w, "Neither PG 17 nor MySQL 8 has a native `uuidv7()` generator, so UUIDv7 values are generated in the Go app for both engines.\n\n")

	writeInsertSection(w, inserts)
	writePaginationSection(w, pagings)
	writeStorageSection(w, inserts)
	writeAnalysisSection(w, inserts, pagings)

	return nil
}

func writeInsertSection(w *bufio.Writer, samples []InsertSample) {
	fmt.Fprintf(w, "## INSERT latency per chunk (ms)\n\n")
	scenarios, checkpoints := indexByScenarioAndCheckpoint(samples)
	if len(scenarios) == 0 {
		fmt.Fprintf(w, "_no insert samples_\n\n")
		return
	}

	fmt.Fprintf(w, "| cum_rows |")
	for _, s := range scenarios {
		fmt.Fprintf(w, " %s |", s)
	}
	fmt.Fprintf(w, "\n|---:|")
	for range scenarios {
		fmt.Fprintf(w, "---:|")
	}
	fmt.Fprintln(w)

	for _, cp := range checkpoints {
		fmt.Fprintf(w, "| %d |", cp)
		for _, s := range scenarios {
			if v, ok := lookupInsert(samples, s, cp); ok {
				fmt.Fprintf(w, " %d |", v.ChunkMS)
			} else {
				fmt.Fprintf(w, " - |")
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "## INSERT throughput per chunk (rows/sec)\n\n")
	fmt.Fprintf(w, "| cum_rows |")
	for _, s := range scenarios {
		fmt.Fprintf(w, " %s |", s)
	}
	fmt.Fprintf(w, "\n|---:|")
	for range scenarios {
		fmt.Fprintf(w, "---:|")
	}
	fmt.Fprintln(w)

	for _, cp := range checkpoints {
		fmt.Fprintf(w, "| %d |", cp)
		for _, s := range scenarios {
			if v, ok := lookupInsert(samples, s, cp); ok {
				fmt.Fprintf(w, " %.0f |", v.RowsPerSec)
			} else {
				fmt.Fprintf(w, " - |")
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
}

func writeStorageSection(w *bufio.Writer, samples []InsertSample) {
	fmt.Fprintf(w, "## Storage after each chunk (MB)\n\n")
	scenarios, checkpoints := indexByScenarioAndCheckpoint(samples)
	if len(scenarios) == 0 {
		fmt.Fprintf(w, "_no storage samples_\n\n")
		return
	}
	fmt.Fprintf(w, "| cum_rows |")
	for _, s := range scenarios {
		fmt.Fprintf(w, " %s total | %s idx |", s, s)
	}
	fmt.Fprintf(w, "\n|---:|")
	for range scenarios {
		fmt.Fprintf(w, "---:|---:|")
	}
	fmt.Fprintln(w)

	for _, cp := range checkpoints {
		fmt.Fprintf(w, "| %d |", cp)
		for _, s := range scenarios {
			if v, ok := lookupInsert(samples, s, cp); ok {
				fmt.Fprintf(w, " %.1f | %.1f |", float64(v.TotalBytes)/1024.0/1024.0, float64(v.IndexBytes)/1024.0/1024.0)
			} else {
				fmt.Fprintf(w, " - | - |")
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
}

func writePaginationSection(w *bufio.Writer, samples []PaginationSample) {
	if len(samples) == 0 {
		return
	}
	fmt.Fprintf(w, "## Pagination at deep offset (ms)\n\n")
	fmt.Fprintf(w, "| scenario | method | offset | limit | millis |\n|---|---|---:|---:|---:|\n")
	sort.SliceStable(samples, func(i, j int) bool {
		if samples[i].Scenario != samples[j].Scenario {
			return samples[i].Scenario < samples[j].Scenario
		}
		return samples[i].Method < samples[j].Method
	})
	for _, s := range samples {
		fmt.Fprintf(w, "| %s | %s | %d | %d | %.2f |\n", s.Scenario, s.Method, s.Offset, s.Limit, s.Millis)
	}
	fmt.Fprintln(w)
}

func writeAnalysisSection(w *bufio.Writer, inserts []InsertSample, pagings []PaginationSample) {
	fmt.Fprintf(w, "## Quick takeaways\n\n")

	if len(inserts) > 0 {
		// last-chunk-vs-first-chunk ratio = degradation indicator
		first := map[string]int64{}
		last := map[string]int64{}
		for _, s := range inserts {
			if _, ok := first[s.Scenario]; !ok {
				first[s.Scenario] = s.ChunkMS
			}
			last[s.Scenario] = s.ChunkMS
		}
		fmt.Fprintf(w, "**Insert degradation (last chunk ms / first chunk ms):**\n\n")
		fmt.Fprintf(w, "| scenario | first | last | ratio |\n|---|---:|---:|---:|\n")
		var keys []string
		for k := range first {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ratio := float64(last[k]) / float64(first[k])
			fmt.Fprintf(w, "| %s | %d | %d | %.2fx |\n", k, first[k], last[k], ratio)
		}
		fmt.Fprintln(w)
	}

	if len(pagings) > 0 {
		// fastest method per scenario
		bySc := map[string]PaginationSample{}
		for _, p := range pagings {
			cur, ok := bySc[p.Scenario]
			if !ok || p.Millis < cur.Millis {
				bySc[p.Scenario] = p
			}
		}
		fmt.Fprintf(w, "**Best pagination method per scenario:**\n\n")
		fmt.Fprintf(w, "| scenario | best method | millis |\n|---|---|---:|\n")
		var keys []string
		for k := range bySc {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "| %s | %s | %.2f |\n", k, bySc[k].Method, bySc[k].Millis)
		}
		fmt.Fprintln(w)
	}
}

func indexByScenarioAndCheckpoint(samples []InsertSample) ([]string, []int) {
	scSet := map[string]bool{}
	cpSet := map[int]bool{}
	for _, s := range samples {
		scSet[s.Scenario] = true
		cpSet[s.CumRows] = true
	}
	var scenarios []string
	for k := range scSet {
		scenarios = append(scenarios, k)
	}
	sort.Strings(scenarios)
	var checkpoints []int
	for k := range cpSet {
		checkpoints = append(checkpoints, k)
	}
	sort.Ints(checkpoints)
	return scenarios, checkpoints
}

func lookupInsert(samples []InsertSample, scenario string, cumRows int) (InsertSample, bool) {
	for _, s := range samples {
		if s.Scenario == scenario && s.CumRows == cumRows {
			return s, true
		}
	}
	return InsertSample{}, false
}
