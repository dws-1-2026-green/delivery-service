package handler

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
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
	status := r.URL.Query().Get("status")
	eventID := r.URL.Query().Get("event_id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	stats, _ := h.store.StatusStats(ctx)
	records, _ := h.store.ListDeliveries(ctx, status, eventID, pageSize+1, offset)

	hasNext := len(records) > pageSize
	if hasNext {
		records = records[:pageSize]
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.Execute(w, map[string]any{
		"Records": records,
		"Stats":   stats,
		"Status":  status,
		"EventID": eventID,
		"Page":    page,
		"HasPrev": page > 1,
		"HasNext": hasNext,
	})
}

var funcMap = template.FuncMap{
	"trunc": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "…"
	},
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
	"buildURL": func(status, eventID string, page int) string {
		u := fmt.Sprintf("/?page=%d", page)
		if status != "" {
			u += "&status=" + status
		}
		if eventID != "" {
			u += "&event_id=" + eventID
		}
		return u
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
  font-size: 11px;
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
  margin: 8px;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

/* ── Title bar ───────────────────────────────────── */
#titlebar {
  background: linear-gradient(to right, #000080, #1084d0);
  color: #fff;
  font-size: 12px;
  font-weight: bold;
  padding: 3px 4px;
  display: flex;
  align-items: center;
  gap: 4px;
  user-select: none;
}
#titlebar .title-icon { font-size: 14px; }
#titlebar .title-text { flex: 1; letter-spacing: 0.5px; }
.wbtn {
  width: 16px; height: 14px;
  background: #c0c0c0;
  border-top: 1px solid #ffffff;
  border-left: 1px solid #ffffff;
  border-right: 1px solid #404040;
  border-bottom: 1px solid #404040;
  font-size: 9px;
  color: #000;
  display: flex; align-items: center; justify-content: center;
  cursor: default;
  font-weight: bold;
  line-height: 1;
}

/* ── Menu bar ────────────────────────────────────── */
#menubar {
  background: #c0c0c0;
  border-bottom: 1px solid #808080;
  padding: 1px 4px;
  font-size: 11px;
  display: flex; gap: 2px;
}
.menu-item {
  padding: 2px 8px;
  cursor: default;
}
.menu-item:hover {
  background: #000080;
  color: #fff;
}

/* ── Content area ────────────────────────────────── */
#content {
  padding: 6px;
  overflow-y: auto;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
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
  font-size: 10px;
  font-weight: bold;
  padding: 2px 6px;
  letter-spacing: 0.5px;
}
.panel-body { padding: 6px 8px; }

