package handler

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jbisss/webhook-manager/delivery-dashboard/internal/store"
)

const pageSize = 50

type Handler struct {
	store store.Store
	tmpl  *template.Template
}

func New(s store.Store) *Handler {
	return &Handler{
		store: s,
		tmpl:  template.Must(template.New("dash").Funcs(funcMap).Parse(pageTmpl)),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	eventID := q.Get("event_id")
	subscriptionID := q.Get("subscription_id")
	destinationURL := q.Get("destination_url")
	groupBy := q.Get("group_by")
	attemptsOp := q.Get("attempts_op")
	if attemptsOp != ">" && attemptsOp != "<" && attemptsOp != "=" {
		attemptsOp = ""
	}
	attemptsVal, _ := strconv.Atoi(q.Get("attempts_val"))

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	stats, _ := h.store.StatusStats(ctx)
	retryDist, _ := h.store.RetryDistribution(ctx)
	throughput, _ := h.store.ThroughputSeries(ctx)
	hasFilter := status != "" || eventID != "" || subscriptionID != "" || destinationURL != "" || attemptsOp != ""
	var filteredStats map[store.Status]int
	if hasFilter {
		filteredStats, _ = h.store.FilteredStats(ctx, status, eventID, subscriptionID, destinationURL, attemptsOp, attemptsVal)
	}

	data := map[string]any{
		"Stats":         stats,
		"Throughput":    throughput,
		"FilteredStats": filteredStats,
		"HasFilter":     hasFilter,
		"RetryDist":     retryDist,
		"Status":        status,
		"EventID":       eventID,
		"SubID":         subscriptionID,
		"DestURL":       destinationURL,
		"GroupBy":       groupBy,
		"AttemptsOp":    attemptsOp,
		"AttemptsVal":   attemptsVal,
		"Page":          page,
		"HasPrev":       false,
		"HasNext":       false,
		"Records":       nil,
		"GroupRows":     nil,
	}

	if groupBy != "" {
		rows, _ := h.store.GroupDeliveries(ctx, groupBy, status, eventID, subscriptionID, destinationURL)
		data["GroupRows"] = rows
	} else {
		records, _ := h.store.ListDeliveries(ctx, status, eventID, subscriptionID, destinationURL, attemptsOp, attemptsVal, pageSize+1, offset)
		hasNext := len(records) > pageSize
		if hasNext {
			records = records[:pageSize]
		}
		data["Records"] = records
		data["HasPrev"] = page > 1
		data["HasNext"] = hasNext
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.Execute(w, data)
}

var funcMap = template.FuncMap{
	"trunc": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "…"
	},
	"bytesToStr": func(b []byte) string { return string(b) },
	"fmtTime": func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.UTC().Format("2006-01-02 15:04:05")
	},
	"fmtTimePtr": func(t *time.Time) string {
		if t == nil {
			return "—"
		}
		return t.UTC().Format("2006-01-02 15:04:05")
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"sparklinePoints": func(points []store.ThroughputPoint, w, h int) template.HTML {
		if len(points) < 2 {
			return ""
		}
		maxC := 1
		for _, p := range points {
			if p.Count > maxC {
				maxC = p.Count
			}
		}
		n := len(points)
		var sb strings.Builder
		for i, p := range points {
			x := float64(i) / float64(n-1) * float64(w)
			y := float64(h) - float64(p.Count)/float64(maxC)*float64(h)
			if i > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(fmt.Sprintf("%.1f,%.1f", x, y))
		}
		return template.HTML(sb.String())
	},
	"sparklineArea": func(points []store.ThroughputPoint, w, h int) template.HTML {
		if len(points) < 2 {
			return ""
		}
		maxC := 1
		for _, p := range points {
			if p.Count > maxC {
				maxC = p.Count
			}
		}
		n := len(points)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("0,%d", h))
		for i, p := range points {
			x := float64(i) / float64(n-1) * float64(w)
			y := float64(h) - float64(p.Count)/float64(maxC)*float64(h)
			sb.WriteString(fmt.Sprintf(" %.1f,%.1f", x, y))
		}
		sb.WriteString(fmt.Sprintf(" %d,%d", w, h))
		return template.HTML(sb.String())
	},
	"sparklineMax": func(points []store.ThroughputPoint) int {
		maxC := 0
		for _, p := range points {
			if p.Count > maxC {
				maxC = p.Count
			}
		}
		return maxC
	},
	"successPct": func(stats map[store.Status]int) string {
		if stats == nil {
			return "—"
		}
		t := 0
		for _, v := range stats {
			t += v
		}
		if t == 0 {
			return "—"
		}
		return fmt.Sprintf("%.1f%%", float64(stats[store.StatusSuccess])*100/float64(t))
	},
	"attClass": func(n int) string {
		switch {
		case n == 0:
			return "att-0"
		case n == 1:
			return "att-1"
		case n == 2:
			return "att-2"
		default:
			return "att-3p"
		}
	},
	"statCount": func(stats map[store.Status]int, s store.Status) int {
		if stats == nil {
			return 0
		}
		return stats[s]
	},
	"total": func(stats map[store.Status]int) int {
		n := 0
		for _, v := range stats {
			n += v
		}
		return n
	},
	"statusClass": func(s store.Status) string {
		switch s {
		case store.StatusSuccess:
			return "success"
		case store.StatusExhausted:
			return "exhausted"
		default:
			return "pending"
		}
	},
	// buildURL builds the list-view URL preserving all active filters.
	"buildURL": func(status, eventID, subID, destURL, groupBy, attemptsOp string, attemptsVal, page int) string {
		u := fmt.Sprintf("/?page=%d", page)
		if status != "" {
			u += "&status=" + url.QueryEscape(status)
		}
		if eventID != "" {
			u += "&event_id=" + url.QueryEscape(eventID)
		}
		if subID != "" {
			u += "&subscription_id=" + url.QueryEscape(subID)
		}
		if destURL != "" {
			u += "&destination_url=" + url.QueryEscape(destURL)
		}
		if groupBy != "" {
			u += "&group_by=" + url.QueryEscape(groupBy)
		}
		if attemptsOp != "" {
			u += "&attempts_op=" + url.QueryEscape(attemptsOp)
			u += fmt.Sprintf("&attempts_val=%d", attemptsVal)
		}
		return u
	},
	// drillURL builds a URL that navigates from a group row into the flat list filtered by that value.
	"drillURL": func(groupBy, value, status string) string {
		u := "/?page=1"
		if status != "" {
			u += "&status=" + url.QueryEscape(status)
		}
		switch groupBy {
		case "event_id":
			u += "&event_id=" + url.QueryEscape(value)
		case "subscription_id":
			u += "&subscription_id=" + url.QueryEscape(value)
		case "destination_url":
			u += "&destination_url=" + url.QueryEscape(value)
		}
		return u
	},
	"hasAnyFilter": func(status, eventID, subID, destURL, attemptsOp string) bool {
		return status != "" || eventID != "" || subID != "" || destURL != "" || attemptsOp != ""
	},
	"groupByLabel": func(groupBy string) string {
		switch groupBy {
		case "event_id":
			return "Event ID"
		case "subscription_id":
			return "Subscription ID"
		case "destination_url":
			return "Destination URL"
		default:
			return groupBy
		}
	},
}

