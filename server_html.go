package main

import (
	"fmt"
	"html"
)

const pageCSS = `
:root {
  --bg: #eef2f6;
  --card: #ffffff;
  --ink: #1f2937;
  --muted: #6b7280;
  --line: #dbe1ea;
  --accent: #0e7490;
  --ok: #047857;
}
*, *::before, *::after { box-sizing: border-box; }
html { color-scheme: light; }
body {
  margin: 0;
  font: 16px/1.45 system-ui, -apple-system, "Segoe UI", sans-serif;
  background: var(--bg);
  color: var(--ink);
}
.wrap { max-width: 980px; margin: 0 auto; padding: 1.25rem 1rem 2.5rem; }
header.app {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.75rem 1.5rem;
  align-items: baseline;
  margin-bottom: 1.25rem;
}
header.app h1 { margin: 0; font-size: 1.2rem; font-weight: 650; letter-spacing: -0.02em; }
nav.links { display: flex; flex-wrap: wrap; gap: 0.85rem; }
nav.links a { color: var(--accent); text-decoration: none; font-weight: 600; font-size: 0.92rem; }
nav.links a:hover { text-decoration: underline; }
.pills { display: flex; flex-wrap: wrap; gap: 0.4rem; margin: 0 0 1rem; }
.pill {
  border-radius: 999px;
  padding: 0.2rem 0.7rem;
  font-size: 0.8rem;
  font-weight: 600;
  background: #e5e7eb;
  color: #374151;
}
.pill.on { background: #d1fae5; color: var(--ok); }
.metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(148px, 1fr));
  gap: 0.75rem;
  margin-bottom: 1rem;
}
.card {
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 0.9rem 1rem;
}
.metric .label {
  font-size: 0.72rem;
  font-weight: 650;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
}
.metric .value {
  margin-top: 0.15rem;
  font-size: 1.55rem;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  margin: 0 0 1rem;
  font-size: 0.9rem;
  color: var(--muted);
}
.toolbar input {
  font: inherit;
  width: 6.5rem;
  padding: 0.35rem 0.5rem;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--card);
  color: var(--ink);
}
.chart img {
  display: block;
  width: 100%;
  height: auto;
  border-radius: 8px;
  background: #fff;
}
.legend, .updated { font-size: 0.8rem; color: var(--muted); }
.updated { margin-top: 0.75rem; }
h2 { font-size: 1.05rem; margin: 0 0 0.75rem; }
p.lead { color: var(--muted); margin: 0 0 1rem; }
form.stack label {
  display: grid;
  gap: 0.25rem;
  margin: 0 0 0.75rem;
  font-size: 0.88rem;
  font-weight: 600;
}
form.stack label.check {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
}
/* Matches on exclusion: configRow emits inputs with no type attribute,
   which input[type=text] would not select. */
form.stack input:not([type=checkbox]):not([type=submit]) {
  font: inherit;
  font-weight: 400;
  padding: 0.4rem 0.55rem;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #fff;
  color: var(--ink);
}
form.stack fieldset {
  border: 1px solid var(--line);
  border-radius: 12px;
  background: var(--card);
  margin: 0 0 1rem;
  padding: 0.9rem 1rem 0.4rem;
}
form.stack legend { font-weight: 650; padding: 0 0.35rem; }
form.stack .hint { font-weight: 400; color: var(--muted); font-size: 0.8rem; }
input[type=submit], button {
  font: inherit;
  font-weight: 650;
  background: var(--accent);
  color: #fff;
  border: 0;
  border-radius: 8px;
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.pin { font-size: 1.4rem; letter-spacing: 0.12em; font-variant-numeric: tabular-nums; }
.qr { display: block; margin: 0.75rem 0 0; border-radius: 8px; }
`

const liveRefreshScript = `<script>
(function () {
  function bumpGraphs() {
    document.querySelectorAll(".chart img").forEach(function (img) {
      var u = new URL(img.src, location.origin);
      u.searchParams.set("_", Date.now().toString());
      img.src = u.toString();
    });
  }
  function setPill(id, label, value, on) {
    var el = document.getElementById(id);
    if (!el) return;
    el.className = on ? "pill on" : "pill";
    el.textContent = label + ": " + value;
  }
  function setText(id, value) {
    var el = document.getElementById(id);
    if (el) el.textContent = value;
  }
  function bumpStatus() {
    fetch("/status", { cache: "no-store" }).then(function (r) {
      if (!r.ok) throw new Error("status " + r.status);
      return r.json();
    }).then(function (s) {
      setPill("pill-pump", "Pump", s.pump, s.pump_on);
      setPill("pill-solar", "Solar", s.solar, s.solar_on);
      setPill("pill-thermostat", "Thermostat", s.thermostat, s.thermostat !== "Off");
      setPill("pill-control", "Control", s.control, s.control === "Manual");
      setPill("pill-sensors", "Sensors", s.sensor_info, s.sensors_ok);
      setText("metric-target", s.target_f.toFixed(1) + " °F");
      setText("metric-pool", s.pool_f.toFixed(1) + " °F");
      setText("metric-roof", s.roof_f.toFixed(1) + " °F");
      setText("updated", "Updated " + s.updated);
    }).catch(function () {});
  }
  setInterval(bumpGraphs, 20000);
  setInterval(bumpStatus, 5000);
})();
</script>
`

func page(title, body string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<title>` + html.EscapeString(title) + `</title>
<style>` + pageCSS + `</style>
</head>
<body>
<div class="wrap">
<header class="app">
<h1>Pool controller</h1>
` + nav() + `
</header>
` + body + `
</div>
</body>
</html>`
}

func nav() string {
	return `<nav class="links">
<a href="/">status</a>
<a href="/pair">homekit</a>
<a href="/calibrate">calibrate</a>
<a href="/config">config</a>
</nav>`
}

func metricCard(id, label, value string) string {
	return `<div class="card metric"><div class="label">` + html.EscapeString(label) +
		`</div><div class="value" id="` + html.EscapeString(id) + `">` + html.EscapeString(value) + `</div></div>`
}

func statusPill(id, label, value string, on bool) string {
	class := "pill"
	if on {
		class += " on"
	}
	return `<span id="` + html.EscapeString(id) + `" class="` + class + `">` + html.EscapeString(label+": "+value) + `</span>`
}

func image(which string, width, height int, scale string) string {
	alt := "Temperature history"
	if which == "pumps" {
		alt = "Pump and solar activity"
	}
	q := html.EscapeString(scale)
	return fmt.Sprintf(
		`<img src="/%s?scale=%s&amp;width=%d&amp;height=%d" width="%d" height="%d" alt="%s">`,
		which, q, width, height, width, height, alt)
}
