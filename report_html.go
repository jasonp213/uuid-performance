package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// scenarioColors gives each known scenario a stable color so the same
// scenario is the same hue across every chart. Unknown scenarios fall
// back to fallbackColors by index.
var scenarioColors = map[string]string{
	"pg_serial":               "#1f77b4", // blue
	"pg_uuidv4_app":           "#ff7f0e", // orange
	"pg_uuidv7_app":           "#2ca02c", // green
	"mysql_autoinc":           "#d62728", // red
	"mysql_autoinc_uuidv4_uk": "#9467bd", // purple
	"mysql_uuidv4_app":        "#8c564b", // brown
	"mysql_uuidv7_app":        "#e377c2", // pink
}

var fallbackColors = []string{
	"#17becf", "#bcbd22", "#7f7f7f", "#aec7e8", "#ffbb78", "#98df8a", "#ff9896",
}

func colorFor(scenario string, idx int) string {
	if c, ok := scenarioColors[scenario]; ok {
		return c
	}
	return fallbackColors[idx%len(fallbackColors)]
}

// chartDataset mirrors the subset of Chart.js dataset options we set.
// Data uses *float64 so missing checkpoints serialize to JSON null and
// Chart.js can span the gap.
type chartDataset struct {
	Label           string     `json:"label"`
	Data            []*float64 `json:"data"`
	BorderColor     string     `json:"borderColor"`
	BackgroundColor string     `json:"backgroundColor"`
}

type htmlPayload struct {
	Labels     []string       `json:"labels"`
	Latency    []chartDataset `json:"latency"`
	Throughput []chartDataset `json:"throughput"`
	Storage    []chartDataset `json:"storage"`
	Pagination paginationData `json:"pagination"`
}

type paginationData struct {
	Labels   []string       `json:"labels"`
	Datasets []chartDataset `json:"datasets"`
}

func fptr(v float64) *float64 { return &v }

func writeHTMLReport(path string, cfg Config, inserts []InsertSample, pagings []PaginationSample) error {
	payload := buildHTMLPayload(inserts, pagings)
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	fmt.Fprintf(w, htmlTemplate,
		time.Now().Format(time.RFC3339),
		cfg.Rows, cfg.Batch, cfg.Chunk, cfg.PaginationN,
		strings.Join(cfg.Scenarios, ", "),
		string(data),
	)
	return nil
}

func buildHTMLPayload(inserts []InsertSample, pagings []PaginationSample) htmlPayload {
	scenarios, checkpoints := indexByScenarioAndCheckpoint(inserts)

	labels := make([]string, len(checkpoints))
	for i, cp := range checkpoints {
		labels[i] = humanRows(cp)
	}

	p := htmlPayload{Labels: labels}

	for i, sc := range scenarios {
		color := colorFor(sc, i)
		lat := chartDataset{Label: sc, BorderColor: color, BackgroundColor: color}
		thr := chartDataset{Label: sc, BorderColor: color, BackgroundColor: color}
		sto := chartDataset{Label: sc, BorderColor: color, BackgroundColor: color}
		for _, cp := range checkpoints {
			if v, ok := lookupInsert(inserts, sc, cp); ok {
				lat.Data = append(lat.Data, fptr(float64(v.ChunkMS)))
				thr.Data = append(thr.Data, fptr(v.RowsPerSec))
				sto.Data = append(sto.Data, fptr(float64(v.TotalBytes)/1024.0/1024.0))
			} else {
				lat.Data = append(lat.Data, nil)
				thr.Data = append(thr.Data, nil)
				sto.Data = append(sto.Data, nil)
			}
		}
		p.Latency = append(p.Latency, lat)
		p.Throughput = append(p.Throughput, thr)
		p.Storage = append(p.Storage, sto)
	}

	p.Pagination = buildPaginationData(pagings)
	return p
}

// buildPaginationData groups the pagination probes into a bar chart:
// one bar group per scenario, one dataset (colored series) per method.
func buildPaginationData(pagings []PaginationSample) paginationData {
	scSet := map[string]bool{}
	methodSet := map[string]bool{}
	val := map[string]map[string]float64{} // scenario -> method -> millis
	for _, s := range pagings {
		scSet[s.Scenario] = true
		methodSet[s.Method] = true
		if val[s.Scenario] == nil {
			val[s.Scenario] = map[string]float64{}
		}
		val[s.Scenario][s.Method] = s.Millis
	}

	scenarios := make([]string, 0, len(scSet))
	for k := range scSet {
		scenarios = append(scenarios, k)
	}
	sort.Strings(scenarios)

	methods := make([]string, 0, len(methodSet))
	for k := range methodSet {
		methods = append(methods, k)
	}
	sort.Strings(methods)

	methodColors := map[string]string{
		"offset":            "#d62728", // red — the O(offset) loser
		"cursor_created_at": "#2ca02c", // green
		"cursor_pk":         "#1f77b4", // blue
	}

	pd := paginationData{Labels: scenarios}
	for i, m := range methods {
		color := methodColors[m]
		if color == "" {
			color = fallbackColors[i%len(fallbackColors)]
		}
		ds := chartDataset{Label: m, BorderColor: color, BackgroundColor: color}
		for _, sc := range scenarios {
			if mv, ok := val[sc][m]; ok {
				ds.Data = append(ds.Data, fptr(mv))
			} else {
				ds.Data = append(ds.Data, nil)
			}
		}
		pd.Datasets = append(pd.Datasets, ds)
	}
	return pd
}

