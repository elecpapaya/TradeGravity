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
  const semiconductorToolsIndex = html.indexOf('src="./semiconductor-tools.js"');
  const experienceToolsIndex = html.indexOf('src="./experience-tools.js"');
  const newsToolsIndex = html.indexOf('src="./news-tools.js"');
  const briefingToolsIndex = html.indexOf('src="./briefing-tools.js"');
  const d3Index = html.indexOf('src="https://cdn.jsdelivr.net/npm/d3@7.9.0/dist/d3.min.js"');
  const appIndex = html.indexOf('src="./app.js"');
  assert.ok(securityIndex >= 0 && dataToolsIndex > securityIndex && explorerToolsIndex > dataToolsIndex && intelligenceToolsIndex > explorerToolsIndex && semiconductorToolsIndex > intelligenceToolsIndex && experienceToolsIndex > semiconductorToolsIndex && newsToolsIndex > experienceToolsIndex && briefingToolsIndex > newsToolsIndex && d3Index > briefingToolsIndex && appIndex > d3Index);
  assert.match(html, /integrity="sha384-[A-Za-z0-9+/=]+"/);
  assert.match(html, /Content-Security-Policy/);
});

test("the experimental news panel exposes scope and trust caveats", () => {
  assert.match(app, /Trade &amp; supply-chain headlines/);
  assert.match(app, /publisher source country/);
  assert.match(app, /does not affect trade metrics/i);
  assert.match(app, /keyword-filtered and deduplicated/i);
  assert.match(app, /temporarily unavailable from GDELT/i);
  assert.match(app, /Different clocks/);
  assert.match(app, /They are not same-period evidence/);
});

test("first-visit onboarding, inline definitions, and export actions are wired once", () => {
  for (const id of [
    "dataHealthBanner", "dataHealthBadge", "dataHealthText", "retryData",
    "metricContext", "metricContextDefinition", "periodContext", "filterContext", "filterContextDetail", "scopeContext", "scopeContextLimit",
    "openOnboarding", "openMethodology", "exportPNG", "exportCSV", "exportReport",
    "onboardingDialog", "methodologyDialog", "dismissOnboarding", "startSampleView",
  ]) {
    assert.equal((html.match(new RegExp(`id=["']${id}["']`, "g")) || []).length, 1, `expected one #${id}`);
  }
  assert.match(html, /30-second orientation/);
  assert.match(html, /Pipeline refresh and recent headlines use different dates/);
  assert.match(html, /Each anchor divided by USA plus China in this two-anchor view/);
  assert.match(app, /tradegravity:onboarding:v1/);
  assert.match(app, /function startVietNamSample/);
  assert.match(app, /function downloadPNGSnapshot/);
  assert.match(app, /function downloadSummaryReport/);
  assert.match(app, /async function copyTextToClipboard/);
  assert.match(app, /document\.execCommand\("copy"\)/);
});

test("data health and mobile layout expose degraded states without hiding recovery", () => {
  assert.match(app, /function renderDataHealth/);
  assert.match(app, /renderDataHealth\(false\)/);
  assert.match(app, /retryData.*window\.location\.reload/);
  assert.match(app, /document\.documentElement\.clientWidth \|\| window\.innerWidth/);
  assert.match(app, /document\.documentElement\.clientHeight \|\| window\.innerHeight/);
  assert.match(css, /\.dataHealthBanner\.is-partial/);
  assert.match(css, /\.dataHealthBanner\.is-failed/);
  assert.match(css, /@media \(max-width: 650px\)[\s\S]*\.viewActions[\s\S]*grid-template-columns:repeat\(2/);
  assert.match(css, /@media \(max-width: 420px\)[\s\S]*\.viewActions[\s\S]*grid-template-columns:1fr/);
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
    "dashboardTabs", "tab-overview", "tab-intelligence", "tab-semiconductors", "tab-products", "tab-quality", "tab-lab",
    "intelligenceSummary", "networkChart", "exposureRankingBody", "dataCatalog",
    "mirrorDiagnostics",
    "scenarioForm", "scenarioPartner", "scenarioProduct", "scenarioTariffSource", "scenarioResult",
    "strategicSectorFilter", "strategicProducts", "strategicCapabilityStatus", "tariffCapabilityStatus",
  ]) {
    assert.equal((html.match(new RegExp(`id=["']${id}["']`, "g")) || []).length, 1, `expected one #${id}`);
  }
  assert.equal((html.match(/role="tab"/g) || []).length, 6);
  assert.equal((html.match(/role="tabpanel"/g) || []).length, 6);
  assert.match(html, /not an inferred physical supply-chain route/i);
  assert.match(html, /does not estimate GDP/i);
});

test("chip lens exposes coverage, stages, roles, monthly signals, policy, evidence, and transparent sensitivity", () => {
  for (const id of [
    "chipCoverageBadge", "chipCoverageSummary", "chipStageFilter", "chipCountryFilter", "chipDownloadCSV",
	"chipPublicationChanges",
    "chipTrends", "chipValueChain", "chipRoleLandscape", "chipDistribution", "chipCountryProfile", "chipTimeline", "chipCapacitySignals",
    "chipMonthlySignals",
    "chipBriefingStatus", "chipBriefingSignals", "briefingDownloadEmail", "briefingDownloadCarousel", "briefingCopyLink", "briefingDeliveryNote",
    "chipScenarioForm", "chipDisruption", "chipSubstitution", "chipScenarioBaseline", "chipScenarioResult",
    "chipSources", "chipCaveats",
  ]) {
    assert.equal((html.match(new RegExp(`id=["']${id}["']`, "g")) || []).length, 1, `expected one #${id}`);
  }
  assert.match(html, /Customs observations, context, and estimates remain separate/i);
  assert.match(html, /not global supplier share or production capacity/i);
  assert.match(html, /open\/public data only/i);
  assert.match(app, /function loadSemiconductorPartitions/);
  assert.match(app, /async function loadSelectedChipMonthly/);
  assert.match(app, /async function renderMirrorDiagnostics/);
  assert.match(app, /function runChipScenario/);
  assert.match(app, /function renderDistributionBriefing/);
  assert.match(app, /function downloadBriefingEmail/);
  assert.match(app, /function downloadBriefingCarousel/);
  assert.match(css, /\.chipValueChain/);
  assert.match(css, /\.briefingSignalGrid/);
  assert.match(css, /\.policyTimeline/);
  assert.match(css, /\.chipRoleTable/);
  assert.match(html, /announcements are never counted as operating capacity/i);
  assert.match(html, /does not collect subscriber data or publish directly to social platforms/i);
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