/* ── Stats ───────────────────────────────────────── */
.stats-row { display: flex; gap: 6px; flex-wrap: wrap; }
.stat-box {
  background: #c0c0c0;
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  padding: 6px 18px;
  min-width: 100px;
  text-align: center;
}
.stat-label { font-size: 10px; color: #444; text-transform: uppercase; letter-spacing: 1px; }
.stat-val { font-size: 22px; font-family: "Courier New", monospace; font-weight: bold; }
.stat-total .stat-val  { color: #000080; }
.stat-pending .stat-val  { color: #996600; }
.stat-success .stat-val  { color: #006600; }
.stat-exhausted .stat-val { color: #880000; }

/* ── Filter form ─────────────────────────────────── */
.filter-row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.filter-row label { font-size: 11px; font-weight: bold; white-space: nowrap; }
select, input[type=text] {
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  background: #fff;
  font-size: 11px;
  padding: 1px 3px;
  font-family: "MS Sans Serif", Tahoma, Verdana, sans-serif;
}
input[type=text] { width: 240px; }
.btn {
  background: #c0c0c0;
  border-top: 2px solid #ffffff;
  border-left: 2px solid #ffffff;
  border-right: 2px solid #808080;
  border-bottom: 2px solid #808080;
  font-size: 11px;
  padding: 1px 14px;
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
  font-size: 11px;
  cursor: pointer;
  padding: 1px 4px;
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
  font-size: 11px;
}
thead tr { background: #000080; color: #fff; }
thead th {
  padding: 3px 6px;
  text-align: left;
  font-size: 10px;
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
  padding: 2px 6px;
  border-bottom: 1px solid #d8d8d8;
  border-right: 1px solid #e8e8e8;
  font-family: "Courier New", monospace;
  vertical-align: top;
  white-space: nowrap;
  overflow: hidden;
  max-width: 200px;
}
td.url { max-width: 260px; }
td.err { max-width: 240px; }
td.center { text-align: center; max-width: 60px; }

/* ── Badge ───────────────────────────────────────── */
.badge {
  display: inline-block;
  padding: 0px 5px;
  font-size: 10px;
  font-weight: bold;
  font-family: "MS Sans Serif", Tahoma, Verdana, sans-serif;
  white-space: nowrap;
  border: 1px solid;
}
.badge-pending   { background: #ffff99; color: #664400; border-color: #ccaa00; }
.badge-success   { background: #ccffcc; color: #004400; border-color: #008800; }
.badge-exhausted { background: #ffcccc; color: #660000; border-color: #cc0000; }

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
  font-size: 11px;
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: 2px 8px;
}
#detail-pane .dk { font-weight: bold; color: #000080; white-space: nowrap; }
#detail-pane .dv { word-break: break-all; }
#detail-close {
  float: right;
  margin: 3px 6px;
}

/* ── Pagination ──────────────────────────────────── */
.pager { display: flex; align-items: center; gap: 6px; font-size: 11px; margin-top: 4px; }
.pager a {
  color: #000080;
  text-decoration: none;
  background: #c0c0c0;
  border-top: 2px solid #ffffff;
  border-left: 2px solid #ffffff;
  border-right: 2px solid #808080;
  border-bottom: 2px solid #808080;
  padding: 1px 10px;
}
.pager a:hover { background: #b0b0e0; }
.pager .cur { font-weight: bold; color: #000080; }

.no-data {
  text-align: center;
  padding: 20px;
  color: #666;
  font-style: italic;
}

/* ── Status bar ──────────────────────────────────── */
#statusbar {
  border-top: 1px solid #808080;
  background: #c0c0c0;
  padding: 2px 6px;
  display: flex;
  gap: 8px;
  font-size: 11px;
}
#statusbar .sb-cell {
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  padding: 1px 6px;
}

/* ── Taskbar ─────────────────────────────────────── */
#taskbar {
  background: #c0c0c0;
  border-top: 2px solid #ffffff;
  padding: 2px 4px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.taskbar-start {
  background: #c0c0c0;
  border-top: 2px solid #ffffff;
  border-left: 2px solid #ffffff;
  border-right: 2px solid #808080;
  border-bottom: 2px solid #808080;
  font-size: 12px;
  font-weight: bold;
  padding: 1px 8px 1px 4px;
  cursor: default;
}
.taskbar-win {
  background: #c0c0c0;
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  font-size: 11px;
  padding: 1px 10px;
  cursor: default;
}
.taskbar-clock {
  margin-left: auto;
  border-top: 1px solid #808080;
  border-left: 1px solid #808080;
  border-right: 1px solid #ffffff;
  border-bottom: 1px solid #ffffff;
  padding: 1px 8px;
  font-family: "Courier New", monospace;
  font-size: 11px;
}
</style>
</head>
<body>

<div id="window">

  <!-- Title bar -->
  <div id="titlebar">
    <span class="title-icon">&#128274;</span>
    <span class="title-text">Delivery Dashboard — Webhook Engine</span>
    <div class="wbtn">_</div>
    <div class="wbtn">&#9633;</div>
    <div class="wbtn" style="color:#800000;font-size:10px;">&#10005;</div>
  </div>

  <!-- Menu bar -->
  <div id="menubar">
    <span class="menu-item"><u>F</u>ile</span>
    <span class="menu-item"><u>V</u>iew</span>
    <span class="menu-item"><u>H</u>elp</span>
  </div>

  <!-- Content -->
  <div id="content">

    <!-- Stats -->
    <div class="panel">
      <div class="panel-title">&#9632; System Statistics</div>
      <div class="panel-body">
        <div class="stats-row">
          <div class="stat-box stat-total">
            <div class="stat-label">Total</div>
            <div class="stat-val">{{ total .Stats }}</div>
          </div>
          <div class="stat-box stat-pending">
            <div class="stat-label">Pending</div>
            <div class="stat-val">{{ statCount .Stats "pending" }}</div>
          </div>
          <div class="stat-box stat-success">
            <div class="stat-label">Success</div>
            <div class="stat-val">{{ statCount .Stats "success" }}</div>
          </div>
          <div class="stat-box stat-exhausted">
            <div class="stat-label">Exhausted</div>
            <div class="stat-val">{{ statCount .Stats "exhausted" }}</div>
          </div>
          <div class="stat-box" style="min-width:auto; padding: 6px 12px; display:flex; flex-direction:column; justify-content:center;">
            <label style="display:flex;align-items:center;gap:4px;cursor:pointer;">
              <input type="checkbox" id="autorefresh" onchange="toggleRefresh(this)">
              <span>Auto-refresh 30s</span>
            </label>
          </div>
        </div>
      </div>
    </div>

    <!-- Filter -->
    <div class="panel">
      <div class="panel-title">&#9658; Filter</div>
      <div class="panel-body">
        <form method="get" action="/" class="filter-row">
          <label>Status:</label>
          <select name="status">
            <option value=""{{ if eq .Status "" }} selected{{ end }}>All</option>
            <option value="pending"{{ if eq .Status "pending" }} selected{{ end }}>pending</option>
            <option value="success"{{ if eq .Status "success" }} selected{{ end }}>success</option>
            <option value="exhausted"{{ if eq .Status "exhausted" }} selected{{ end }}>exhausted</option>
          </select>
          <label>Event ID:</label>
          <input type="text" name="event_id" value="{{ .EventID }}" placeholder="evt_01H...">
          <button type="submit" class="btn">&#9654; Apply</button>
          {{ if or (ne .Status "") (ne .EventID "") }}
          <button type="button" class="btn-clear" onclick="location.href='/'">&#10005; Clear</button>
          {{ end }}
        </form>
      </div>
    </div>

    <!-- Detail pane (hidden by default) -->
    <div class="panel" id="detail-pane">
      <div class="panel-title">
        &#9658; Delivery Details
        <button class="btn" id="detail-close" onclick="closeDetail()" style="float:right;margin:-1px 0 0 0;padding:0 6px;font-size:10px;">&#10005; Close</button>
      </div>
      <div class="panel-body" id="detail-body"></div>
    </div>

    <!-- Table -->
    <div class="panel" style="flex:1; display:flex; flex-direction:column; min-height:0;">
      <div class="panel-title">
        &#9658; Deliveries
        {{ if ne .Status "" }}&nbsp;[{{ .Status }}]{{ end }}
        {{ if ne .EventID "" }}&nbsp;event={{ .EventID }}{{ end }}
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
              <th>#</th>
              <th>Next Attempt</th>
              <th>Last Error</th>
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
              data-created="{{ fmtTime .CreatedAt }}"
              data-updated="{{ fmtTime .UpdatedAt }}"
            >
              <td title="{{ .ID }}">{{ trunc .ID 20 }}</td>
              <td title="{{ .EventID }}">{{ trunc .EventID 20 }}</td>
              <td title="{{ .SubscriptionID }}">{{ trunc .SubscriptionID 16 }}</td>
              <td class="url" title="{{ .DestinationURL }}">{{ trunc .DestinationURL 38 }}</td>
              <td class="center">{{ .Method }}</td>
              <td class="center"><span class="badge badge-{{ statusClass .Status }}">{{ .Status }}</span></td>
              <td class="center">{{ .Attempts }}</td>
              <td>{{ fmtTimePtr .NextAttempt }}</td>
              <td class="err" title="{{ .LastError }}">{{ trunc .LastError 38 }}</td>
              <td>{{ fmtTime .CreatedAt }}</td>
              <td>{{ fmtTime .UpdatedAt }}</td>
            </tr>
            {{ end }}
          </tbody>
        </table>
        </div>

        <div class="pager">
          {{ if .HasPrev }}
          <a href="{{ buildURL .Status .EventID (sub .Page 1) }}">&#9664; Prev</a>
          {{ end }}
          <span class="cur">Page {{ .Page }}</span>
          {{ if .HasNext }}
          <a href="{{ buildURL .Status .EventID (add .Page 1) }}">Next &#9654;</a>
          {{ end }}
        </div>

        {{ else }}
        <div class="no-data">[ no records found ]</div>
        {{ end }}

      </div>
    </div>

  </div><!-- /content -->

  <!-- Status bar -->
  <div id="statusbar">
    <span class="sb-cell" id="sb-status">Ready</span>
    <span class="sb-cell">delivery-dashboard v1.0</span>
  </div>

</div><!-- /window -->

<!-- Taskbar -->
<div id="taskbar">
  <button class="taskbar-start">&#9658; Start</button>
  <div class="taskbar-win">&#128274; Delivery Dashboard</div>
  <div class="taskbar-clock" id="clock">--:--:--</div>
</div>

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

// Auto-refresh
var refreshTimer = null;
function toggleRefresh(cb) {
  if (cb.checked) {
    refreshTimer = setTimeout(function(){ location.reload(); }, 30000);
    document.getElementById('sb-status').textContent = 'Auto-refresh in 30s...';
  } else {
    clearTimeout(refreshTimer);
    document.getElementById('sb-status').textContent = 'Ready';
  }
}

// Row detail
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

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}
</script>
</body>
</html>`