// humanRows formats a row count compactly: 100000 -> "100k", 1000000 -> "1M".
func humanRows(n int) string {
	switch {
	case n >= 1_000_000 && n%1_000_000 == 0:
		return fmt.Sprintf("%dM", n/1_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000 && n%1_000 == 0:
		return fmt.Sprintf("%dk", n/1_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>UUID Primary Key Benchmark</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
<style>
  :root { color-scheme: light dark; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
         margin: 0 auto; max-width: 1000px; padding: 24px; line-height: 1.5; }
  h1 { margin-bottom: 4px; }
  .meta { color: #888; font-size: 0.9em; margin-bottom: 24px; }
  .config { background: rgba(127,127,127,0.08); border-radius: 8px; padding: 12px 16px;
            font-size: 0.9em; margin-bottom: 32px; }
  .config code { font-size: 0.95em; }
  .chart-card { border: 1px solid rgba(127,127,127,0.25); border-radius: 10px;
                padding: 16px; margin-bottom: 28px; }
  .chart-card h2 { margin-top: 0; font-size: 1.1em; }
  .chart-card p { color: #888; font-size: 0.85em; margin-top: 0; }
  .chart-wrap { position: relative; height: 360px; }
  footer { color: #888; font-size: 0.8em; margin-top: 40px; }
</style>
</head>
<body>
<h1>UUID Primary Key Benchmark</h1>
<div class="meta">Generated %[1]s &middot; PostgreSQL 17 &middot; MySQL 8.0</div>

<div class="config">
  <strong>Configuration</strong><br>
  Rows/scenario: <code>%[2]d</code> &middot;
  Batch: <code>%[3]d</code> &middot;
  Chunk: <code>%[4]d</code> &middot;
  Pagination limit: <code>%[5]d</code><br>
  Scenarios: <code>%[6]s</code>
</div>

<div class="chart-card">
  <h2>INSERT latency per chunk (ms)</h2>
  <p>Lower is better. UUIDv4 should climb as the table grows; serial &amp; UUIDv7 stay roughly flat. Click a legend entry to toggle a scenario.</p>
  <div class="chart-wrap"><canvas id="latency"></canvas></div>
</div>

<div class="chart-card">
  <h2>INSERT throughput per chunk (rows/sec)</h2>
  <p>Higher is better.</p>
  <div class="chart-wrap"><canvas id="throughput"></canvas></div>
</div>

<div class="chart-card">
  <h2>Total relation size (MB)</h2>
  <p>Table + index size after each checkpoint.</p>
  <div class="chart-wrap"><canvas id="storage"></canvas></div>
</div>

<div class="chart-card">
  <h2>Pagination at deep offset (ms, log scale)</h2>
  <p>Time to fetch one deep page. <code>offset</code> scales O(offset); cursor methods stay near-constant.</p>
  <div class="chart-wrap"><canvas id="pagination"></canvas></div>
</div>

<footer>Generated by the uuid-performance benchmark driver. Charts via Chart.js (CDN).</footer>

<script>
const DATA = %[7]s;

const lineOpts = (yLabel, opts = {}) => ({
  type: 'line',
  options: {
    responsive: true, maintainAspectRatio: false,
    interaction: { mode: 'index', intersect: false },
    elements: { line: { tension: 0.2 }, point: { radius: 2, hoverRadius: 5 } },
    spanGaps: true,
    scales: {
      x: { title: { display: true, text: 'cumulative rows' } },
      y: Object.assign({ title: { display: true, text: yLabel }, beginAtZero: true }, opts.y || {})
    },
    plugins: { legend: { position: 'bottom' } }
  }
});

function lineDatasets(series) {
  return series.map(s => ({
    label: s.label, data: s.data,
    borderColor: s.borderColor, backgroundColor: s.backgroundColor
  }));
}

new Chart(document.getElementById('latency'),
  Object.assign({ data: { labels: DATA.labels, datasets: lineDatasets(DATA.latency) } }, lineOpts('ms')));

new Chart(document.getElementById('throughput'),
  Object.assign({ data: { labels: DATA.labels, datasets: lineDatasets(DATA.throughput) } }, lineOpts('rows/sec')));

new Chart(document.getElementById('storage'),
  Object.assign({ data: { labels: DATA.labels, datasets: lineDatasets(DATA.storage) } }, lineOpts('MB')));

new Chart(document.getElementById('pagination'), {
  type: 'bar',
  data: { labels: DATA.pagination.labels, datasets: DATA.pagination.datasets.map(d => ({
    label: d.label, data: d.data, backgroundColor: d.backgroundColor, borderColor: d.borderColor
  })) },
  options: {
    responsive: true, maintainAspectRatio: false,
    scales: {
      x: { title: { display: true, text: 'scenario' } },
      y: { type: 'logarithmic', title: { display: true, text: 'ms (log)' } }
    },
    plugins: { legend: { position: 'bottom' } }
  }
});
</script>
</body>
</html>
`
