const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const html = fs.readFileSync(path.join(__dirname, "index.html"), "utf8");
const app = fs.readFileSync(path.join(__dirname, "app.js"), "utf8");
const css = fs.readFileSync(path.join(__dirname, "styles.css"), "utf8");

test("index loads trusted helpers before the application and keeps D3 pinned", () => {
  const securityIndex = html.indexOf('src="./security.js"');
  const dataToolsIndex = html.indexOf('src="./data-tools.js"');
  const explorerToolsIndex = html.indexOf('src="./explorer-tools.js"');
  const intelligenceToolsIndex = html.indexOf('src="./intelligence-tools.js"');
  const d3Index = html.indexOf('src="https://cdn.jsdelivr.net/npm/d3@7.9.0/dist/d3.min.js"');
  const appIndex = html.indexOf('src="./app.js"');
  assert.ok(securityIndex >= 0 && dataToolsIndex > securityIndex && explorerToolsIndex > dataToolsIndex && intelligenceToolsIndex > explorerToolsIndex && d3Index > intelligenceToolsIndex && appIndex > d3Index);
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
  assert.match(html, /href="#mainContent"/);
});

test("dashboard tabs and scalable intelligence targets appear exactly once", () => {
  for (const id of [
    "dashboardTabs", "tab-overview", "tab-intelligence", "tab-products", "tab-quality", "tab-lab",
    "intelligenceSummary", "networkChart", "exposureRankingBody", "dataCatalog",
    "scenarioForm", "scenarioPartner", "scenarioProduct", "scenarioTariffSource", "scenarioResult",
    "strategicSectorFilter", "strategicProducts", "strategicCapabilityStatus", "tariffCapabilityStatus",
  ]) {
    assert.equal((html.match(new RegExp(`id=["']${id}["']`, "g")) || []).length, 1, `expected one #${id}`);
  }
  assert.equal((html.match(/role="tab"/g) || []).length, 5);
  assert.equal((html.match(/role="tabpanel"/g) || []).length, 5);
  assert.match(html, /not an inferred physical supply-chain route/i);
  assert.match(html, /does not estimate GDP/i);
});

test("overview treemaps survive hidden-tab resizes and redraw when shown", () => {
  const treemapStart = app.indexOf("function buildTreemap");
  const treemapSource = app.slice(treemapStart, treemapStart + 500);
  const measureIndex = treemapSource.indexOf("getBoundingClientRect");
  const visibilityGuardIndex = treemapSource.indexOf("if (width <= 0 || height <= 0) return");
  const clearIndex = treemapSource.indexOf('svg.selectAll("*").remove()');
  assert.ok(treemapStart >= 0 && measureIndex >= 0, "expected treemap dimension measurement");
  assert.ok(visibilityGuardIndex > measureIndex, "expected a hidden-SVG dimension guard");
  assert.ok(clearIndex > visibilityGuardIndex, "expected existing tiles to survive hidden-tab renders");

  const tabStart = app.indexOf("function setActiveTab");
  const tabSource = app.slice(tabStart, tabStart + 1400);
  assert.match(tabSource, /state\.tab === "overview"/);
  assert.match(tabSource, /requestAnimationFrame/);
  assert.match(tabSource, /buildTreemap\(els\.svgUSA/);
  assert.match(tabSource, /buildTreemap\(els\.svgCHN/);
});

test("treemap labels use compact values and stay clipped to their tiles", () => {
  const treemapStart = app.indexOf("function buildTreemap");
  const treemapEnd = app.indexOf("function currentViewState", treemapStart);
  const treemapSource = app.slice(treemapStart, treemapEnd);
  assert.match(app, /function formatCompactMetricValue/);
  assert.match(treemapSource, /formatCompactMetricValue\(d\.data\.value\)/);
  assert.match(treemapSource, /width >= value\.length \* 7 \+ 12/);
  assert.equal((treemapSource.match(/\.attr\("clip-path"/g) || []).length, 3);
});

test("treemap focus styling stays on the tile rectangle", () => {
  assert.match(css, /\.tile:focus\{outline:none\}/);
  assert.match(css, /\.tile:focus-visible > rect\{[^}]*stroke:/);
  assert.doesNotMatch(css, /\.tile:focus-visible[^{]*\{[^}]*outline/);
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
