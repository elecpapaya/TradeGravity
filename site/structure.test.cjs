const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const html = fs.readFileSync(path.join(__dirname, "index.html"), "utf8");

test("index loads trusted helpers before the application and keeps D3 pinned", () => {
  const securityIndex = html.indexOf('src="./security.js"');
  const dataToolsIndex = html.indexOf('src="./data-tools.js"');
  const explorerToolsIndex = html.indexOf('src="./explorer-tools.js"');
  const d3Index = html.indexOf('src="https://cdn.jsdelivr.net/npm/d3@7.9.0/dist/d3.min.js"');
  const appIndex = html.indexOf('src="./app.js"');
  assert.ok(securityIndex >= 0 && dataToolsIndex > securityIndex && explorerToolsIndex > dataToolsIndex && d3Index > explorerToolsIndex && appIndex > d3Index);
  assert.match(html, /integrity="sha384-[A-Za-z0-9+/=]+"/);
  assert.match(html, /Content-Security-Policy/);
});

test("accessible table controls and targets appear exactly once", () => {
  for (const id of [
    "dataTableTitle",
    "tableSearch",
    "downloadCSV",
    "downloadJSON",
    "tableSummary",
    "tradeTableBody",
    "usaMetricHeader",
    "chnMetricHeader",
    "combinedMetricHeader",
  ]) {
    assert.equal((html.match(new RegExp(`id=["']${id}["']`, "g")) || []).length, 1, `expected one #${id}`);
  }
  assert.match(html, /<caption class="srOnly">/);
  assert.match(html, /aria-sort="descending"/);
  assert.match(html, /href="#dataTableTitle"/);
});

test("advanced explorer controls and analysis targets appear exactly once", () => {
  for (const id of [
    "periodFilter", "comparisonMode", "regionFilter", "incomeFilter", "groupFilter",
    "normalization", "copyShareURL", "timeSeries", "products", "qualityDashboard", "explanation",
  ]) {
    assert.equal((html.match(new RegExp(`id=["']${id}["']`, "g")) || []).length, 1, `expected one #${id}`);
  }
});

test("static markup has no inline event handlers and protects new tabs", () => {
  assert.doesNotMatch(html, /<[^>]+\son[a-z]+\s*=/i);
  for (const match of html.matchAll(/<a\b[^>]*target="_blank"[^>]*>/gi)) {
    assert.match(match[0], /rel="[^"]*noopener[^"]*"/i);
  }
});