const pageTmpl = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Delivery Dashboard</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
html, body { height: 100%; }
body {
  font-family: "MS Sans Serif", Tahoma, Verdana, Arial, sans-serif;
  font-size: 13px;
  background: #008080;
  display: flex;
  flex-direction: column;
  height: 100%;
}

/* ── Window ─────────────────────────────────────── */
#window {
  background: #c0c0c0;
  border-top: 2px solid #ffffff;
  border-left: 2px solid #ffffff;
  border-right: 2px solid #404040;
  border-bottom: 2px solid #404040;
  margin: 10px;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

/* ── Title bar ───────────────────────────────────── */
#titlebar {
  background: linear-gradient(to right, #000080, #1084d0);
  color: #fff;
  font-size: 14px;
  font-weight: bold;
  padding: 5px 8px;
  display: flex;
  align-items: center;
  gap: 6px;
  user-select: none;
}
#titlebar .title-icon { font-size: 16px; }
#titlebar .title-text { flex: 1; letter-spacing: 0.5px; }

/* ── Content area ────────────────────────────────── */
#content {
  padding: 10px;
  overflow-y: auto;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* ── Panels ──────────────────────────────────────── */
.panel {
  background: #c0c0c0;
  border-top: 1px solid #ffffff;
  border-left: 1px solid #ffffff;
  border-right: 1px solid #808080;
  border-bottom: 1px solid #808080;
}
.panel-title {
  background: linear-gradient(to right, #000080, #1084d0);
  color: #fff;
  font-size: 12px;
  font-weight: bold;
  padding: 4px 10px;
  letter-spacing: 0.5px;
}
.panel-body { padding: 10px 12px; }

/* ── Stats ───────────────────────────────────────── */
.stats-row { display: flex; gap: 10px; flex-wrap: wrap; }
.stat-box {
  background: #c0c0c0;
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  padding: 10px 28px;
  min-width: 120px;
  text-align: center;
}
.stat-label { font-size: 11px; color: #444; text-transform: uppercase; letter-spacing: 1px; }
.stat-val { font-size: 28px; font-family: "Courier New", monospace; font-weight: bold; }
.stat-total .stat-val    { color: #000080; }
.stat-pending .stat-val  { color: #996600; }
.stat-success .stat-val  { color: #006600; }
.stat-exhausted .stat-val { color: #880000; }

/* ── Filter form ─────────────────────────────────── */
.filter-grid {
  display: grid;
  grid-template-columns: auto 1fr auto 1fr;
  gap: 6px 10px;
  align-items: center;
}
.filter-grid label { font-size: 13px; font-weight: bold; white-space: nowrap; }
.filter-actions { display: flex; gap: 8px; align-items: center; margin-top: 8px; flex-wrap: wrap; }
select, input[type=text] {
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  background: #fff;
  font-size: 13px;
  padding: 3px 6px;
  font-family: "MS Sans Serif", Tahoma, Verdana, sans-serif;
  width: 100%;
}
.btn {
  background: #c0c0c0;
  border-top: 2px solid #ffffff;
  border-left: 2px solid #ffffff;
  border-right: 2px solid #808080;
  border-bottom: 2px solid #808080;
  font-size: 13px;
  padding: 3px 18px;
  cursor: pointer;
  font-family: "MS Sans Serif", Tahoma, Verdana, sans-serif;
  white-space: nowrap;
}
.btn:active {
  border-top: 2px solid #808080;
  border-left: 2px solid #808080;
  border-right: 2px solid #ffffff;
  border-bottom: 2px solid #ffffff;
}
.btn-clear {
  background: #c0c0c0;
  border: none;
  color: #800000;
  font-size: 13px;
  cursor: pointer;
  padding: 3px 6px;
  text-decoration: underline;
  font-family: "MS Sans Serif", Tahoma, Verdana, sans-serif;
}

/* ── Table ───────────────────────────────────────── */
.table-wrap {
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
  background: #ffffff;
  font-size: 13px;
}
thead tr { background: #000080; color: #fff; }
thead th {
  padding: 5px 10px;
  text-align: left;
  font-size: 12px;
  font-weight: bold;
  letter-spacing: 0.5px;
  white-space: nowrap;
  border-right: 1px solid #0000aa;
}
tbody tr:nth-child(odd)  { background: #f0f0f0; }
tbody tr:nth-child(even) { background: #ffffff; }
tbody tr:hover { background: #c0c8e8; cursor: pointer; }
tbody tr.selected { background: #000080 !important; color: #fff; }
tbody tr.selected .badge { border-color: #fff; }
td {
  padding: 4px 10px;
  border-bottom: 1px solid #d8d8d8;
  border-right: 1px solid #e8e8e8;
  font-family: "Courier New", monospace;
  vertical-align: top;
  white-space: nowrap;
  overflow: hidden;
  max-width: 220px;
}
td.url { max-width: 300px; }
td.err { max-width: 260px; }
td.center { text-align: center; max-width: 70px; }
td.num    { text-align: right; max-width: 80px; }

/* ── Badge ───────────────────────────────────────── */
.badge {
  display: inline-block;
  padding: 1px 7px;
  font-size: 11px;
  font-weight: bold;
  font-family: "MS Sans Serif", Tahoma, Verdana, sans-serif;
  white-space: nowrap;
  border: 1px solid;
}
.badge-pending   { background: #ffff99; color: #664400; border-color: #ccaa00; }
.badge-success   { background: #ccffcc; color: #004400; border-color: #008800; }
.badge-exhausted { background: #ffcccc; color: #660000; border-color: #cc0000; }

/* ── Group summary numbers ───────────────────────── */
.g-total     { color: #000080; font-weight: bold; }
.g-success   { color: #006600; }
.g-pending   { color: #996600; }
.g-exhausted { color: #880000; }

/* ── Detail pane ─────────────────────────────────── */
#detail-pane {
  display: none;
  background: #c0c0c0;
  border-top: 1px solid #ffffff;
  border-left: 1px solid #ffffff;
  border-right: 1px solid #808080;
  border-bottom: 1px solid #808080;
}
#detail-pane .panel-body {
  font-family: "Courier New", monospace;
  font-size: 12px;
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 4px 8px;
}
#detail-pane .dk { font-weight: bold; color: #000080; white-space: nowrap; }
#detail-pane .dv { word-break: break-all; }

/* ── Pagination ──────────────────────────────────── */
.pager { display: flex; align-items: center; gap: 8px; font-size: 13px; margin-top: 6px; }
.pager a {
  color: #000080;
  text-decoration: none;
  background: #c0c0c0;
  border-top: 2px solid #ffffff;
  border-left: 2px solid #ffffff;
  border-right: 2px solid #808080;
  border-bottom: 2px solid #808080;
  padding: 2px 14px;
}
.pager a:hover { background: #b0b0e0; }
.pager .cur { font-weight: bold; color: #000080; }

.no-data {
  text-align: center;
  padding: 30px;
  color: #666;
  font-style: italic;
  font-size: 13px;
}

/* ── Status bar ──────────────────────────────────── */
#statusbar {
  border-top: 1px solid #808080;
  background: #c0c0c0;
  padding: 3px 8px;
  display: flex;
  gap: 8px;
  font-size: 13px;
  align-items: center;
}
#statusbar .sb-cell {
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  padding: 2px 8px;
}
.statusbar-clock {
  margin-left: auto;
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  padding: 2px 10px;
  font-family: "Courier New", monospace;
  font-size: 13px;
}

/* ── Stats sections ─────────────────────────────── */
.stats-sections { display: flex; flex-direction: column; gap: 8px; flex: 1; }
.stats-section-label {
  font-size: 10px; font-weight: bold; color: #000080;
  letter-spacing: 1px; text-transform: uppercase;
  border-bottom: 1px solid #808080; padding-bottom: 2px; margin-bottom: 4px;
}
.stats-right { display: flex; flex-direction: column; gap: 8px; align-items: flex-end; justify-content: space-between; }
.stat-pct { font-size: 11px; color: #006600; font-family: "Courier New", monospace; margin-top: 2px; }
.stat-pct-filtered { color: #004488; }

/* ── Refresh countdown in titlebar ──────────────── */
.refresh-counter {
  font-size: 15px;
  font-weight: bold;
  font-family: "Courier New", monospace;
  color: #ffffff;
  background: rgba(0,0,0,0.35);
  padding: 3px 12px;
  border-radius: 2px;
  letter-spacing: 1px;
  min-width: 56px;
  text-align: center;
}

/* ── Filtered/total denominator ─────────────────── */
.stat-denom {
  font-size: 15px;
  color: #444444;
  font-weight: normal;
}

/* ── Sparkline ───────────────────────────────────── */
.sparkline-wrap {
  position: relative;
  background: #ffffff;
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  padding: 4px 6px 2px;
}
.sparkline-labels {
  display: flex;
  justify-content: space-between;
  font-family: "Courier New", monospace;
  font-size: 10px;
  color: #666;
  margin-top: 2px;
}
.sparkline-max {
  position: absolute;
  top: 4px;
  right: 8px;
  font-size: 10px;
  font-family: "Courier New", monospace;
  color: #000080;
  font-weight: bold;
}

/* ── Live indicator ──────────────────────────────── */
.live-indicator {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: bold;
  letter-spacing: 1px;
  color: #90ff90;
}
.live-dot {
  width: 8px; height: 8px;
  background: #00ff00;
  border-radius: 50%;
  box-shadow: 0 0 4px #00ff00;
  animation: liveblink 1.2s ease-in-out infinite;
}
@keyframes liveblink {
  0%, 100% { opacity: 1; }
  50%       { opacity: 0.15; }
}

/* ── Retry distribution chart ────────────────────── */
.retry-chart { display: flex; flex-direction: column; gap: 5px; }
.rc-row { display: flex; align-items: center; gap: 6px; }
.rc-label {
  font-family: "Courier New", monospace;
  font-weight: bold;
  font-size: 12px;
  width: 26px;
  text-align: right;
}
.rc-bar-wrap {
  flex: 1;
  background: #d8d8d8;
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  height: 13px;
  min-width: 80px;
}
.rc-bar { height: 100%; min-width: 2px; }
.rc-0  { background: #228B22; }
.rc-1  { background: #DAA520; }
.rc-2  { background: #CC6600; }
.rc-3p { background: #8B0000; }
.rc-count {
  font-family: "Courier New", monospace;
  font-size: 12px;
  width: 52px;
  color: #444;
}

/* ── Attempt badges ──────────────────────────────── */
.att-badge {
  display: inline-block;
  padding: 1px 6px;
  font-weight: bold;
  font-size: 12px;
  font-family: "Courier New", monospace;
  border: 1px solid;
}
.att-0  { background: #e8ffe8; color: #006600; border-color: #008800; }
.att-1  { background: #ffffd0; color: #664400; border-color: #aaaa00; }
.att-2  { background: #ffe8cc; color: #884400; border-color: #cc6600; }
.att-3p { background: #ffe8e8; color: #880000; border-color: #cc0000; }
</style>
</head>
<body>

<div id="window">

  <!-- Title bar -->
  <div id="titlebar">
    <span class="title-icon">&#128274;</span>
    <span class="title-text">Delivery Dashboard — Webhook Engine</span>
    <span class="live-indicator"><span class="live-dot"></span>LIVE</span>
    <span class="refresh-counter" id="title-refresh">↻ 10s</span>
  </div>

  <!-- Content -->
  <div id="content">

    <!-- Stats -->
    <div class="panel">
      <div class="panel-title">System Statistics</div>
      <div class="panel-body" style="display:flex;gap:12px;align-items:flex-start;">

        <!-- Stats boxes -->
        <div class="stats-sections">
          <div class="stats-row">
            <div class="stat-box stat-total">
              <div class="stat-label">Total</div>
              <div class="stat-val">
                {{ if .HasFilter }}{{ total .FilteredStats }}<span class="stat-denom">/{{ total .Stats }}</span>{{ else }}{{ total .Stats }}{{ end }}
              </div>
            </div>
            <div class="stat-box stat-pending">
              <div class="stat-label">Pending</div>
              <div class="stat-val">
                {{ if .HasFilter }}{{ statCount .FilteredStats "pending" }}<span class="stat-denom">/{{ statCount .Stats "pending" }}</span>{{ else }}{{ statCount .Stats "pending" }}{{ end }}
              </div>
            </div>
            <div class="stat-box stat-success">
              <div class="stat-label">Success</div>
              <div class="stat-val">
                {{ if .HasFilter }}{{ statCount .FilteredStats "success" }}<span class="stat-denom">/{{ statCount .Stats "success" }}</span>{{ else }}{{ statCount .Stats "success" }}{{ end }}
              </div>
              <div class="stat-pct">{{ if .HasFilter }}{{ successPct .FilteredStats }}{{ else }}{{ successPct .Stats }}{{ end }}</div>
            </div>
            <div class="stat-box stat-exhausted">
              <div class="stat-label">Exhausted</div>
              <div class="stat-val">
                {{ if .HasFilter }}{{ statCount .FilteredStats "exhausted" }}<span class="stat-denom">/{{ statCount .Stats "exhausted" }}</span>{{ else }}{{ statCount .Stats "exhausted" }}{{ end }}
              </div>
            </div>
          </div>
        </div>

        <!-- Right: separator + retry dist + refresh -->
        <div style="display:flex;gap:10px;align-items:flex-start;margin-left:auto;">
          <div style="width:1px;background:#808080;align-self:stretch;"></div>
          <div class="stat-box" style="min-width:180px;">
            <div class="stat-label" style="margin-bottom:6px;">Retry Distribution</div>
            <div class="retry-chart">
              {{ range .RetryDist }}
              <div class="rc-row">
                <span class="rc-label">{{ .Label }}</span>
                <div class="rc-bar-wrap"><div class="rc-bar {{ .BarClass }}" style="width:{{ .Pct }}%"></div></div>
                <span class="rc-count">{{ .Count }}</span>
              </div>
              {{ end }}
            </div>
          </div>
          <div style="display:flex;flex-direction:column;justify-content:center;">
            <button class="btn" onclick="document.getElementById('filter-form').submit()">&#8635; Refresh</button>
          </div>
        </div>

      </div>
    </div>

    <!-- Throughput sparkline -->
    <div class="panel">
      <div class="panel-title">Throughput &nbsp;<span style="font-weight:normal;font-size:11px;">deliveries created / minute — last 60 min</span></div>
      <div class="panel-body" style="padding:6px 10px;">
        {{ if .Throughput }}
        <div class="sparkline-wrap">
          <span class="sparkline-max">peak: {{ sparklineMax .Throughput }}/min</span>
          <svg viewBox="0 0 600 55" preserveAspectRatio="none" style="width:100%;height:55px;display:block;">
            <line x1="0" y1="18" x2="600" y2="18" stroke="#eeeeee" stroke-width="1"/>
            <line x1="0" y1="36" x2="600" y2="36" stroke="#eeeeee" stroke-width="1"/>
            <polygon points="{{ sparklineArea .Throughput 600 53 }}" fill="#000080" fill-opacity="0.10"/>
            <polyline points="{{ sparklinePoints .Throughput 600 53 }}" fill="none" stroke="#000080" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/>
          </svg>
          <div class="sparkline-labels">
            <span>−60 min</span>
            <span>−45 min</span>
            <span>−30 min</span>
            <span>−15 min</span>
            <span>now</span>
          </div>
        </div>
        {{ else }}
        <div class="no-data">[ no data for last 60 minutes ]</div>
        {{ end }}
      </div>
    </div>

    <!-- Filter -->
    <div class="panel">
      <div class="panel-title">Filter &amp; Group By</div>
      <div class="panel-body">
        <form id="filter-form" method="get" action="/">
          <div class="filter-grid">
            <label>Status:</label>
            <select name="status">
              <option value=""{{ if eq .Status "" }} selected{{ end }}>All</option>
              <option value="pending"{{ if eq .Status "pending" }} selected{{ end }}>pending</option>
              <option value="success"{{ if eq .Status "success" }} selected{{ end }}>success</option>
              <option value="exhausted"{{ if eq .Status "exhausted" }} selected{{ end }}>exhausted</option>
            </select>
            <label>Event ID:</label>
            <input type="text" name="event_id" value="{{ .EventID }}" placeholder="evt_01H...">

            <label>Subscription ID:</label>
            <input type="text" name="subscription_id" value="{{ .SubID }}" placeholder="sub_01H...">
            <label>Destination URL:</label>
            <input type="text" name="destination_url" value="{{ .DestURL }}" placeholder="https://...">

            <label>Retry #:</label>
            <div style="display:flex;gap:4px;align-items:center;">
              <select name="attempts_op" id="att-op" style="width:auto;" onchange="document.getElementById('att-val').disabled=!this.value">
                <option value="">— Any —</option>
                <option value="="{{ if eq .AttemptsOp "=" }} selected{{ end }}>=</option>
                <option value=">"{{ if eq .AttemptsOp ">" }} selected{{ end }}>&gt;</option>
                <option value="<"{{ if eq .AttemptsOp "<" }} selected{{ end }}>&lt;</option>
              </select>
              <input type="number" name="attempts_val" id="att-val" value="{{ .AttemptsVal }}" min="0" style="width:70px;"{{ if eq .AttemptsOp "" }} disabled{{ end }}>
            </div>
          </div>
          <div class="filter-actions">
            <label style="font-weight:bold;">Group by:</label>
            <select name="group_by" style="width:auto;">
              <option value=""{{ if eq .GroupBy "" }} selected{{ end }}>— None (flat list) —</option>
              <option value="event_id"{{ if eq .GroupBy "event_id" }} selected{{ end }}>Event ID</option>
              <option value="subscription_id"{{ if eq .GroupBy "subscription_id" }} selected{{ end }}>Subscription ID</option>
              <option value="destination_url"{{ if eq .GroupBy "destination_url" }} selected{{ end }}>Destination URL</option>
            </select>
            <button type="submit" class="btn">&#9654; Apply</button>
            {{ if hasAnyFilter .Status .EventID .SubID .DestURL .AttemptsOp }}
            <button type="button" class="btn-clear" onclick="location.href='/'">&#10005; Clear filters</button>
            {{ end }}
          </div>
        </form>
      </div>
    </div>

    {{ if .GroupBy }}

    <!-- Grouped view -->
    <div class="panel" style="flex:1; display:flex; flex-direction:column; min-height:0;">
      <div class="panel-title">
        Grouped by: {{ groupByLabel .GroupBy }}
        {{ if ne .Status "" }}&nbsp;[{{ .Status }}]{{ end }}
        &nbsp;<span style="font-weight:normal;">— click a row to drill into its deliveries</span>
      </div>
      <div class="panel-body" style="flex:1; display:flex; flex-direction:column; min-height:0; padding:4px;">
        {{ if .GroupRows }}
        <div class="table-wrap" style="flex:1; overflow:auto;">
        <table>
          <thead>
            <tr>
              <th>{{ groupByLabel .GroupBy }}</th>
              <th style="text-align:right;">Total</th>
              <th style="text-align:right;">Success</th>
              <th style="text-align:right;">Pending</th>
              <th style="text-align:right;">Exhausted</th>
            </tr>
          </thead>
          <tbody>
            {{ $status := .Status }}{{ $groupBy := .GroupBy }}
            {{ range .GroupRows }}
            <tr onclick="location.href='{{ drillURL $groupBy .Value $status }}'">
              <td class="url" title="{{ .Value }}">{{ trunc .Value 60 }}</td>
              <td class="num g-total">{{ .Total }}</td>
              <td class="num g-success">{{ .Success }}</td>
              <td class="num g-pending">{{ .Pending }}</td>
              <td class="num g-exhausted">{{ .Exhausted }}</td>
            </tr>
            {{ end }}
          </tbody>
        </table>
        </div>
        {{ else }}
        <div class="no-data">[ no groups found ]</div>
        {{ end }}
      </div>
    </div>

    {{ else }}

    <!-- Table area: list left, detail right -->
    <div style="display:flex;gap:8px;flex:1;min-height:0;">

    <!-- Flat list -->
    <div class="panel" style="flex:1;min-width:0;display:flex;flex-direction:column;min-height:0;">
      <div class="panel-title">
        Deliveries
        {{ if ne .Status "" }}&nbsp;[{{ .Status }}]{{ end }}
        {{ if ne .EventID "" }}&nbsp;event={{ .EventID }}{{ end }}
        {{ if ne .SubID "" }}&nbsp;sub={{ .SubID }}{{ end }}
        {{ if ne .DestURL "" }}&nbsp;url={{ trunc .DestURL 30 }}{{ end }}
        &nbsp;<span style="font-weight:normal;">(page {{ .Page }}, up to 50 per page)</span>
      </div>
      <div class="panel-body" style="flex:1; display:flex; flex-direction:column; min-height:0; padding:4px;">

        {{ if .Records }}
        <div class="table-wrap" style="flex:1; overflow:auto;">
        <table>
          <thead>
            <tr>
              <th>Delivery ID</th>
              <th>Event ID</th>
              <th>Subscription ID</th>
              <th>Destination URL</th>
              <th>MTH</th>
              <th>Status</th>
              <th>Retry #</th>
              <th>Next Attempt</th>
              <th>Last Error</th>
              <th>Payload</th>
              <th>Created At</th>
              <th>Updated At</th>
            </tr>
          </thead>
          <tbody id="tbody">
            {{ range .Records }}
            <tr onclick="showDetail(this)"
              data-id="{{ .ID }}"
              data-event="{{ .EventID }}"
              data-sub="{{ .SubscriptionID }}"
              data-url="{{ .DestinationURL }}"
              data-method="{{ .Method }}"
              data-status="{{ .Status }}"
              data-attempts="{{ .Attempts }}"
              data-next="{{ fmtTimePtr .NextAttempt }}"
              data-error="{{ .LastError }}"
              data-payload="{{ bytesToStr .Payload }}"
              data-created="{{ fmtTime .CreatedAt }}"
              data-updated="{{ fmtTime .UpdatedAt }}"
            >
              <td title="{{ .ID }}">{{ trunc .ID 20 }}</td>
              <td title="{{ .EventID }}">{{ trunc .EventID 20 }}</td>
              <td title="{{ .SubscriptionID }}">{{ trunc .SubscriptionID 16 }}</td>
              <td class="url" title="{{ .DestinationURL }}">{{ trunc .DestinationURL 38 }}</td>
              <td class="center">{{ .Method }}</td>
              <td class="center"><span class="badge badge-{{ statusClass .Status }}">{{ .Status }}</span></td>
              <td class="center"><span class="att-badge {{ attClass .Attempts }}">{{ .Attempts }}</span></td>
              <td>{{ fmtTimePtr .NextAttempt }}</td>
              <td class="err" title="{{ .LastError }}">{{ trunc .LastError 38 }}</td>
              <td title="{{ bytesToStr .Payload }}">{{ trunc (bytesToStr .Payload) 38 }}</td>
              <td>{{ fmtTime .CreatedAt }}</td>
              <td>{{ fmtTime .UpdatedAt }}</td>
            </tr>
            {{ end }}
          </tbody>
        </table>
        </div>

        <div class="pager">
          {{ if .HasPrev }}
          <a href="{{ buildURL .Status .EventID .SubID .DestURL .GroupBy .AttemptsOp .AttemptsVal (sub .Page 1) }}">&#9664; Prev</a>
          {{ end }}
          <span class="cur">Page {{ .Page }}</span>
          {{ if .HasNext }}
          <a href="{{ buildURL .Status .EventID .SubID .DestURL .GroupBy .AttemptsOp .AttemptsVal (add .Page 1) }}">Next &#9654;</a>
          {{ end }}
        </div>

        {{ else }}
        <div class="no-data">[ no records found ]</div>
        {{ end }}

      </div>
    </div>

    <!-- Detail pane (right side) -->
    <div class="panel" id="detail-pane" style="flex:0 0 340px;overflow-y:auto;">
      <div class="panel-title">
        Delivery Details
        <button class="btn" onclick="closeDetail()" style="float:right;margin:-1px 0 0 0;padding:0 6px;font-size:10px;">&#10005; Close</button>
      </div>
      <div class="panel-body" id="detail-body"></div>
    </div>

    </div><!-- /table area -->

    {{ end }}<!-- /GroupBy toggle -->

  </div><!-- /content -->

  <!-- Status bar -->
  <div id="statusbar">
    <span class="sb-cell" id="sb-status">Ready</span>
    <span class="sb-cell">delivery-dashboard v1.0</span>
    <span class="statusbar-clock" id="clock">--:--:--</span>
  </div>

</div><!-- /window -->

<script>
// Clock
function tick() {
  var d = new Date();
  document.getElementById('clock').textContent =
    d.getHours().toString().padStart(2,'0') + ':' +
    d.getMinutes().toString().padStart(2,'0') + ':' +
    d.getSeconds().toString().padStart(2,'0');
}
tick(); setInterval(tick, 1000);

// Auto-refresh countdown
var REFRESH_SECS = 10;
var countdown = REFRESH_SECS;
var refreshTimer = setInterval(function() {
  countdown--;
  var ti = document.getElementById('title-refresh');
  if (ti) ti.textContent = countdown > 0 ? '↻ ' + countdown + 's' : '↻ …';
  if (countdown <= 0) {
    clearInterval(refreshTimer);
    if (selectedRow) sessionStorage.setItem('_dlv', selectedRow.dataset.id);
    document.getElementById('filter-form').submit();
  }
}, 1000);

// Row detail (flat list only)
var selectedRow = null;
function showDetail(row) {
  if (selectedRow) selectedRow.classList.remove('selected');
  selectedRow = row;
  row.classList.add('selected');

  var d = row.dataset;
  var pairs = [
    ['Delivery ID',     d.id],
    ['Event ID',        d.event],
    ['Subscription ID', d.sub],
    ['Destination URL', d.url],
    ['Method',          d.method],
    ['Status',          d.status],
    ['Attempts',        d.attempts],
    ['Next Attempt',    d.next],
    ['Last Error',      d.error || '—'],
    ['Payload',         d.payload || '—'],
    ['Created At',      d.created],
    ['Updated At',      d.updated],
  ];

  var html = '';
  pairs.forEach(function(p) {
    html += '<span class="dk">' + p[0] + ':</span><span class="dv">' + esc(p[1]) + '</span>';
  });

  document.getElementById('detail-body').innerHTML = html;
  document.getElementById('detail-pane').style.display = 'block';
  document.getElementById('sb-status').textContent = 'Selected: ' + d.id;
}

function closeDetail() {
  document.getElementById('detail-pane').style.display = 'none';
  if (selectedRow) { selectedRow.classList.remove('selected'); selectedRow = null; }
  document.getElementById('sb-status').textContent = 'Ready';
}

// Restore selected row after auto-refresh
var _savedId = sessionStorage.getItem('_dlv');
if (_savedId) {
  sessionStorage.removeItem('_dlv');
  document.querySelectorAll('#tbody tr').forEach(function(row) {
    if (row.dataset.id === _savedId) { showDetail(row); }
  });
}

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}
</script>
</body>
</html>`
