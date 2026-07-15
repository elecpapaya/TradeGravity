// Treemap MVP (v4): two anchor rectangles (USA/CHN).
// Inside each: sub-rectangles per country sized by selected metric (trade/export/import).
// Adds optional flag icons via CDN.
// - If row.iso2 exists, use it.
// - Else try ISO3->ISO2 map (partial).
// If no ISO2 found, skip flag.

const DATA_URL = "./data/latest.json";
const META_URL = "./data/meta.json";
const SERIES_URL = "./data/series.json";
const QUALITY_URL = "./data/quality.json";
const PRODUCTS_INDEX_URL = "./data/products/index.json";
const STRATEGIC_INDEX_URL = "./data/strategic-hs6/index.json";
const TARIFF_INDEX_URL = "./data/tariffs/index.json";
const MATRIX_INDEX_URL = "./data/bilateral-matrix/index.json";
const CATALOG_URL = "./data/catalog.json";
const security = globalThis.TradeGravitySecurity;
if (!security) {
  throw new Error("TradeGravity security helpers failed to load.");
}
const dataTools = globalThis.TradeGravityDataTools;
if (!dataTools) {
  throw new Error("TradeGravity data helpers failed to load.");
}
const explorerTools = globalThis.TradeGravityExplorerTools;
if (!explorerTools) {
  throw new Error("TradeGravity explorer helpers failed to load.");
}
const intelligenceTools = globalThis.TradeGravityIntelligenceTools;
if (!intelligenceTools) {
  throw new Error("TradeGravity intelligence helpers failed to load.");
}
const experienceTools = globalThis.TradeGravityExperienceTools;
if (!experienceTools) {
  throw new Error("TradeGravity experience helpers failed to load.");
}
const newsTools = globalThis.TradeGravityNewsTools;
if (!newsTools) {
  throw new Error("TradeGravity news helpers failed to load.");
}
const { encodeCSV, escapeHTML, normalizeISO2, normalizeISO3, safeHTTPSURL } = security;
const { buildCSVMatrix: createCSVMatrix } = dataTools;
const {
  availablePeriods,
  buildFilteredJSON,
  deriveRowsForPeriod,
  filterExplorerRows,
  normalizedMetricValue,
  parseViewState,
  serializeViewState,
} = explorerTools;
const {
  buildAnchorNetwork,
	buildPartnerNetwork,
  buildIntelligenceProfile,
  estimateTariffScenario,
  rankExposureRows,
  selectPreferredTariffs,
} = intelligenceTools;
const {
  buildSummaryReport,
  deriveDataHealth,
  metricDefinition,
  tabLimitation,
} = experienceTools;
const {
  DEFAULT_MAX_ITEMS: NEWS_MAX,
  DEFAULT_WINDOW_DAYS: NEWS_WINDOW_DAYS,
  buildGdeltURL,
  curateNewsArticles,
} = newsTools;

const els = {
  svgUSA: document.getElementById("svg-usa"),
  svgCHN: document.getElementById("svg-chn"),
  metric: document.getElementById("metric"),
  metricGroup: document.getElementById("metricGroup"),
  colorGroup: document.getElementById("colorGroup"),
  selection: document.getElementById("selection"),
  indicators: document.getElementById("indicators"),
  tooltip: document.getElementById("tooltip"),
  topN: document.getElementById("topN"),
  growthLegend: document.getElementById("growthLegend"),
  dataStatus: document.getElementById("dataStatus"),
  sourceLink: document.getElementById("sourceLink"),
  tableSearch: document.getElementById("tableSearch"),
  downloadCSV: document.getElementById("downloadCSV"),
  downloadJSON: document.getElementById("downloadJSON"),
  tableSummary: document.getElementById("tableSummary"),
  tableBody: document.getElementById("tradeTableBody"),
  usaMetricHeader: document.getElementById("usaMetricHeader"),
  chnMetricHeader: document.getElementById("chnMetricHeader"),
  combinedMetricHeader: document.getElementById("combinedMetricHeader"),
  periodFilter: document.getElementById("periodFilter"),
  comparisonMode: document.getElementById("comparisonMode"),
  regionFilter: document.getElementById("regionFilter"),
  incomeFilter: document.getElementById("incomeFilter"),
  groupFilter: document.getElementById("groupFilter"),
  normalization: document.getElementById("normalization"),
  copyShareURL: document.getElementById("copyShareURL"),
  timeSeries: document.getElementById("timeSeries"),
  products: document.getElementById("products"),
  qualityDashboard: document.getElementById("qualityDashboard"),
  explanation: document.getElementById("explanation"),
  dashboardTabs: document.getElementById("dashboardTabs"),
  intelligenceSummary: document.getElementById("intelligenceSummary"),
  networkChart: document.getElementById("networkChart"),
	networkTitle: document.getElementById("networkTitle"),
	networkDescription: document.getElementById("networkDescription"),
	networkNote: document.getElementById("networkNote"),
	intelligenceScopeBadge: document.getElementById("intelligenceScopeBadge"),
  exposureRankingBody: document.getElementById("exposureRankingBody"),
  productAvailability: document.getElementById("productAvailability"),
  strategicSectorFilter: document.getElementById("strategicSectorFilter"),
  strategicProducts: document.getElementById("strategicProducts"),
  strategicCapabilityStatus: document.getElementById("strategicCapabilityStatus"),
  strategicCapabilityText: document.getElementById("strategicCapabilityText"),
  tariffCapabilityStatus: document.getElementById("tariffCapabilityStatus"),
  tariffCapabilityText: document.getElementById("tariffCapabilityText"),
  dataCatalog: document.getElementById("dataCatalog"),
  scenarioForm: document.getElementById("scenarioForm"),
  scenarioPartner: document.getElementById("scenarioPartner"),
  scenarioProduct: document.getElementById("scenarioProduct"),
  scenarioTariffBase: document.getElementById("scenarioTariffBase"),
  scenarioTariffChange: document.getElementById("scenarioTariffChange"),
  scenarioElasticity: document.getElementById("scenarioElasticity"),
  scenarioPassThrough: document.getElementById("scenarioPassThrough"),
  scenarioTariffSource: document.getElementById("scenarioTariffSource"),
  scenarioResult: document.getElementById("scenarioResult"),
  dataHealthBanner: document.getElementById("dataHealthBanner"),
  dataHealthBadge: document.getElementById("dataHealthBadge"),
  dataHealthText: document.getElementById("dataHealthText"),
  retryData: document.getElementById("retryData"),
  metricContext: document.getElementById("metricContext"),
  metricContextDefinition: document.getElementById("metricContextDefinition"),
  periodContext: document.getElementById("periodContext"),
  filterContext: document.getElementById("filterContext"),
  filterContextDetail: document.getElementById("filterContextDetail"),
  scopeContext: document.getElementById("scopeContext"),
  scopeContextLimit: document.getElementById("scopeContextLimit"),
  openOnboarding: document.getElementById("openOnboarding"),
  openMethodology: document.getElementById("openMethodology"),
  exportPNG: document.getElementById("exportPNG"),
  exportCSV: document.getElementById("exportCSV"),
  exportReport: document.getElementById("exportReport"),
  onboardingDialog: document.getElementById("onboardingDialog"),
  methodologyDialog: document.getElementById("methodologyDialog"),
  methodologyCurrentView: document.getElementById("methodologyCurrentView"),
  dismissOnboarding: document.getElementById("dismissOnboarding"),
  startSampleView: document.getElementById("startSampleView"),
};

let state = {
  latestRows: [],
  rows: [],
  seriesRows: [],
  quality: null,
  catalog: null,
  productIndex: null,
  strategicIndex: null,
  tariffIndex: null,
	matrixIndex: null,
  productCache: {},
  strategicCache: {},
  tariffCache: {},
	matrixCache: {},
	matrixPromises: {},
  explanationCache: {},
  metric: "trade",
  colorMode: "value",
  highlightKey: null, // ISO3
  selectedRow: null,
  topN: 25,
  meta: null,
  tableQuery: "",
  schemaVersion: "",
  generatedAt: "",
  provider: "",
  period: "latest",
  comparisonMode: "comparable",
  region: "",
  income: "",
  group: "",
  normalization: "raw",
  tab: "overview",
  strategicSector: "all",
  resourceStates: [],
  dataHealth: null,
  preserveScenarioInputs: false,
};

const ONBOARDING_STORAGE_KEY = "tradegravity:onboarding:v1";

// Minimal ISO3->ISO2 fallback map (overridden by iso3_to_iso2.json if present).
const FALLBACK_ISO3_TO_ISO2 = {
  KOR:"KR", JPN:"JP", CHN:"CN", USA:"US", DEU:"DE", FRA:"FR", GBR:"GB", ITA:"IT", ESP:"ES",
  CAN:"CA", MEX:"MX", BRA:"BR", IND:"IN", IDN:"ID", VNM:"VN", AUS:"AU", RUS:"RU", TUR:"TR",
  SAU:"SA", ARE:"AE", ZAF:"ZA", EGY:"EG", NGA:"NG", ARG:"AR", CHL:"CL", COL:"CO", PER:"PE",
  NLD:"NL", BEL:"BE", SWE:"SE", NOR:"NO", DNK:"DK", FIN:"FI", POL:"PL", CZE:"CZ", HUN:"HU",
  ISR:"IL", IRL:"IE", PRT:"PT", CHE:"CH", AUT:"AT", GRC:"GR", UKR:"UA", THA:"TH", MYS:"MY",
  SGP:"SG", PHL:"PH", PAK:"PK", BGD:"BD", NZL:"NZ", KAZ:"KZ"
};
let ISO3_TO_ISO2 = { ...FALLBACK_ISO3_TO_ISO2 };
const INDICATORS = [
  { id: "NY.GDP.MKTP.CD", label: "GDP (current US$)", format: "usd" },
  { id: "NY.GDP.PCAP.CD", label: "GDP per capita (US$)", format: "usd" },
  { id: "SP.POP.TOTL", label: "Population", format: "number" },
];
const indicatorCache = {};
const indicatorPromises = {};
const newsCache = {};
const newsPromises = {};

const GROWTH_COLORS = {
  neg: "#ff7b6b",
  zero: "#2b323c",
  pos: "#86e7b0",
  missing: "rgba(255,255,255,.06)"
};

const growthScale = d3.scaleLinear()
  .domain([-0.5, 0, 0.5])
  .range([GROWTH_COLORS.neg, GROWTH_COLORS.zero, GROWTH_COLORS.pos])
  .clamp(true);

function iso2FromRow(row){
  const iso2 = normalizeISO2(row.iso2 || row.ISO2);
  if (iso2) return iso2;
  const iso3 = normalizeISO3(row.iso3 || row.ISO3);
  return normalizeISO2(ISO3_TO_ISO2[iso3]);
}

// Flag CDN URL. Uses PNG (20px height).
// Docs: https://flagcdn.com/
function flagURL(iso2){
  const normalized = normalizeISO2(iso2);
  if (!normalized) return "";
  const cc = normalized.toLowerCase();
  return `https://flagcdn.com/h20/${cc}.png`;
}

let regionNames = null;
try {
  regionNames = new Intl.DisplayNames(["en"], { type: "region" });
} catch {
  // Older browsers fall back to the ISO3 label.
}

function displayCountryName(iso2, fallback){
  const normalized = normalizeISO2(iso2);
  if (normalized && regionNames) {
    const name = regionNames.of(normalized);
    if (name && name !== normalized) return name;
  }
  return String(fallback || "").trim().slice(0, 100);
}

function formatPipelineTime(value){
  const parsed = new Date(value);
  if (!Number.isFinite(parsed.getTime())) return "unknown refresh time";
  return new Intl.DateTimeFormat("en", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "UTC",
  }).format(parsed) + " UTC";
}

function calculateCoverage(rows){
  const periodCounts = new Map();
  let available = 0;
  for (const row of rows) {
    for (const side of ["usa", "chn"]) {
      const block = row[side] || {};
      if (!block.period) continue;
      available++;
      const key = `${block.period_type || "?"}:${block.period}`;
      periodCounts.set(key, (periodCounts.get(key) || 0) + 1);
    }
  }
  return {
    available,
    expected: rows.length * 2,
    periodCounts: Object.fromEntries(periodCounts),
  };
}

function periodSummary(periodCounts){
  const entries = Object.entries(periodCounts || {});
  if (entries.length === 0) return "periods unavailable";
  return entries
    .sort((a, b) => b[0].localeCompare(a[0]))
    .map(([key, count]) => `${key.replace(":", " ")} (${count})`)
    .join(", ");
}

function renderDatasetStatus(latest, metadata){
  if (!els.dataStatus) return;
  const fallback = calculateCoverage(state.rows);
  const provider = String(metadata?.provider || latest?.provider || "unknown").trim().toLowerCase();
  const available = Number(metadata?.available_partner_blocks ?? fallback.available);
  const expected = Number(metadata?.expected_partner_blocks ?? fallback.expected);
  const periods = metadata?.period_counts || fallback.periodCounts;
  const generatedAt = metadata?.generated_at || latest?.generated_at;

  els.dataStatus.textContent = `Pipeline refreshed ${formatPipelineTime(generatedAt)} · ${provider.toUpperCase()} · coverage ${available}/${expected} partner blocks · observations ${periodSummary(periods)}`;

  if (els.sourceLink) {
    const sources = {
      wits: "https://wits.worldbank.org/",
      comtrade: "https://comtradeplus.un.org/",
    };
    els.sourceLink.href = sources[provider] || "https://wits.worldbank.org/";
    els.sourceLink.textContent = provider === "unknown" ? "Data source" : `${provider.toUpperCase()} source`;
  }
}

function renderDataHealth(coreReady = true){
  const health = deriveDataHealth({
    coreReady,
    metadata: state.meta || { generated_at: state.generatedAt, provider: state.provider },
    quality: state.quality,
    resources: state.resourceStates,
    generatedAt: state.generatedAt,
  });
  state.dataHealth = health;
  if (!els.dataHealthBanner) return health;
  els.dataHealthBanner.classList.remove("is-loading", "is-current", "is-partial", "is-failed");
  els.dataHealthBanner.classList.add(`is-${health.level}`);
  if (els.dataHealthBadge) els.dataHealthBadge.textContent = health.label;
  if (els.dataHealthText) {
    els.dataHealthText.textContent = health.summary;
    els.dataHealthText.title = health.details.join(" ");
  }
  if (els.retryData) els.retryData.hidden = health.level === "current";
  return health;
}

function activePeriodLabel(){
  if (state.period && state.period !== "latest") return state.period.replace(":", " ");
  const periods = new Set();
  for (const row of state.rows) {
    for (const side of ["usa", "chn"]) {
      const block = row?.[side];
      if (block?.period) periods.add(`${block.period_type || "?"} ${block.period}`);
    }
  }
  if (periods.size === 0) return "Latest by reporter · period unavailable";
  if (periods.size === 1) return `Latest by reporter · ${Array.from(periods)[0]}`;
  return `Latest by reporter · ${periods.size} observation periods`;
}

function activeFilterLabel(){
  const filters = [];
  if (state.group) filters.push(`group ${state.group}`);
  if (state.region) filters.push(`region ${state.region}`);
  if (state.income) filters.push(`income ${state.income}`);
  if (state.tableQuery) filters.push(`search “${state.tableQuery}”`);
  return filters.length > 0 ? filters.join(" · ") : "All reporters";
}

function activeTabLabel(){
  return {
    overview: "Overview",
    intelligence: "Intelligence",
    products: "Products",
    quality: "Data & Quality",
    lab: "Scenario Lab",
  }[state.tab] || "Overview";
}

function renderViewContext(){
  const definition = metricDefinition(state.metric, state.normalization);
  const limit = tabLimitation(state.tab);
  if (els.metricContext) els.metricContext.textContent = metricLabel();
  if (els.metricContextDefinition) els.metricContextDefinition.textContent = definition;
  if (els.periodContext) els.periodContext.textContent = activePeriodLabel();
  if (els.filterContext) els.filterContext.textContent = activeFilterLabel();
  if (els.filterContextDetail) {
    const comparison = state.comparisonMode === "comparable" ? "Same-period comparison" : "All available periods";
    const country = state.selectedRow ? `${state.selectedRow.name} selected` : "no country selected";
    els.filterContextDetail.textContent = `${state.rows.length} reporters · ${comparison} · ${country}.`;
  }
  if (els.scopeContext) els.scopeContext.textContent = activeTabLabel();
  if (els.scopeContextLimit) els.scopeContextLimit.textContent = limit;
  if (els.methodologyCurrentView) {
    els.methodologyCurrentView.innerHTML = `<b>Active view:</b> ${escapeHTML(metricLabel())} · ${escapeHTML(activePeriodLabel())}. ${escapeHTML(definition)} ${escapeHTML(limit)}`;
  }
}

function fmt(n){
  if (n == null || !isFinite(n)) return "0";
  const abs = Math.abs(n);
  if (abs >= 1e12) return (n/1e12).toFixed(2) + "T";
  if (abs >= 1e9) return (n/1e9).toFixed(2) + "B";
  if (abs >= 1e6) return (n/1e6).toFixed(2) + "M";
  if (abs >= 1e3) return (n/1e3).toFixed(2) + "K";
  return String(Math.round(n));
}

function fmtPct(value){
  if (value == null || !isFinite(value)) return "-";
  const pct = value * 100;
  const sign = pct > 0 ? "+" : "";
  return sign + pct.toFixed(1) + "%";
}

function toNullableNumber(value){
  if (value == null) return null;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function toFiniteNumber(value, fallback = 0){
  if (value == null || value === "") return fallback;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function normalizeGrowth(value){
  if (!value || typeof value !== "object") return null;
  return {
    export: toNullableNumber(value.export),
    import: toNullableNumber(value.import),
    trade: toNullableNumber(value.trade),
  };
}

function growthBasisLabel(value){
  const basis = (value?.growth_basis || "yoy").toUpperCase();
  return basis === "YOY" ? "YoY" : basis;
}

function getGrowthValue(row, side){
  const o = row[side] || {};
  const g = o.growth || {};
  const value = g[state.metric];
  return toNullableNumber(value);
}

function growthColor(value){
  if (value == null || !isFinite(value)) return GROWTH_COLORS.missing;
  return growthScale(value);
}

function getMetricValue(row, side){
  return normalizedMetricValue(row, side, state.metric, state.normalization) ?? 0;
}

function normalizePartnerBlock(value){
  const block = value && typeof value === "object" ? value : {};
  const exportValue = toFiniteNumber(block.export);
  const importValue = toFiniteNumber(block.import);
  const tradeValue = toFiniteNumber(block.trade, exportValue + importValue);
  return {
    period: String(block.period || "").trim().slice(0, 16),
    period_type: String(block.period_type || "").trim().slice(0, 1),
    prev_period: String(block.prev_period || "").trim().slice(0, 16),
    export: exportValue,
    import: importValue,
    trade: tradeValue,
    growth: normalizeGrowth(block.growth),
    growth_basis: String(block.growth_basis || "").trim().slice(0, 16),
  };
}

function normalizeRows(rows){
  return (rows || []).map(r => {
    const iso3 = normalizeISO3(r.iso3 || r.ISO3);
    if (!iso3) return null;
    const usa = normalizePartnerBlock(r.usa);
    const chn = normalizePartnerBlock(r.chn);
    const total = toFiniteNumber(r.total, usa.trade + chn.trade);
    const share_cn = toFiniteNumber(r.share_cn, total ? chn.trade/total : 0);
    const iso2 = iso2FromRow(r);
    return {
      iso3,
      name: String(r.name || displayCountryName(iso2, iso3)).trim().slice(0, 100),
      iso2,
      region: String(r.region || "").trim().slice(0, 100),
      income_group: String(r.income_group || "").trim().slice(0, 100),
      groups: Array.isArray(r.groups) ? r.groups.map(value => String(value).trim().slice(0, 30)).filter(Boolean) : [],
      population: r.population && typeof r.population === "object" ? r.population : { value: null, year: "" },
      gdp: r.gdp && typeof r.gdp === "object" ? r.gdp : { value: null, year: "" },
      usa,
      chn,
      total,
      share_cn,
      same_period: Object.prototype.hasOwnProperty.call(r, "same_period")
        ? Boolean(r.same_period)
        : Boolean(usa.period && usa.period === chn.period && usa.period_type === chn.period_type),
      comparison_period: String(r.comparison_period || (usa.period === chn.period ? usa.period : "")).trim().slice(0, 16),
    };
  }).filter(Boolean);
}

const exactNumberFormatter = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

function metricLabel(){
  const base = {
    trade: "total trade",
    export: "exports",
    import: "imports",
  }[state.metric] || state.metric;
  if (state.normalization === "per_capita") return `${base} per person`;
  if (state.normalization === "gdp_share") return `${base} as share of GDP`;
  return base;
}

function filteredTableRows(){
  return state.rows.slice().sort((a, b) => {
    const right = getMetricValue(b, "usa") + getMetricValue(b, "chn");
    const left = getMetricValue(a, "usa") + getMetricValue(a, "chn");
    return right - left || a.iso3.localeCompare(b.iso3);
  });
}

function formatMetricValue(value){
  if (value == null || !Number.isFinite(Number(value))) return "—";
  if (state.normalization === "gdp_share") return `${(Number(value) * 100).toFixed(2)}%`;
  if (state.normalization === "per_capita") return `$${Number(value).toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
  return exactNumberFormatter.format(Number(value));
}

function formatCompactMetricValue(value){
  if (value == null || !Number.isFinite(Number(value))) return "—";
  if (state.normalization === "gdp_share") return `${(Number(value) * 100).toFixed(2)}%`;
  if (state.normalization === "per_capita") return `$${fmt(Number(value))}`;
  return fmt(Number(value));
}

function appendTableCell(tableRow, value, className = "", title = ""){
  const cell = document.createElement("td");
  cell.textContent = String(value ?? "");
  if (className) cell.className = className;
  if (title) cell.title = title;
  tableRow.appendChild(cell);
  return cell;
}

function selectCountry(row){
  if (!row || row.iso3 === "OTH") return;
  state.selectedRow = row;
  state.highlightKey = row.iso3;
  syncURL();
  renderViewContext();
  applyHighlight(row.iso3);
  setSelection(row);
  setIndicators(row);
  renderTimeSeries();
  renderProducts();
  renderStrategicProducts();
  renderExplanation();
  renderDataTable();
  renderIntelligence();
  renderScenarioBaseline();
}

function formatNominalUSD(value){
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  return `${number < 0 ? "−" : ""}$${fmt(Math.abs(number))}`;
}

function renderDataTable(){
  if (!els.tableBody) return;
  const label = metricLabel();
  if (els.usaMetricHeader) els.usaMetricHeader.textContent = `USA ${label}`;
  if (els.chnMetricHeader) els.chnMetricHeader.textContent = `CHN ${label}`;
  if (els.combinedMetricHeader) els.combinedMetricHeader.textContent = `Combined ${label}`;

  const rows = filteredTableRows();
  const fragment = document.createDocumentFragment();
  for (const row of rows) {
    const tableRow = document.createElement("tr");
    if (state.selectedRow?.iso3 === row.iso3) tableRow.classList.add("is-selected");

    const reporterCell = document.createElement("td");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "tableCountryButton";
    button.title = `Open details for ${row.name}`;
    button.addEventListener("click", () => selectCountry(row));
    const name = document.createElement("span");
    name.textContent = row.name;
    const iso = document.createElement("span");
    iso.className = "iso";
    iso.textContent = row.iso3;
    button.append(name, iso);
    reporterCell.appendChild(button);
    tableRow.appendChild(reporterCell);

    appendTableCell(tableRow, row.usa.period || "—", "", row.usa.period_type || "");
    const usaValue = getMetricValue(row, "usa");
    appendTableCell(tableRow, formatMetricValue(usaValue), "numeric", String(usaValue));
    appendTableCell(tableRow, row.chn.period || "—", "", row.chn.period_type || "");
    const chnValue = getMetricValue(row, "chn");
    appendTableCell(tableRow, formatMetricValue(chnValue), "numeric", String(chnValue));
    const combined = usaValue + chnValue;
    appendTableCell(tableRow, formatMetricValue(combined), "numeric", String(combined));
    appendTableCell(tableRow, combined > 0 ? `${(chnValue / combined * 100).toFixed(1)}%` : "—", "numeric");
    fragment.appendChild(tableRow);
  }
  els.tableBody.replaceChildren(fragment);
  if (els.tableSummary) {
    const mode = state.comparisonMode === "comparable" ? "same-period only" : "all periods";
    els.tableSummary.textContent = `${rows.length} reporters · ${mode} · sorted by combined ${label}`;
  }
}

function downloadTableCSV(){
  const rows = filteredTableRows();
  const matrix = createCSVMatrix(rows, {
    schemaVersion: state.schemaVersion,
    generatedAt: state.generatedAt,
    provider: state.provider,
  });
  const csv = encodeCSV(matrix);
  const blob = new Blob(["\uFEFF", csv], { type: "text/csv;charset=utf-8" });
  const objectURL = URL.createObjectURL(blob);
  const link = document.createElement("a");
  const date = String(state.generatedAt || "").match(/^\d{4}-\d{2}-\d{2}/)?.[0] || "latest";
  link.href = objectURL;
  link.download = `tradegravity-${date}.csv`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(objectURL), 0);
}

function downloadFilteredJSON(){
  const filters = currentViewState();
  const payload = buildFilteredJSON(filteredTableRows(), {
    schemaVersion: state.schemaVersion,
    generatedAt: state.generatedAt,
    provider: state.provider,
    filters,
  });
  const blob = new Blob([JSON.stringify(payload, null, 2), "\n"], { type: "application/json;charset=utf-8" });
  const objectURL = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = objectURL;
  link.download = `tradegravity-filtered-${state.period.replace(/[^A-Za-z0-9-]/g, "-")}.json`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(objectURL), 0);
}

function downloadBlob(blob, filename){
  const objectURL = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = objectURL;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(objectURL), 0);
}

function exportFilename(extension){
  const country = state.selectedRow?.iso3 ? `-${state.selectedRow.iso3.toLowerCase()}` : "";
  const period = String(state.period || "latest").replace(/[^A-Za-z0-9-]/g, "-").toLowerCase();
  return `tradegravity-${state.tab}-${period}${country}.${extension}`;
}

function reportModel(){
  syncURL();
  const rows = filteredTableRows();
  const selected = state.selectedRow;
  const selectedUSA = selected ? getMetricValue(selected, "usa") : 0;
  const selectedCHN = selected ? getMetricValue(selected, "chn") : 0;
  const comparisonQuality = selected?.same_period
    ? `Same period (${selected.comparison_period || selected.usa?.period || selected.chn?.period || "unknown"})`
    : "Mixed or missing periods";
  return {
    exportedAt: new Date().toISOString(),
    generatedAt: state.generatedAt,
    provider: state.provider,
    tabLabel: activeTabLabel(),
    metricLabel: metricLabel(),
    periodLabel: activePeriodLabel(),
    comparisonLabel: state.comparisonMode === "comparable" ? "Same-period only" : "All available periods",
    filterLabel: activeFilterLabel(),
    metricDefinition: metricDefinition(state.metric, state.normalization),
    health: state.dataHealth || renderDataHealth(true),
    selected: selected ? {
      name: selected.name,
      iso3: selected.iso3,
      usaValue: formatMetricValue(selectedUSA),
      chnValue: formatMetricValue(selectedCHN),
      combinedValue: formatMetricValue(selectedUSA + selectedCHN),
      usaPeriod: selected.usa?.period || "",
      chnPeriod: selected.chn?.period || "",
      chinaShare: selectedUSA + selectedCHN > 0 ? `${(selectedCHN / (selectedUSA + selectedCHN) * 100).toFixed(1)}%` : "—",
      comparisonQuality,
    } : null,
    topRows: rows.slice(0, 10).map(row => {
      const usa = getMetricValue(row, "usa");
      const chn = getMetricValue(row, "chn");
      return {
        name: row.name,
        iso3: row.iso3,
        usaValue: formatMetricValue(usa),
        chnValue: formatMetricValue(chn),
        combinedValue: formatMetricValue(usa + chn),
        periodQuality: row.same_period ? `Same period (${row.comparison_period || row.usa?.period || row.chn?.period || "unknown"})` : "Mixed/missing",
      };
    }),
    limit: tabLimitation(state.tab),
    viewURL: window.location.href,
  };
}

function downloadSummaryReport(){
  const report = buildSummaryReport(reportModel());
  downloadBlob(new Blob([report], { type: "text/markdown;charset=utf-8" }), exportFilename("md"));
}

function copyFormState(source, clone){
  const sourceFields = source.querySelectorAll("input, select, textarea");
  const cloneFields = clone.querySelectorAll("input, select, textarea");
  sourceFields.forEach((field, index) => {
    const target = cloneFields[index];
    if (!target) return;
    if (target.tagName === "SELECT") {
      Array.from(target.options).forEach(option => option.toggleAttribute("selected", option.value === field.value));
    } else {
      target.setAttribute("value", field.value);
      target.textContent = field.value;
    }
  });
}

function snapshotStyleText(){
  let styles = "";
  for (const sheet of Array.from(document.styleSheets)) {
    try {
      styles += Array.from(sheet.cssRules || []).map(rule => rule.cssText).join("\n");
    } catch {
      // Cross-origin styles are not required for the first-party dashboard snapshot.
    }
  }
  return styles;
}

function imageFromDataURL(url){
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("Snapshot image could not be rendered."));
    image.src = url;
  });
}

function canvasBlob(canvas){
  return new Promise((resolve, reject) => {
    canvas.toBlob(blob => blob ? resolve(blob) : reject(new Error("PNG encoding failed.")), "image/png");
  });
}

function wrapCanvasText(context, text, x, y, maxWidth, lineHeight, maxLines = 3){
  const words = String(text || "").split(/\s+/).filter(Boolean);
  let line = "";
  let lineCount = 0;
  for (const word of words) {
    const candidate = line ? `${line} ${word}` : word;
    if (context.measureText(candidate).width > maxWidth && line) {
      context.fillText(line, x, y + lineCount * lineHeight);
      line = word;
      lineCount++;
      if (lineCount >= maxLines) return y + lineCount * lineHeight;
    } else {
      line = candidate;
    }
  }
  if (line && lineCount < maxLines) {
    context.fillText(line, x, y + lineCount * lineHeight);
    lineCount++;
  }
  return y + lineCount * lineHeight;
}

async function fallbackSnapshotBlob(){
  const model = reportModel();
  const canvas = document.createElement("canvas");
  canvas.width = 1600;
  canvas.height = 1000;
  const ctx = canvas.getContext("2d");
  ctx.fillStyle = "#0b0d12";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  const gradient = ctx.createLinearGradient(0, 0, canvas.width, 0);
  gradient.addColorStop(0, "rgba(90,162,255,.16)");
  gradient.addColorStop(1, "rgba(255,107,87,.10)");
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, canvas.width, 180);
  ctx.fillStyle = "#f2f4f8";
  ctx.font = "700 38px system-ui, sans-serif";
  ctx.fillText("TradeGravity", 70, 68);
  ctx.font = "700 25px system-ui, sans-serif";
  ctx.fillText(`${model.tabLabel} · ${model.metricLabel}`, 70, 112);
  ctx.fillStyle = "rgba(255,255,255,.67)";
  ctx.font = "18px system-ui, sans-serif";
  ctx.fillText(`${model.periodLabel} · ${model.comparisonLabel} · ${model.filterLabel}`, 70, 148);

  ctx.fillStyle = "#e7d37c";
  ctx.font = "700 15px ui-monospace, monospace";
  ctx.fillText((model.health.label || "DATA STATUS").toUpperCase(), 70, 224);
  ctx.fillStyle = "rgba(255,255,255,.68)";
  ctx.font = "17px system-ui, sans-serif";
  wrapCanvasText(ctx, model.health.summary, 70, 254, 1460, 24, 2);

  let top = 330;
  if (model.selected) {
    ctx.fillStyle = "#f2f4f8";
    ctx.font = "700 26px system-ui, sans-serif";
    ctx.fillText(`${model.selected.name} (${model.selected.iso3})`, 70, top);
    ctx.font = "700 22px ui-monospace, monospace";
    ctx.fillStyle = "#5aa2ff";
    ctx.fillText(`USA ${model.selected.usaValue}`, 70, top + 42);
    ctx.fillStyle = "#ff7b6b";
    ctx.fillText(`CHN ${model.selected.chnValue}`, 430, top + 42);
    ctx.fillStyle = "#e7d37c";
    ctx.fillText(`Combined ${model.selected.combinedValue}`, 790, top + 42);
    top += 105;
  }

  ctx.fillStyle = "rgba(255,255,255,.76)";
  ctx.font = "700 17px system-ui, sans-serif";
  ctx.fillText("Leading reporters in the filtered view", 70, top);
  top += 35;
  ctx.font = "15px ui-monospace, monospace";
  for (const [index, row] of model.topRows.slice(0, 8).entries()) {
    const y = top + index * 54;
    ctx.fillStyle = "rgba(255,255,255,.1)";
    ctx.fillRect(70, y - 22, 1460, 42);
    ctx.fillStyle = "rgba(255,255,255,.88)";
    ctx.fillText(`${String(index + 1).padStart(2, "0")}  ${row.name} (${row.iso3})`, 88, y + 5);
    ctx.fillStyle = "#5aa2ff";
    ctx.fillText(`USA ${row.usaValue}`, 700, y + 5);
    ctx.fillStyle = "#ff7b6b";
    ctx.fillText(`CHN ${row.chnValue}`, 1010, y + 5);
    ctx.fillStyle = "#e7d37c";
    ctx.fillText(row.combinedValue, 1360, y + 5);
  }
  ctx.fillStyle = "rgba(255,255,255,.55)";
  ctx.font = "15px system-ui, sans-serif";
  wrapCanvasText(ctx, model.limit, 70, 930, 1460, 21, 2);
  return canvasBlob(canvas);
}

async function activeViewSnapshotBlob(){
  const panel = document.querySelector(`[data-tab-panel="${state.tab}"]`);
  if (!panel) return fallbackSnapshotBlob();
  const stage = document.createElement("div");
  stage.setAttribute("xmlns", "http://www.w3.org/1999/xhtml");
  stage.style.cssText = "position:fixed;left:-100000px;top:0;width:1440px;background:#0b0d12;color:rgba(255,255,255,.92);overflow:hidden;";
  const style = document.createElement("style");
  style.textContent = `${snapshotStyleText()} body{overflow:visible!important}.tabPanel{display:block!important}.viewUtility{grid-template-columns:1fr!important}.viewActions{display:none!important}`;
  stage.appendChild(style);
  const brand = document.querySelector(".brand")?.cloneNode(true);
  if (brand) {
    const header = document.createElement("header");
    header.className = "topbar";
    header.appendChild(brand);
    stage.appendChild(header);
  }
  const health = els.dataHealthBanner?.cloneNode(true);
  if (health) {
    health.querySelectorAll("button").forEach(button => button.remove());
    stage.appendChild(health);
  }
  const context = document.querySelector(".viewUtility")?.cloneNode(true);
  if (context) {
    context.querySelector(".viewActions")?.remove();
    stage.appendChild(context);
  }
  const panelClone = panel.cloneNode(true);
  panelClone.removeAttribute("hidden");
  copyFormState(panel, panelClone);
  panelClone.querySelectorAll("img, image").forEach(image => image.remove());
  stage.appendChild(panelClone);
  document.body.appendChild(stage);
  try {
    if (document.fonts?.ready) await document.fonts.ready;
    const width = 1440;
    const height = Math.max(720, Math.min(2400, stage.scrollHeight));
    stage.style.cssText = `position:relative;left:0;top:0;width:${width}px;height:${height}px;background:#0b0d12;color:rgba(255,255,255,.92);overflow:hidden;`;
    const serialized = new XMLSerializer().serializeToString(stage);
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}"><foreignObject width="100%" height="100%">${serialized}</foreignObject></svg>`;
    const image = await imageFromDataURL(`data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`);
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#0b0d12";
    ctx.fillRect(0, 0, width, height);
    ctx.drawImage(image, 0, 0);
    return await canvasBlob(canvas);
  } catch (error) {
    console.warn("[TradeGravity] active-view PNG capture fell back to summary snapshot.", error);
    return fallbackSnapshotBlob();
  } finally {
    stage.remove();
  }
}

async function downloadPNGSnapshot(){
  if (!els.exportPNG) return;
  const original = els.exportPNG.textContent;
  els.exportPNG.disabled = true;
  els.exportPNG.textContent = "Preparing PNG…";
  try {
    const blob = await activeViewSnapshotBlob();
    downloadBlob(blob, exportFilename("png"));
    els.exportPNG.textContent = "PNG ready";
  } catch (error) {
    console.error(error);
    els.exportPNG.textContent = "PNG failed";
  } finally {
    setTimeout(() => {
      els.exportPNG.disabled = false;
      els.exportPNG.textContent = original;
    }, 1400);
  }
}

function rememberOnboarding(){
  try { localStorage.setItem(ONBOARDING_STORAGE_KEY, "seen"); } catch { /* Storage can be unavailable in privacy modes. */ }
}

function onboardingWasSeen(){
  try { return localStorage.getItem(ONBOARDING_STORAGE_KEY) === "seen"; } catch { return true; }
}

function openAppDialog(dialog){
  if (!dialog || dialog.open) return;
  if (typeof dialog.showModal === "function") dialog.showModal();
  else dialog.setAttribute("open", "");
}

function closeAppDialog(dialog){
  if (!dialog?.open) return;
  if (dialog === els.onboardingDialog) rememberOnboarding();
  if (typeof dialog.close === "function") dialog.close();
  else dialog.removeAttribute("open");
}

function startVietNamSample(){
  const periods = availablePeriods(state.seriesRows).map(item => item.key);
  state.period = periods.includes("Y:2023") ? "Y:2023" : "latest";
  const hasASEAN = state.latestRows.some(row => (row.groups || []).includes("ASEAN"));
  state.group = hasASEAN ? "ASEAN" : "";
  state.region = "";
  state.income = "";
  state.tableQuery = "";
  state.comparisonMode = "comparable";
  state.normalization = "raw";
  syncExplorerControls();
  refreshRows({ syncURL: false });
  const sample = state.rows.find(row => row.iso3 === "VNM") || state.rows[0] || null;
  setActiveTab("overview", { syncURL: false });
  if (sample) selectCountry(sample);
  else syncURL();
  rememberOnboarding();
  closeAppDialog(els.onboardingDialog);
}

function initializeExperienceControls(){
  els.openOnboarding?.addEventListener("click", () => openAppDialog(els.onboardingDialog));
  els.openMethodology?.addEventListener("click", () => {
    renderViewContext();
    openAppDialog(els.methodologyDialog);
  });
  els.dismissOnboarding?.addEventListener("click", () => {
    rememberOnboarding();
    closeAppDialog(els.onboardingDialog);
  });
  els.startSampleView?.addEventListener("click", startVietNamSample);
  document.querySelectorAll("[data-close-dialog]").forEach(button => {
    button.addEventListener("click", () => closeAppDialog(document.getElementById(button.dataset.closeDialog)));
  });
  for (const dialog of [els.onboardingDialog, els.methodologyDialog]) {
    dialog?.addEventListener("click", event => {
      if (event.target === dialog) closeAppDialog(dialog);
    });
  }
  els.exportPNG?.addEventListener("click", downloadPNGSnapshot);
  els.exportCSV?.addEventListener("click", downloadTableCSV);
  els.exportReport?.addEventListener("click", downloadSummaryReport);
  els.retryData?.addEventListener("click", () => window.location.reload());
  if (!onboardingWasSeen()) setTimeout(() => openAppDialog(els.onboardingDialog), 250);
}

function setSelection(row){
  if (!row){
    els.selection.innerHTML = "<span class='subtle'>Click a country tile to view details.</span>";
    return;
  }
  const us = row.usa || {};
  const cn = row.chn || {};
  const name = escapeHTML(row.name);
  const iso3 = escapeHTML(row.iso3);
  const flag = flagURL(row.iso2);
  const metric = escapeHTML(state.metric);
  const usaMetricValue = formatMetricValue(getMetricValue(row, "usa"));
  const chnMetricValue = formatMetricValue(getMetricValue(row, "chn"));
  const combinedMetricValue = formatMetricValue(getMetricValue(row, "usa") + getMetricValue(row, "chn"));
  const comparability = row.same_period
    ? `Same period (${escapeHTML(row.comparison_period || us.period || cn.period || "-")})`
    : "Mixed or missing periods";
  const html = `
    <div style="font-weight:800; margin-bottom:8px; display:flex; align-items:center; gap:10px;">
      ${flag ? `<img alt="Flag of ${name}" src="${flag}" style="width:22px;height:16px;border-radius:4px;border:1px solid rgba(255,255,255,.12)"/>` : ""}
      <div>${name} <span style="color:rgba(255,255,255,.55); font-family:var(--mono); font-size:12px;">(${iso3})</span></div>
    </div>
    <div class="kv"><span>USA period</span><b>${escapeHTML(us.period || "-")}</b></div>
    <div class="kv"><span>USA ${metric}</span><b>${usaMetricValue}</b></div>
    <div class="kv"><span>USA prev period</span><b>${escapeHTML(us.prev_period || "-")}</b></div>
    <div class="kv"><span>USA growth (${escapeHTML(growthBasisLabel(us))})</span><b>${fmtPct(getGrowthValue(row, "usa"))}</b></div>
    <div style="height:8px"></div>
    <div class="kv"><span>CHN period</span><b>${escapeHTML(cn.period || "-")}</b></div>
    <div class="kv"><span>CHN ${metric}</span><b>${chnMetricValue}</b></div>
    <div class="kv"><span>CHN prev period</span><b>${escapeHTML(cn.prev_period || "-")}</b></div>
    <div class="kv"><span>CHN growth (${escapeHTML(growthBasisLabel(cn))})</span><b>${fmtPct(getGrowthValue(row, "chn"))}</b></div>
    <div style="height:10px"></div>
    <div class="kv"><span>China share of total trade</span><b>${(row.share_cn*100).toFixed(1)}%</b></div>
    <div class="kv"><span>USA + CHN selected metric</span><b>${combinedMetricValue}</b></div>
    <div class="kv"><span>Comparison quality</span><b>${comparability}</b></div>
  `;
  els.selection.innerHTML = html;
}

function showTooltip(ev, row, side){
  const o = row[side] || {};
  const name = escapeHTML(row.name);
  const iso3 = escapeHTML(row.iso3);
  const flag = flagURL(row.iso2);
  const sideLabel = escapeHTML(side.toUpperCase());
  const metric = escapeHTML(state.metric);
  els.tooltip.style.display = "block";
  els.tooltip.innerHTML = `
    <div class="t1" style="display:flex;align-items:center;gap:10px;">
      ${flag ? `<img alt="Flag of ${name}" src="${flag}" style="width:20px;height:14px;border-radius:4px;border:1px solid rgba(255,255,255,.10)"/>` : ""}
      <div>${name} <span style="color:rgba(255,255,255,.55); font-family:var(--mono); font-size:12px;">(${iso3})</span></div>
    </div>
    <div class="t2">
      <div class="kv"><span>${sideLabel} period</span><b>${escapeHTML(o.period || "-")}</b></div>
      <div class="kv"><span>${sideLabel} ${metric}</span><b>${formatMetricValue(getMetricValue(row, side))}</b></div>
      <div class="kv"><span>${sideLabel} prev</span><b>${escapeHTML(o.prev_period || "-")}</b></div>
      <div class="kv"><span>${sideLabel} growth (${escapeHTML(growthBasisLabel(o))})</span><b>${fmtPct(getGrowthValue(row, side))}</b></div>
      <div class="kv"><span>China share of total trade</span><b>${(row.share_cn*100).toFixed(1)}%</b></div>
    </div>
  `;
  const pad = 12;
  const rect = els.tooltip.getBoundingClientRect();
  const viewportWidth = document.documentElement.clientWidth || window.innerWidth;
  const viewportHeight = document.documentElement.clientHeight || window.innerHeight;
  const x = Math.max(8, Math.min(viewportWidth - rect.width - 8, ev.clientX + pad));
  const y = Math.max(8, Math.min(viewportHeight - rect.height - 8, ev.clientY + pad));
  els.tooltip.style.left = x + "px";
  els.tooltip.style.top = y + "px";
}

function hideTooltip(){
  els.tooltip.style.display = "none";
}

function applyHighlight(key){
  state.highlightKey = key;
  d3.selectAll("[data-iso3]").classed("is-hi", d => {
    const k = d?.data?.iso3 ?? d?.iso3;
    return key && k === key;
  });
  d3.selectAll(".tile").classed("is-dim", d => {
    const k = d?.data?.iso3 ?? d?.iso3;
    return key && k !== key;
  });
}

function buildTreemap(svgEl, side, rows){
  const svg = d3.select(svgEl);
  const { width, height } = svgEl.getBoundingClientRect();
  if (width <= 0 || height <= 0) return;

  svg.selectAll("*").remove();
  svg.attr("viewBox", `0 0 ${width} ${height}`);

  const rawChildren = rows.map(r => ({
    iso3: r.iso3,
    name: r.name,
    iso2: r.iso2,
    value: Math.max(0, getMetricValue(r, side)),
    row: r
  })).filter(d => d.value > 0);

  // Top N + Others grouping (by value for the selected metric/side)
  const topN = Math.max(5, Math.min(200, state.topN || 25));
  rawChildren.sort((a,b) => (b.value||0) - (a.value||0));

  const top = rawChildren.slice(0, topN);
  const rest = rawChildren.slice(topN);

  let children = top;
  if (rest.length > 0){
    const othersValue = rest.reduce((s,d)=>s+(d.value||0), 0);
    children = top.concat([{
      iso3: "OTH",
      name: `Others (${rest.length})`,
      iso2: "",
      value: othersValue,
      row: { iso3:"OTH", name:`Others (${rest.length})`, iso2:"", usa:{}, chn:{}, total: othersValue, share_cn: 0 }
    }]);
  }

  const root = d3.hierarchy({ name: side, children })
    .sum(d => d.value)
    .sort((a,b) => (b.value||0) - (a.value||0));

  const layout = d3.treemap()
    .size([width, height])
    .paddingInner(2)
    .paddingOuter(6)
    .round(true);

  layout(root);

  const labelSet = new Set(children.map(d => d.iso3));

  const baseFill = side === "usa"
    ? "rgba(90,162,255,.18)"
    : "rgba(255,107,87,.18)";
  const stroke = side === "usa"
    ? "rgba(90,162,255,.35)"
    : "rgba(255,107,87,.35)";
  const useGrowthColor = state.colorMode === "growth";

  const defs = svg.append("defs");

  const g = svg.append("g");

  const nodes = g.selectAll("g.tile")
    .data(root.leaves())
    .enter()
    .append("g")
    .attr("class","tile")
    .attr("data-iso3", d => d.data.iso3)
    .attr("tabindex", 0)
    .attr("role", "button")
    .attr("aria-label", d => `${d.data.name}, ${side.toUpperCase()} ${metricLabel()} ${formatMetricValue(d.data.value)}`)
    .attr("transform", d => `translate(${d.x0},${d.y0})`);

  // Clip path per tile so flag doesn't spill out
  nodes.each(function(d){
    const id = `clip-${side}-${d.data.iso3}`;
    defs.append("clipPath")
      .attr("id", id)
      .append("rect")
      .attr("rx", 6)
      .attr("ry", 6)
      .attr("x", 0)
      .attr("y", 0)
      .attr("width", Math.max(0, d.x1 - d.x0))
      .attr("height", Math.max(0, d.y1 - d.y0));
    d.__clipId = id;
  });

  nodes.append("rect")
    .attr("rx", 6)
    .attr("ry", 6)
    .attr("width", d => Math.max(0, d.x1 - d.x0))
    .attr("height", d => Math.max(0, d.y1 - d.y0))
    .attr("fill", d => useGrowthColor ? growthColor(getGrowthValue(d.data.row, side)) : baseFill)
    .attr("stroke", stroke)
    .attr("stroke-width", 1);

  // Flag image (always show when iso2 is available)
  const FLAG_W = 20, FLAG_H = 14;
  nodes.append("image")
    .attr("class", "flagImg")
    .attr("href", d => {
      const iso2 = d.data.iso2;
      const width = d.x1 - d.x0;
      const height = d.y1 - d.y0;
      return iso2 && width >= 32 && height >= 22 ? flagURL(iso2) : null;
    })
    .attr("x", 6)
    .attr("y", 6)
    .attr("width", FLAG_W)
    .attr("height", FLAG_H)
    .attr("clip-path", d => `url(#${d.__clipId})`);

  // labels
  nodes.append("text")
    .attr("class","tileLabel")
    .attr("x", 6)
    .attr("y", 18)
    .text(d => {
      const width = d.x1 - d.x0;
      const height = d.y1 - d.y0;
      const minimumWidth = d.data.iso2 ? 56 : 30;
      return labelSet.has(d.data.iso3) && width >= minimumWidth && height >= 24 ? d.data.iso3 : "";
    })
    .attr("fill", "rgba(255,255,255,.78)")
    .attr("font-size", 12)
    .attr("font-family", "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace")
    .style("pointer-events","none")
    .attr("dx", d => {
      const w = (d.x1 - d.x0), h = (d.y1 - d.y0);
      return (d.data.iso2 && w >= 56 && h >= 24) ? 24 : 0;
    })
    .attr("clip-path", d => `url(#${d.__clipId})`);

  nodes.append("text")
    .attr("class","tileValue")
    .attr("x", 6)
    .attr("y", 34)
    .text(d => {
      if (!labelSet.has(d.data.iso3)) return "";
      const width = d.x1 - d.x0;
      const height = d.y1 - d.y0;
      const value = formatCompactMetricValue(d.data.value);
      return height >= 42 && width >= value.length * 7 + 12 ? value : "";
    })
    .attr("fill", "rgba(255,255,255,.55)")
    .attr("font-size", 11)
    .attr("font-family", "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace")
    .style("pointer-events","none")
    .attr("clip-path", d => `url(#${d.__clipId})`);

  nodes
    .on("mousemove", (ev, d) => {
      const row = d.data.row;
      applyHighlight(row.iso3);
      showTooltip(ev, row, side);
    })
    .on("mouseleave", () => {
      hideTooltip();
      applyHighlight(state.highlightKey);
    })
    .on("click", (ev, d) => {
      selectCountry(d.data.row);
    })
    .on("focus", (ev, d) => {
      applyHighlight(d.data.row.iso3);
    })
    .on("keydown", (ev, d) => {
      if (ev.key !== "Enter" && ev.key !== " ") return;
      ev.preventDefault();
      selectCountry(d.data.row);
    });
}

function boundedInputValue(element, fallback, minimum, maximum){
  const value = Number(element?.value);
  return Number.isFinite(value) ? Math.max(minimum, Math.min(maximum, value)) : fallback;
}

function currentViewState(){
  return {
    metric: state.metric,
    color: state.colorMode,
    top: state.topN,
    mode: state.comparisonMode,
    period: state.period,
    region: state.region,
    income: state.income,
    group: state.group,
    normalization: state.normalization,
    country: state.selectedRow?.iso3 || "",
    query: state.tableQuery,
    tab: state.tab,
    sector: state.strategicSector,
    scenarioPartner: els.scenarioPartner?.value || "usa",
    scenarioProduct: els.scenarioProduct?.value || "",
    tariffBase: boundedInputValue(els.scenarioTariffBase, 0, 0, 300),
    tariffChange: boundedInputValue(els.scenarioTariffChange, 10, -100, 300),
    elasticity: boundedInputValue(els.scenarioElasticity, -1.5, -10, -0.05),
    passThrough: boundedInputValue(els.scenarioPassThrough, 1, 0, 1),
  };
}

function syncURL(){
  const query = serializeViewState(currentViewState());
  const next = `${window.location.pathname}${query ? `?${query}` : ""}${window.location.hash}`;
  window.history.replaceState(null, "", next);
}

async function copyTextToClipboard(text){
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const field = document.createElement("textarea");
    field.value = text;
    field.setAttribute("readonly", "");
    field.style.position = "fixed";
    field.style.left = "-9999px";
    document.body.appendChild(field);
    field.select();
    let copied = false;
    try {
      copied = document.execCommand("copy");
    } finally {
      field.remove();
    }
    return copied;
  }
}

function refreshRows(options = {}){
  const periodRows = deriveRowsForPeriod(state.latestRows, state.seriesRows, state.period);
  state.rows = filterExplorerRows(periodRows, {
    mode: state.comparisonMode,
    region: state.region,
    income: state.income,
    group: state.group,
    query: state.tableQuery,
  });
  if (state.selectedRow) {
    const selected = state.rows.find(row => row.iso3 === state.selectedRow.iso3);
    if (selected) {
      state.selectedRow = selected;
    } else {
      state.selectedRow = null;
      state.highlightKey = null;
    }
  }
  if (options.syncURL !== false) syncURL();
  renderAll();
}

function fillSelect(select, values, firstLabel){
  if (!select) return;
  const selected = select.value;
  const fragment = document.createDocumentFragment();
  const first = document.createElement("option");
  first.value = "";
  first.textContent = firstLabel;
  fragment.appendChild(first);
  for (const value of values) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = value;
    fragment.appendChild(option);
  }
  select.replaceChildren(fragment);
  if (values.includes(selected)) select.value = selected;
}

function populateExplorerControls(){
  if (els.periodFilter) {
    const periods = availablePeriods(state.seriesRows);
    const fragment = document.createDocumentFragment();
    const latest = document.createElement("option");
    latest.value = "latest";
    latest.textContent = "Latest by reporter";
    fragment.appendChild(latest);
    for (const period of periods) {
      const option = document.createElement("option");
      option.value = period.key;
      option.textContent = `${period.key.replace(":", " ")} · ${period.comparable}/${period.reporters} comparable`;
      fragment.appendChild(option);
    }
    els.periodFilter.replaceChildren(fragment);
  }
  const regions = Array.from(new Set(state.latestRows.map(row => row.region).filter(Boolean))).sort();
  const incomes = Array.from(new Set(state.latestRows.map(row => row.income_group).filter(Boolean))).sort();
  const groups = Array.from(new Set(state.latestRows.flatMap(row => row.groups || []).filter(Boolean))).sort();
  fillSelect(els.regionFilter, regions, "All regions");
  fillSelect(els.incomeFilter, incomes, "All income groups");
  fillSelect(els.groupFilter, groups, "All groups");
}

function reconcileExplorerState(){
  for (const [element, property, fallback] of [
    [els.periodFilter, "period", "latest"],
    [els.regionFilter, "region", ""],
    [els.incomeFilter, "income", ""],
    [els.groupFilter, "group", ""],
  ]) {
    if (!element) continue;
    const values = Array.from(element.options).map(option => option.value);
    if (!values.includes(state[property])) state[property] = fallback;
  }
}

function populateStrategicControls(){
  if (!els.strategicSectorFilter) return;
  const sectors = Array.isArray(state.strategicIndex?.sectors) ? state.strategicIndex.sectors : [];
  const fragment = document.createDocumentFragment();
  const all = document.createElement("option");
  all.value = "all";
  all.textContent = "All strategic sectors";
  fragment.appendChild(all);
  for (const sector of sectors) {
    const option = document.createElement("option");
    option.value = sector;
    option.textContent = sector.replaceAll("_", " ").replace(/\b\w/g, char => char.toUpperCase());
    fragment.appendChild(option);
  }
  els.strategicSectorFilter.replaceChildren(fragment);
  if (state.strategicSector !== "all" && !sectors.includes(state.strategicSector)) {
    state.strategicSector = "all";
  }
  els.strategicSectorFilter.value = state.strategicSector;

  if (els.scenarioProduct) {
    const products = Array.isArray(state.tariffIndex?.products) && state.tariffIndex.products.length > 0
      ? state.tariffIndex.products
      : (Array.isArray(state.strategicIndex?.products) ? state.strategicIndex.products : []);
    const selectedCode = els.scenarioProduct.value;
    const productFragment = document.createDocumentFragment();
    const aggregate = document.createElement("option");
    aggregate.value = "";
    aggregate.textContent = "Aggregate imports (manual tariff)";
    productFragment.appendChild(aggregate);
    for (const product of products) {
      if (!/^\d{6}$/.test(String(product.code || ""))) continue;
      const option = document.createElement("option");
      option.value = product.code;
      option.textContent = `${product.code} · ${product.label}`;
      productFragment.appendChild(option);
    }
    els.scenarioProduct.replaceChildren(productFragment);
    const codes = products.map(product => product.code);
    els.scenarioProduct.value = codes.includes(selectedCode) ? selectedCode : (codes.includes("854231") ? "854231" : "");
  }
}

function syncExplorerControls(){
  if (els.periodFilter) els.periodFilter.value = state.period;
  if (els.comparisonMode) els.comparisonMode.value = state.comparisonMode;
  if (els.regionFilter) els.regionFilter.value = state.region;
  if (els.incomeFilter) els.incomeFilter.value = state.income;
  if (els.groupFilter) els.groupFilter.value = state.group;
  if (els.normalization) els.normalization.value = state.normalization;
  if (els.topN) els.topN.value = String(state.topN);
  if (els.tableSearch) els.tableSearch.value = state.tableQuery;
}

function syncScenarioControls(view){
  if (!view) return;
  state.preserveScenarioInputs = true;
  if (els.scenarioPartner) els.scenarioPartner.value = view.scenarioPartner || "usa";
  if (els.scenarioProduct) {
    const available = Array.from(els.scenarioProduct.options).some(option => option.value === view.scenarioProduct);
    els.scenarioProduct.value = available ? view.scenarioProduct : "";
  }
  if (els.scenarioTariffBase) els.scenarioTariffBase.value = String(view.tariffBase ?? 0);
  if (els.scenarioTariffChange) els.scenarioTariffChange.value = String(view.tariffChange ?? 10);
  if (els.scenarioElasticity) els.scenarioElasticity.value = String(view.elasticity ?? -1.5);
  if (els.scenarioPassThrough) els.scenarioPassThrough.value = String(view.passThrough ?? 1);
}

function seriesMetricValue(point, side, selected){
  return normalizedMetricValue({
    [side]: point?.[side],
    population: selected?.population,
    gdp: selected?.gdp,
  }, side, state.metric, state.normalization);
}

function renderTimeSeries(){
  if (!els.timeSeries) return;
  const selected = state.selectedRow;
  if (!selected) {
    els.timeSeries.textContent = "Select a country to view its time series.";
    return;
  }
  const reporter = state.seriesRows.find(row => row.iso3 === selected.iso3);
  const annual = (reporter?.points || []).filter(point => point.period_type === "Y" && (point.usa?.available || point.chn?.available));
  const points = annual.length >= 2 ? annual.slice(-10) : (reporter?.points || []).slice(-10);
  if (points.length === 0) {
    els.timeSeries.textContent = `No time-series observations are available for ${selected.name}.`;
    return;
  }

  const width = Math.max(360, els.timeSeries.clientWidth - 32 || 560);
  const height = 180;
  const margin = { top: 12, right: 16, bottom: 30, left: 58 };
  const values = points.flatMap(point => [seriesMetricValue(point, "usa", selected), seriesMetricValue(point, "chn", selected)]).filter(Number.isFinite);
  const maxValue = d3.max(values) || 1;
  const x = d3.scalePoint().domain(points.map(point => point.period)).range([margin.left, width - margin.right]).padding(0.35);
  const y = d3.scaleLinear().domain([0, maxValue]).nice().range([height - margin.bottom, margin.top]);
  const wrap = document.createElement("div");
  const svgNode = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svgNode.classList.add("trendChart");
  svgNode.setAttribute("viewBox", `0 0 ${width} ${height}`);
  svgNode.setAttribute("role", "img");
  svgNode.setAttribute("aria-label", `${selected.name} ${metricLabel()} trend with USA and China`);
  wrap.appendChild(svgNode);
  const legend = document.createElement("div");
  legend.className = "trendLegend";
  legend.innerHTML = `<span>USA</span><span class="china">China</span>`;
  wrap.appendChild(legend);
  const note = document.createElement("div");
  note.className = "analysisNote";
  note.textContent = `${points.length} published periods. ${state.normalization === "raw" ? "Nominal US dollars." : "Normalization uses the latest published World Bank denominator and is intended for comparison, not constant-price analysis."}`;
  wrap.appendChild(note);
  els.timeSeries.replaceChildren(wrap);

  const svg = d3.select(svgNode);
  svg.append("g").attr("class", "trendAxis").attr("transform", `translate(0,${height - margin.bottom})`).call(d3.axisBottom(x).tickValues(x.domain().filter((_, index) => index % Math.max(1, Math.ceil(points.length / 6)) === 0)));
  svg.append("g").attr("class", "trendAxis").attr("transform", `translate(${margin.left},0)`).call(d3.axisLeft(y).ticks(4).tickFormat(value => state.normalization === "gdp_share" ? `${(value * 100).toFixed(1)}%` : fmt(value)));
  const line = side => d3.line().defined(point => Number.isFinite(seriesMetricValue(point, side, selected))).x(point => x(point.period)).y(point => y(seriesMetricValue(point, side, selected)));
  for (const [side, color] of [["usa", "#5aa2ff"], ["chn", "#ff6b57"]]) {
    svg.append("path").datum(points).attr("fill", "none").attr("stroke", color).attr("stroke-width", 2.5).attr("d", line(side));
    svg.selectAll(`circle.${side}`).data(points.filter(point => Number.isFinite(seriesMetricValue(point, side, selected)))).enter().append("circle").attr("class", side).attr("cx", point => x(point.period)).attr("cy", point => y(seriesMetricValue(point, side, selected))).attr("r", 3).attr("fill", color).append("title").text(point => `${point.period}: ${formatMetricValue(seriesMetricValue(point, side, selected))}`);
  }
}

async function renderProducts(){
  if (!els.products) return;
  const selected = state.selectedRow;
  if (!selected) {
    els.products.textContent = "Select a country to view product chapters.";
    return;
  }
  if (!state.productIndex?.reporters?.includes(selected.iso3)) {
    els.products.textContent = `No HS2 product file is available for ${selected.name}.`;
    return;
  }
  els.products.textContent = "Loading HS2 product chapters…";
  try {
    if (!state.productCache[selected.iso3]) {
      state.productCache[selected.iso3] = fetch(`./data/products/${selected.iso3}.json`, { cache: "no-store" }).then(response => {
        if (!response.ok) throw new Error(`product request failed (${response.status})`);
        return response.json();
      });
    }
    const file = await state.productCache[selected.iso3];
    if (state.selectedRow?.iso3 !== selected.iso3) return;
    const requested = state.period === "latest" ? "" : state.period.split(":").slice(1).join(":");
    const period = requested && file.rows?.some(row => row.period === requested) ? requested : file.periods?.[0];
    const rows = (file.rows || []).filter(row => row.period === period).map(row => {
      const usa = normalizedMetricValue({ ...selected, usa: row.usa }, "usa", state.metric, state.normalization) ?? 0;
      const chn = normalizedMetricValue({ ...selected, chn: row.chn }, "chn", state.metric, state.normalization) ?? 0;
      return { ...row, normalizedUSA: usa, normalizedCHN: chn, normalizedTotal: usa + chn };
    }).sort((a, b) => b.normalizedTotal - a.normalizedTotal).slice(0, 10);
    if (rows.length === 0) {
      els.products.textContent = `No HS2 observations are available for ${selected.name} in the selected product period.`;
      return;
    }
    const body = rows.map(row => `<tr><td><span class="evidenceTag">HS ${escapeHTML(row.code)}</span><br>${escapeHTML(row.name)}</td><td class="numeric">${formatMetricValue(row.normalizedUSA)}</td><td class="numeric">${formatMetricValue(row.normalizedCHN)}</td><td class="numeric">${formatMetricValue(row.normalizedTotal)}</td></tr>`).join("");
    els.products.innerHTML = `<table class="miniTable"><thead><tr><th>Chapter</th><th class="numeric">USA</th><th class="numeric">China</th><th class="numeric">Combined</th></tr></thead><tbody>${body}</tbody></table><div class="analysisNote">${escapeHTML(String(file.provider || "").toUpperCase())} · ${escapeHTML(file.classification)} level ${Number(file.level)} · ${escapeHTML(period || "unknown period")}. Product totals are not substituted for the headline provider.</div>`;
  } catch (error) {
    console.error(error);
    els.products.textContent = "Failed to load this country's product file.";
  }
}

function strategicRegistryTable(products){
  const rows = products.map(product => `<tr><td><span class="sectorTag">${escapeHTML(product.sector.replaceAll("_", " "))}</span></td><td><span class="evidenceTag">HS ${escapeHTML(product.code)}</span><br>${escapeHTML(product.label)}</td><td>${escapeHTML(product.revision_note)}</td></tr>`).join("");
  return `<table class="miniTable"><thead><tr><th>Sector</th><th>Product</th><th>Revision scope</th></tr></thead><tbody>${rows}</tbody></table>`;
}

function requestedAnnualPeriod(){
  if (state.period === "latest") return "";
  const period = state.period.split(":").slice(1).join(":");
  return /^\d{4}$/.test(period) ? period : "";
}

async function loadStrategicPartition(iso3){
  if (!/^[A-Z]{3}$/.test(String(iso3 || ""))) return null;
  const partitions = (state.strategicIndex?.partitions || [])
    .filter(partition => partition.reporter_iso3 === iso3 && /^\d{4}$/.test(String(partition.period || "")))
    .sort((a, b) => String(b.period).localeCompare(String(a.period)));
  if (partitions.length === 0) return null;
  const requested = requestedAnnualPeriod();
  const partition = partitions.find(item => item.period === requested) || partitions[0];
  const cacheKey = `${iso3}/${partition.period}`;
  if (!state.strategicCache[cacheKey]) {
    state.strategicCache[cacheKey] = fetch(`./data/strategic-hs6/${iso3}/${partition.period}.json`, { cache: "no-store" }).then(response => {
      if (!response.ok) throw new Error(`strategic HS6 request failed (${response.status})`);
      return response.json();
    }).catch(error => {
      delete state.strategicCache[cacheKey];
      throw error;
    });
  }
  return { partition, requested, file: await state.strategicCache[cacheKey] };
}

async function loadTariffPartition(iso3){
  if (!/^[A-Z]{3}$/.test(String(iso3 || ""))) return null;
  const partitions = (state.tariffIndex?.partitions || [])
    .filter(partition => partition.importer_iso3 === iso3 && /^\d{4}$/.test(String(partition.year || "")))
    .sort((a, b) => String(b.year).localeCompare(String(a.year)));
  if (partitions.length === 0) return null;
  const requested = requestedAnnualPeriod();
  const partition = partitions.find(item => item.year === requested) || partitions[0];
  const cacheKey = `${iso3}/${partition.year}`;
  if (!state.tariffCache[cacheKey]) {
    state.tariffCache[cacheKey] = fetch(`./data/tariffs/${iso3}/${partition.year}.json`, { cache: "no-store" }).then(response => {
      if (!response.ok) throw new Error(`tariff request failed (${response.status})`);
      return response.json();
    }).catch(error => {
      delete state.tariffCache[cacheKey];
      throw error;
    });
  }
  return { partition, requested, file: await state.tariffCache[cacheKey] };
}

function tariffRateLabel(row){
  if (!row || !Number.isFinite(Number(row.rate_percent))) return "—";
  const method = row.data_type === "ave_estimated" ? "incl. AVE" : "reported";
  return `${Number(row.rate_percent).toFixed(2)}% · ${method}`;
}

async function renderStrategicProducts(){
  if (!els.strategicProducts) return;
  const index = state.strategicIndex;
  if (!index || !Array.isArray(index.products)) {
    els.strategicProducts.textContent = "No strategic HS6 registry is published.";
    return;
  }
  const registryProducts = index.products.filter(product => state.strategicSector === "all" || product.sector === state.strategicSector);
  const selected = state.selectedRow;
  if (!selected) {
    els.strategicProducts.innerHTML = `${strategicRegistryTable(registryProducts)}<div class="analysisNote">Registry ${Number(index.products.length)} products · ${Number(index.sectors?.length || 0)} sectors. Select a reporter to load available trade and tariff partitions.</div>`;
    return;
  }
  els.strategicProducts.textContent = "Loading strategic HS6 trade and tariff partitions…";
  try {
    const [tradePartition, tariffPartition] = await Promise.all([
      loadStrategicPartition(selected.iso3),
      loadTariffPartition(selected.iso3),
    ]);
    if (state.selectedRow?.iso3 !== selected.iso3) return;
    const tradeByCode = new Map((tradePartition?.file?.rows || []).map(row => [row.code, row]));
    const tariffByCode = selectPreferredTariffs(tariffPartition?.file?.rows || []);
    const rows = registryProducts.map(product => {
      const trade = tradeByCode.get(product.code);
      const usa = trade ? normalizedMetricValue({ ...selected, usa: trade.usa }, "usa", state.metric, state.normalization) : null;
      const chn = trade ? normalizedMetricValue({ ...selected, chn: trade.chn }, "chn", state.metric, state.normalization) : null;
      return { ...product, trade, tariff: tariffByCode.get(product.code), normalizedUSA: usa, normalizedCHN: chn, normalizedTotal: Number.isFinite(usa) || Number.isFinite(chn) ? (Number(usa) || 0) + (Number(chn) || 0) : null };
    }).sort((a, b) => (Number(b.normalizedTotal) || 0) - (Number(a.normalizedTotal) || 0) || a.code.localeCompare(b.code));
    if (rows.length === 0) {
      els.strategicProducts.textContent = "No strategic products match this sector.";
      return;
    }
    const body = rows.map(row => `<tr><td><span class="sectorTag">${escapeHTML(row.sector.replaceAll("_", " "))}</span></td><td><span class="evidenceTag">HS ${escapeHTML(row.code)}</span><br>${escapeHTML(row.label)}</td><td>${escapeHTML(row.trade?.classification || row.tariff?.classification || "—")}<br><span class="subtle">${escapeHTML(row.revision_note)}</span></td><td class="numeric">${formatMetricValue(row.normalizedUSA)}</td><td class="numeric">${formatMetricValue(row.normalizedCHN)}</td><td class="numeric">${formatMetricValue(row.normalizedTotal)}</td><td class="numeric" title="MFN simple average; trade remedies are not included">${escapeHTML(tariffRateLabel(row.tariff))}</td></tr>`).join("");
    const tradeNote = tradePartition ? `${String(tradePartition.file.provider || "").toUpperCase()} trade ${tradePartition.partition.period}` : "trade partition unavailable";
    const tariffNote = tariffPartition ? `${String(tariffPartition.file.provider || "").toUpperCase()} MFN ${tariffPartition.partition.year}` : "tariff partition unavailable";
    const requested = requestedAnnualPeriod();
    const fallbacks = [tradePartition && requested && requested !== tradePartition.partition.period ? `trade→${tradePartition.partition.period}` : "", tariffPartition && requested && requested !== tariffPartition.partition.year ? `tariff→${tariffPartition.partition.year}` : ""].filter(Boolean);
    els.strategicProducts.innerHTML = `<table class="miniTable"><thead><tr><th>Sector</th><th>HS6 product</th><th>Source revision</th><th class="numeric">USA</th><th class="numeric">China</th><th class="numeric">Combined</th><th class="numeric">MFN avg.</th></tr></thead><tbody>${body}</tbody></table><div class="analysisNote">${escapeHTML(tradeNote)} · ${escapeHTML(tariffNote)}. Source classifications remain row-level; MFN values are not bilateral trade-remedy rates.${fallbacks.length ? ` Period fallback: ${escapeHTML(fallbacks.join(", "))}.` : ""}</div>`;
  } catch (error) {
    console.error(error);
    els.strategicProducts.textContent = "Failed to load this reporter's strategic trade or tariff partition.";
  }
}

function renderQualityDashboard(){
  if (!els.qualityDashboard) return;
  const quality = state.quality;
  if (!quality?.summary) {
    els.qualityDashboard.textContent = "No quality report is available.";
    return;
  }
  const summary = quality.summary;
  const selectedIssue = state.selectedRow ? (quality.reporter_issues || []).find(item => item.iso3 === state.selectedRow.iso3) : null;
  const latestRun = (quality.collection_runs || [])[0];
  const comparisons = (quality.provider_comparison || []).filter(item => !state.selectedRow || item.iso3 === state.selectedRow.iso3).slice(0, 4);
  const stats = [
    [summary.comparable_reporters, "Comparable reporters"],
    [summary.incomparable_reporters, "Mixed/missing periods"],
    [summary.missing_partner_blocks, "Missing partner blocks"],
    [summary.stale_partner_blocks, "Stale partner blocks"],
    [summary.provider_comparison_count, "Provider comparisons"],
    [quality.dominant_period || "—", "Dominant period"],
  ].map(([value, label]) => `<div class="qualityStat"><b>${escapeHTML(value)}</b><span>${escapeHTML(label)}</span></div>`).join("");
  const issueHTML = state.selectedRow
    ? `<div class="subSectionTitle">Selected reporter</div><div>${selectedIssue ? selectedIssue.issues.map(value => `<span class="statusPill warning">${escapeHTML(value)}</span>`).join(" ") : '<span class="statusPill success">No flagged issue</span>'}</div>`
    : "";
  const runStatus = latestRun ? ["success", "partial", "failed"].includes(latestRun.status) ? latestRun.status : "warning" : "warning";
  const runHTML = latestRun ? `<div class="subSectionTitle">Latest collection run</div><div><span class="statusPill ${runStatus}">${escapeHTML(latestRun.status)}</span> ${escapeHTML(latestRun.provider)} ${escapeHTML(latestRun.mode)} · ${Number(latestRun.success_count)}/${Number(latestRun.request_count)} requests · ${Number(latestRun.stored_count)} stored</div>` : "";
  const comparisonHTML = comparisons.length ? `<div class="subSectionTitle">Same-period provider deltas</div><table class="miniTable"><thead><tr><th>Reporter/partner</th><th>Period</th><th class="numeric">Delta</th></tr></thead><tbody>${comparisons.map(item => `<tr><td>${escapeHTML(item.iso3)} / ${escapeHTML(item.partner)}</td><td>${escapeHTML(item.period)}</td><td class="numeric">${fmtPct(item.delta_ratio)}</td></tr>`).join("")}</tbody></table>` : "";
  els.qualityDashboard.innerHTML = `<div class="qualityStats">${stats}</div>${issueHTML}${runHTML}${comparisonHTML}<div class="analysisNote">Provider deltas compare the headline total with the sum of HS2 observations only when reporter, partner, flow, and observation period match.</div>`;
}

function fallbackExplanation(row){
  const usa = getMetricValue(row, "usa");
  const chn = getMetricValue(row, "chn");
  const leader = usa >= chn ? "USA" : "China";
  return {
    generator: { type: "rules", status: "fallback", model: "none" },
    summary: `${row.name}'s published ${metricLabel()} is larger with ${leader} in this view.`,
    statements: [
      { text: `USA: ${formatMetricValue(usa)} for ${row.usa.period || "an unavailable period"}.`, evidence_ids: ["TOTAL-USA"] },
      { text: `China: ${formatMetricValue(chn)} for ${row.chn.period || "an unavailable period"}.`, evidence_ids: ["TOTAL-CHN"] },
      { text: row.same_period ? "The two partner values are directly comparable by observation period." : "The partner values use mixed or missing periods and should not be treated as a same-period comparison.", evidence_ids: ["QUALITY-PERIOD"] },
    ],
    evidence: [
      { id: "TOTAL-USA", label: "Headline USA observation", period: row.usa.period, source: state.provider },
      { id: "TOTAL-CHN", label: "Headline China observation", period: row.chn.period, source: state.provider },
      { id: "QUALITY-PERIOD", label: row.same_period ? "Same-period check passed" : "Same-period check failed", period: row.comparison_period, source: "quality.json" },
    ],
  };
}

function renderExplanationData(data){
  const generator = data.generator || {};
  const statements = (data.statements || []).map(statement => `<li>${escapeHTML(statement.text)} <span class="evidenceTag">${(statement.evidence_ids || []).map(id => `[${escapeHTML(id)}]`).join(" ")}</span></li>`).join("");
  const evidence = (data.evidence || []).map(item => `<tr><td><span class="evidenceTag">${escapeHTML(item.id)}</span></td><td>${escapeHTML(item.label)}</td><td>${escapeHTML(item.period || "—")}</td><td>${escapeHTML(item.source || "—")}</td></tr>`).join("");
  els.explanation.innerHTML = `<div><span class="statusPill ${generator.status === "success" ? "success" : "warning"}">${escapeHTML(generator.type || "rules")}</span> ${generator.model && generator.model !== "none" ? escapeHTML(generator.model) : "deterministic evidence summary"}</div><p>${escapeHTML(data.summary || "No summary available.")}</p><ol class="evidenceList">${statements}</ol><table class="miniTable"><thead><tr><th>Evidence</th><th>Meaning</th><th>Period</th><th>Source</th></tr></thead><tbody>${evidence}</tbody></table><div class="analysisNote">Explanations are generated at build time. The browser never receives an AI API key, and unsupported citations are rejected by the build step.</div>`;
}

async function renderExplanation(){
  if (!els.explanation) return;
  const selected = state.selectedRow;
  if (!selected) {
    els.explanation.textContent = "Select a country to view its explanation.";
    return;
  }
  if (state.period !== "latest") {
    renderExplanationData(fallbackExplanation(selected));
    return;
  }
  els.explanation.textContent = "Loading evidence-grounded explanation…";
  try {
    if (!state.explanationCache[selected.iso3]) {
      state.explanationCache[selected.iso3] = fetch(`./data/explanations/${selected.iso3}.json`, { cache: "no-store" }).then(response => response.ok ? response.json() : null).catch(() => null);
    }
    const data = await state.explanationCache[selected.iso3];
    if (state.selectedRow?.iso3 !== selected.iso3) return;
    renderExplanationData(data || fallbackExplanation(selected));
  } catch {
    renderExplanationData(fallbackExplanation(selected));
  }
}

function intelligenceRows(){
  return state.rows.map(row => ({
    ...row,
    usa: {
      ...row.usa,
      trade: normalizedMetricValue(row, "usa", "trade", state.normalization) ?? 0,
      export: normalizedMetricValue(row, "usa", "export", state.normalization) ?? 0,
      import: normalizedMetricValue(row, "usa", "import", state.normalization) ?? 0,
    },
    chn: {
      ...row.chn,
      trade: normalizedMetricValue(row, "chn", "trade", state.normalization) ?? 0,
      export: normalizedMetricValue(row, "chn", "export", state.normalization) ?? 0,
      import: normalizedMetricValue(row, "chn", "import", state.normalization) ?? 0,
    },
  }));
}

function renderIntelligenceSummary(rows){
  if (!els.intelligenceSummary) return;
  const selected = rows.find(row => row.iso3 === state.selectedRow?.iso3);
  if (!selected) {
    els.intelligenceSummary.textContent = "Select a country in Overview or the exposure ranking.";
    return;
  }
  const profile = buildIntelligenceProfile(selected, state.metric);
  const divergence = profile.growthDivergence == null ? "—" : `${(profile.growthDivergence * 100).toFixed(1)}pp`;
  const signals = profile.signals.map(signal => `<div class="signalItem ${escapeHTML(signal.level)}">${escapeHTML(signal.label)}</div>`).join("");
  els.intelligenceSummary.innerHTML = `
    <div class="signalMetrics">
      <div class="signalMetric"><span>Observed ${escapeHTML(metricLabel())}</span><b>${formatMetricValue(profile.total)}</b></div>
      <div class="signalMetric"><span>China share</span><b>${(profile.chinaShare * 100).toFixed(1)}%</b></div>
      <div class="signalMetric"><span>Two-partner HHI</span><b>${profile.concentration.toFixed(3)}</b></div>
      <div class="signalMetric"><span>Net balance</span><b>${formatMetricValue(profile.netBalance)}</b></div>
      <div class="signalMetric"><span>Growth divergence</span><b>${escapeHTML(divergence)}</b></div>
      <div class="signalMetric"><span>Observation scope</span><b>USA + CHN</b></div>
    </div>
    <div class="subSectionTitle">Threshold signals for ${escapeHTML(profile.name)}</div>
    <div class="signalList">${signals}</div>
    <div class="analysisNote">HHI and shares cover only the two published anchor partners. They are not whole-world concentration measures.</div>`;
}

function renderExposureRanking(rows){
  if (!els.exposureRankingBody) return;
  const profiles = rankExposureRows(rows, state.metric);
  const fragment = document.createDocumentFragment();
  for (const profile of profiles) {
    const tr = document.createElement("tr");
    if (state.selectedRow?.iso3 === profile.iso3) tr.classList.add("is-selected");
    const nameCell = document.createElement("td");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "tableCountryButton";
    button.textContent = profile.name;
    const iso = document.createElement("span");
    iso.className = "iso";
    iso.textContent = profile.iso3;
    button.appendChild(iso);
    button.addEventListener("click", () => {
      const selected = state.rows.find(row => row.iso3 === profile.iso3);
      if (selected) selectCountry(selected);
    });
    nameCell.appendChild(button);
    tr.appendChild(nameCell);
    appendTableCell(tr, formatMetricValue(profile.total), "numeric");
    appendTableCell(tr, `${(profile.chinaShare * 100).toFixed(1)}%`, "numeric");
    appendTableCell(tr, profile.concentration.toFixed(3), "numeric");
    appendTableCell(tr, formatMetricValue(profile.netBalance), "numeric");
    appendTableCell(tr, profile.signals[0]?.label || "—");
    fragment.appendChild(tr);
  }
  els.exposureRankingBody.replaceChildren(fragment);
}

function renderAnchorNetwork(rows){
  if (!els.networkChart || state.tab !== "intelligence") return;
	if (els.networkTitle) els.networkTitle.textContent = "Observed anchor network";
	if (els.networkDescription) els.networkDescription.textContent = "Filtered reporters connected to USA and China; link width represents the selected metric.";
	if (els.networkNote) els.networkNote.textContent = "Select a country to load its reported multi-partner matrix when available. This fallback is not an inferred physical supply-chain route.";
  const width = Math.max(500, els.networkChart.getBoundingClientRect().width || 0);
  const height = 330;
  const graph = buildAnchorNetwork(rows, state.metric, Math.min(state.topN, 30));
  const svg = d3.select(els.networkChart).attr("viewBox", `0 0 ${width} ${height}`);
  svg.selectAll("*").remove();
  const reporters = graph.nodes.filter(node => node.kind === "reporter");
  const profiles = new Map(rankExposureRows(rows, state.metric).map(profile => [profile.iso3, profile]));
  const positions = new Map([
    ["USA", { x: 46, y: height / 2 }],
    ["CHN", { x: width - 46, y: height / 2 }],
  ]);
  reporters.forEach((node, index) => {
    const profile = profiles.get(node.id);
    const step = reporters.length > 1 ? (height - 44) / (reporters.length - 1) : 0;
    positions.set(node.id, {
      x: 120 + (width - 240) * (profile?.chinaShare ?? 0.5),
      y: reporters.length > 1 ? 22 + index * step : height / 2,
    });
  });
  const maxLink = d3.max(graph.links, link => link.value) || 1;
  const linkWidth = d3.scaleSqrt().domain([0, maxLink]).range([0.5, 7]);
  svg.append("g").selectAll("line").data(graph.links).join("line")
    .attr("class", "networkLink")
    .attr("x1", link => positions.get(link.source)?.x)
    .attr("y1", link => positions.get(link.source)?.y)
    .attr("x2", link => positions.get(link.target)?.x)
    .attr("y2", link => positions.get(link.target)?.y)
    .attr("stroke-width", link => linkWidth(link.value));
  const maxNode = d3.max(reporters, node => node.total) || 1;
  const nodeRadius = d3.scaleSqrt().domain([0, maxNode]).range([3, 11]);
  const groups = svg.append("g").selectAll("g").data(graph.nodes).join("g")
    .attr("class", node => `networkNode ${node.kind}${state.selectedRow?.iso3 === node.id ? " is-selected" : ""}`)
    .attr("transform", node => `translate(${positions.get(node.id)?.x || 0},${positions.get(node.id)?.y || 0})`)
    .attr("tabindex", node => node.kind === "reporter" ? 0 : null)
    .attr("role", node => node.kind === "reporter" ? "button" : null)
    .on("click", (_, node) => {
      if (node.kind !== "reporter") return;
      const selected = state.rows.find(row => row.iso3 === node.id);
      if (selected) selectCountry(selected);
    })
    .on("keydown", (event, node) => {
      if (node.kind !== "reporter" || (event.key !== "Enter" && event.key !== " ")) return;
      event.preventDefault();
      const selected = state.rows.find(row => row.iso3 === node.id);
      if (selected) selectCountry(selected);
    });
  groups.append("circle").attr("r", node => node.kind === "anchor" ? 14 : nodeRadius(node.total));
  groups.append("text")
    .attr("x", node => node.id === "USA" ? 20 : node.id === "CHN" ? -20 : 0)
    .attr("y", node => node.kind === "anchor" ? 4 : -8)
    .attr("text-anchor", node => node.id === "CHN" ? "end" : node.id === "USA" ? "start" : "middle")
    .text(node => node.kind === "anchor" ? node.id : node.id);
  groups.append("title").text(node => node.kind === "anchor" ? node.label : `${node.label}: ${formatMetricValue(node.total)}`);
}

function matrixPartitionFor(reporterISO3, preferredPeriod = ""){
	const reporter = normalizeISO3(reporterISO3);
	if (!reporter) return null;
	const partitions = (Array.isArray(state.matrixIndex?.partitions) ? state.matrixIndex.partitions : [])
	  .filter(partition => normalizeISO3(partition?.reporter_iso3) === reporter && /^\d{4}$/.test(String(partition?.period || "")))
	  .sort((a, b) => String(b.period).localeCompare(String(a.period)));
	return partitions.find(partition => String(partition.period) === String(preferredPeriod || "")) || partitions[0] || null;
}

async function loadMatrixPartition(partition){
	const reporter = normalizeISO3(partition?.reporter_iso3);
	const period = String(partition?.period || "");
	if (!reporter || !/^\d{4}$/.test(period)) return null;
	const key = `${reporter}/${period}`;
	if (Object.prototype.hasOwnProperty.call(state.matrixCache, key)) return state.matrixCache[key];
	if (!state.matrixPromises[key]) {
	  state.matrixPromises[key] = fetch(`./data/bilateral-matrix/${reporter}/${period}.json`, { cache: "no-store" })
		.then(response => response.ok ? response.json() : null)
		.then(file => {
		  const valid = normalizeISO3(file?.reporter_iso3) === reporter
			&& String(file?.period || "") === period
			&& Array.isArray(file?.rows);
		  state.matrixCache[key] = valid ? file : null;
		  return state.matrixCache[key];
		})
		.catch(() => {
		  state.matrixCache[key] = null;
		  return null;
		})
		.finally(() => { delete state.matrixPromises[key]; });
	}
	return state.matrixPromises[key];
}

function normalizeMatrixValue(value, selected){
	const numeric = Math.max(0, Number(value) || 0);
	if (state.normalization === "gdp_share") {
	  const gdp = Number(selected?.gdp?.value);
	  return gdp > 0 ? numeric / gdp : 0;
	}
	if (state.normalization === "per_capita") {
	  const population = Number(selected?.population?.value);
	  return population > 0 ? numeric / population : 0;
	}
	return numeric;
}

function renderPartnerNetwork(file, selected){
	if (!els.networkChart || state.tab !== "intelligence") return;
	const transformed = file.rows.map(row => ({
	  ...row,
	  export_usd: normalizeMatrixValue(row.export_usd, selected),
	  import_usd: normalizeMatrixValue(row.import_usd, selected),
	  trade_usd: normalizeMatrixValue(row.trade_usd, selected),
	}));
	const graph = buildPartnerNetwork(transformed, selected.iso3, state.metric, Math.min(state.topN, 30));
	const width = Math.max(500, els.networkChart.getBoundingClientRect().width || 0);
	const height = 330;
	const svg = d3.select(els.networkChart).attr("viewBox", `0 0 ${width} ${height}`);
	svg.selectAll("*").remove();
	const center = { x: width / 2, y: height / 2 };
	const partners = graph.nodes.filter(node => node.kind === "partner");
	const radiusX = Math.max(150, width / 2 - 62);
	const radiusY = height / 2 - 38;
	const positions = new Map([[selected.iso3, center]]);
	partners.forEach((node, index) => {
	  const angle = -Math.PI / 2 + (index * Math.PI * 2 / Math.max(1, partners.length));
	  positions.set(node.id, { x: center.x + Math.cos(angle) * radiusX, y: center.y + Math.sin(angle) * radiusY });
	});
	const maxLink = d3.max(graph.links, link => link.value) || 1;
	const linkWidth = d3.scaleSqrt().domain([0, maxLink]).range([0.5, 8]);
	svg.append("g").selectAll("line").data(graph.links).join("line")
	  .attr("class", "networkLink")
	  .attr("x1", link => positions.get(link.source)?.x)
	  .attr("y1", link => positions.get(link.source)?.y)
	  .attr("x2", link => positions.get(link.target)?.x)
	  .attr("y2", link => positions.get(link.target)?.y)
	  .attr("stroke-width", link => linkWidth(link.value));
	const nodeRadius = d3.scaleSqrt().domain([0, maxLink]).range([3, 11]);
	const groups = svg.append("g").selectAll("g").data(graph.nodes).join("g")
	  .attr("class", node => `networkNode ${node.kind}`)
	  .attr("transform", node => `translate(${positions.get(node.id)?.x || 0},${positions.get(node.id)?.y || 0})`);
	groups.append("circle").attr("r", node => node.kind === "reporter" ? 16 : nodeRadius(node.total));
	groups.append("text")
	  .attr("y", node => node.kind === "reporter" ? 4 : -9)
	  .attr("text-anchor", "middle")
	  .text(node => node.id);
	groups.append("title").text(node => node.kind === "reporter"
	  ? `${selected.name || selected.iso3} (${selected.iso3})`
	  : `${node.id}: ${formatMetricValue(node.total)}`);
	if (els.networkTitle) els.networkTitle.textContent = `${selected.name || selected.iso3} partner network`;
	if (els.networkDescription) els.networkDescription.textContent = `Top ${partners.length} reported partners for ${file.period}; link width represents ${metricLabel().toLowerCase()}.`;
	if (els.networkNote) els.networkNote.textContent = `${graph.scope}. Partner totals describe reported bilateral trade, not ports, firms, intermediate-input paths, or physical rerouting.`;
}

async function renderTradeNetwork(rows){
	const selected = rows.find(row => row.iso3 === state.selectedRow?.iso3);
	const partition = matrixPartitionFor(selected?.iso3, selected?.comparison_period);
	if (!selected || !partition) {
	  renderAnchorNetwork(rows);
	  return;
	}
	const selectedISO3 = selected.iso3;
	if (els.networkNote) els.networkNote.textContent = `Loading ${selectedISO3} reported partner matrix…`;
	const file = await loadMatrixPartition(partition);
	if (state.tab !== "intelligence" || state.selectedRow?.iso3 !== selectedISO3) return;
	if (!file) {
	  renderAnchorNetwork(rows);
	  return;
	}
	renderPartnerNetwork(file, selected);
}

function renderIntelligence(){
  const rows = intelligenceRows();
  renderIntelligenceSummary(rows);
  renderExposureRanking(rows);
	if (els.intelligenceScopeBadge) {
	  els.intelligenceScopeBadge.textContent = Number(state.matrixIndex?.partitions?.length || 0) > 0 ? "Multi-partner on selection" : "Two-anchor scope";
	}
	renderTradeNetwork(rows);
}

function renderCatalog(){
  if (els.productAvailability) {
    const product = state.catalog?.resources?.find(resource => resource.id === "product_chapters");
    const strategic = state.catalog?.resources?.find(resource => resource.id === "strategic_hs6");
    const tariffs = state.catalog?.resources?.find(resource => resource.id === "tariff_schedules");
    const hs2Label = product?.status === "ready" ? `HS${Number(product.product_level || 2)}` : "HS2 unknown";
    const hs6Label = strategic?.status === "ready" ? "strategic HS6" : strategic?.status === "partial" ? "HS6 registry" : "HS6 planned";
    const tariffLabel = tariffs?.status === "ready" ? "MFN tariffs" : "tariffs partial";
    els.productAvailability.textContent = `${hs2Label} + ${hs6Label} + ${tariffLabel}`;
  }
  if (els.tariffCapabilityStatus) {
    const partitions = Number(state.tariffIndex?.partitions?.length || 0);
    const importers = Number(state.tariffIndex?.importers?.length || 0);
    const observations = Number(state.tariffIndex?.observation_count || 0);
    els.tariffCapabilityStatus.textContent = partitions > 0 ? "Available" : "Contract ready";
    els.tariffCapabilityStatus.className = `statusPill ${partitions > 0 ? "success" : "warning"}`;
    if (els.tariffCapabilityText) {
      els.tariffCapabilityText.textContent = `${importers} importers · ${partitions} importer/year partitions · ${observations} revision-aware HS6 rates.`;
    }
  }
  if (els.strategicCapabilityStatus) {
    const partitions = Number(state.strategicIndex?.partitions?.length || 0);
    const products = Number(state.strategicIndex?.products?.length || 0);
    els.strategicCapabilityStatus.textContent = partitions > 0 ? "Available" : products > 0 ? "Registry ready" : "Unavailable";
    els.strategicCapabilityStatus.className = `statusPill ${partitions > 0 ? "success" : "warning"}`;
    if (els.strategicCapabilityText) {
      els.strategicCapabilityText.textContent = `${products} curated codes · ${Number(state.strategicIndex?.sectors?.length || 0)} sectors · ${partitions} reporter/year partitions.`;
    }
  }
  if (!els.dataCatalog) return;
  const resources = state.catalog?.resources;
  if (!Array.isArray(resources)) {
    els.dataCatalog.textContent = "No machine-readable catalog is available for this publication.";
    return;
  }
  const rows = resources.map(resource => {
    const status = String(resource.status || "unknown");
    const statusClass = status === "ready" ? "success" : status === "partial" ? "warning" : "planned";
    return `<div class="catalogItem"><div><span class="statusPill ${statusClass}">${escapeHTML(status)}</span> <b>${escapeHTML(resource.title || resource.id)}</b></div><span>${escapeHTML(resource.grain || "unspecified grain")} · ${escapeHTML(resource.partitioning || "single file")}</span><code>${escapeHTML(resource.href || "not published")}</code></div>`;
  }).join("");
  els.dataCatalog.innerHTML = `<div class="catalogList">${rows}</div><div class="analysisNote">Catalog schema ${escapeHTML(state.catalog.schema_version || "unknown")} · generated ${escapeHTML(state.catalog.generated_at || "unknown")}. Planned resources are contracts, not data claims.</div>`;
}

async function buildScenarioContext(){
  const selected = state.selectedRow;
  const side = els.scenarioPartner?.value || "usa";
  const productCode = /^\d{6}$/.test(String(els.scenarioProduct?.value || "")) ? els.scenarioProduct.value : "";
  if (!selected) return null;
  let tradePartition = null;
  let tariffPartition = null;
  if (productCode) {
    [tradePartition, tariffPartition] = await Promise.all([
      loadStrategicPartition(selected.iso3).catch(error => { console.warn(error); return null; }),
      loadTariffPartition(selected.iso3).catch(error => { console.warn(error); return null; }),
    ]);
  }
  const tradeRow = (tradePartition?.file?.rows || []).find(row => row.code === productCode);
  const tariffRow = selectPreferredTariffs(tariffPartition?.file?.rows || []).get(productCode);
  const product = (state.tariffIndex?.products || state.strategicIndex?.products || []).find(item => item.code === productCode);
  const productBaseline = tradeRow && Number.isFinite(Number(tradeRow?.[side]?.import)) ? Number(tradeRow[side].import) : null;
  const aggregateBaseline = Number(selected?.[side]?.import) || 0;
  return {
    selected, side, productCode, product, tradePartition, tariffPartition, tradeRow, tariffRow,
    baseline: productBaseline ?? aggregateBaseline,
    baselineKind: productBaseline == null ? "aggregate" : "hs6",
  };
}

async function renderScenarioBaseline(){
  if (!els.scenarioResult) return;
  const selected = state.selectedRow;
  if (!selected) {
    els.scenarioResult.textContent = "Select a country in Overview or Intelligence, then run a scenario.";
    if (els.scenarioTariffSource) els.scenarioTariffSource.textContent = "Select a reporter and product to look up a published MFN rate.";
    return;
  }
  const requestedISO3 = selected.iso3;
  const context = await buildScenarioContext();
  if (!context || state.selectedRow?.iso3 !== requestedISO3) return;
  const partnerName = context.side === "chn" ? "China" : "the United States";
  const productName = context.productCode ? `${context.productCode} ${context.product?.label || "strategic product"}` : "all products";
  if (context.productCode && els.scenarioTariffBase && !state.preserveScenarioInputs) {
    els.scenarioTariffBase.value = context.tariffRow ? String(Number(context.tariffRow.rate_percent).toFixed(2)) : "0";
  }
  state.preserveScenarioInputs = false;
  syncURL();
  if (els.scenarioTariffSource) {
    if (context.tariffRow) {
      els.scenarioTariffSource.textContent = `Default: ${tariffRateLabel(context.tariffRow)} · ${context.tariffRow.classification}/${context.tariffRow.nomenclature} · ${context.tariffPartition.partition.year} · ${String(context.tariffPartition.file.provider || "").toUpperCase()} · WLD/MFN schedule.`;
    } else if (context.productCode) {
      els.scenarioTariffSource.textContent = `No published MFN rate for ${context.selected.iso3}/${context.productCode}; enter the existing rate manually.`;
    } else {
      els.scenarioTariffSource.textContent = "Aggregate mode has no defensible single HS6 tariff. Enter the existing rate manually.";
    }
  }
  const baselineNote = context.baselineKind === "hs6"
    ? `HS6 import baseline from ${context.tradePartition.partition.period}.`
    : `HS6 imports are unavailable, so this preview uses aggregate partner imports; do not interpret it as a product forecast.`;
  els.scenarioResult.innerHTML = `<div class="signalMetric"><span>${escapeHTML(context.selected.name)} imports of ${escapeHTML(productName)} from ${escapeHTML(partnerName)}</span><b>${formatNominalUSD(context.baseline)}</b></div><div class="analysisNote">${escapeHTML(baselineNote)} Ready to run with the visible assumptions.</div>`;
}

async function runScenario(){
  if (!els.scenarioResult || !state.selectedRow) {
    await renderScenarioBaseline();
    return;
  }
  const context = await buildScenarioContext();
  if (!context) return;
  const result = estimateTariffScenario({
    baselineImport: context.baseline,
    existingTariffPct: els.scenarioTariffBase?.value,
    tariffChangePct: els.scenarioTariffChange?.value,
    elasticity: els.scenarioElasticity?.value,
    passThrough: els.scenarioPassThrough?.value,
  });
  els.scenarioResult.innerHTML = `
    <div class="scenarioResults">
      <div class="scenarioResultMetric"><span>Baseline imports</span><b>${formatNominalUSD(result.baselineImport)}</b></div>
      <div class="scenarioResultMetric"><span>Illustrative imports</span><b>${formatNominalUSD(result.projectedImport)}</b></div>
      <div class="scenarioResultMetric"><span>Import response</span><b>${fmtPct(result.responseRatio)}</b></div>
      <div class="scenarioResultMetric"><span>Illustrative change</span><b>${formatNominalUSD(result.importChange)}</b></div>
      <div class="scenarioResultMetric"><span>Tariff rate</span><b>${result.existingTariffPct.toFixed(1)}% → ${result.projectedTariffRate.toFixed(1)}%</b></div>
      <div class="scenarioResultMetric"><span>Revenue change</span><b>${formatNominalUSD(result.revenueChange)}</b></div>
    </div>
    <div class="analysisNote">${escapeHTML(result.warning)} ${context.baselineKind === "hs6" ? `HS6 baseline ${escapeHTML(context.tradePartition.partition.period)}.` : "Aggregate baseline fallback; product interpretation is not valid."} ${context.tariffRow ? `MFN source ${escapeHTML(context.tariffPartition.partition.year)} ${escapeHTML(context.tariffRow.classification)}.` : "Tariff entered manually."} Elasticity ${result.elasticity.toFixed(2)} · pass-through ${result.passThrough.toFixed(2)} · response capped at −95%.</div>`;
}

function setActiveTab(tab, options = {}){
  const allowed = new Set(["overview", "intelligence", "products", "quality", "lab"]);
  state.tab = allowed.has(tab) ? tab : "overview";
  const buttons = els.dashboardTabs?.querySelectorAll("[role='tab']") || [];
  buttons.forEach(button => {
    const active = button.dataset.tab === state.tab;
    button.classList.toggle("is-active", active);
    button.setAttribute("aria-selected", String(active));
    button.tabIndex = active ? 0 : -1;
    if (active && options.focus) button.focus();
  });
  document.querySelectorAll("[data-tab-panel]").forEach(panel => {
    panel.hidden = panel.dataset.tabPanel !== state.tab;
  });
  if (options.syncURL !== false) syncURL();
  renderViewContext();
  if (state.tab === "overview") {
    requestAnimationFrame(() => {
      buildTreemap(els.svgUSA, "usa", state.rows);
      buildTreemap(els.svgCHN, "chn", state.rows);
      applyHighlight(state.highlightKey);
    });
  }
  if (state.tab === "intelligence") renderIntelligence();
  if (state.tab === "lab") renderScenarioBaseline();
}

function initializeTabs(){
  if (!els.dashboardTabs) return;
  els.dashboardTabs.addEventListener("click", event => {
    const button = event.target.closest("[role='tab']");
    if (button?.dataset.tab) setActiveTab(button.dataset.tab);
  });
  els.dashboardTabs.addEventListener("keydown", event => {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    const buttons = Array.from(els.dashboardTabs.querySelectorAll("[role='tab']"));
    const index = buttons.indexOf(document.activeElement);
    if (index < 0) return;
    event.preventDefault();
    let next = index;
    if (event.key === "ArrowLeft") next = (index - 1 + buttons.length) % buttons.length;
    if (event.key === "ArrowRight") next = (index + 1) % buttons.length;
    if (event.key === "Home") next = 0;
    if (event.key === "End") next = buttons.length - 1;
    setActiveTab(buttons[next].dataset.tab, { focus: true });
  });
}

function renderAll(){
  const rows = state.rows;
  renderViewContext();
  buildTreemap(els.svgUSA, "usa", rows);
  buildTreemap(els.svgCHN, "chn", rows);

  applyHighlight(state.highlightKey);
  if (state.selectedRow) {
    setSelection(state.selectedRow);
  } else {
    setSelection(null);
  }
  renderDataTable();
  renderTimeSeries();
  renderProducts();
  renderStrategicProducts();
  renderQualityDashboard();
  renderExplanation();
  renderIntelligence();
  renderCatalog();
  if (state.tab === "lab") renderScenarioBaseline();
}

async function setIndicators(row){
  if (!els.indicators) return;
  if (!row) {
    els.indicators.innerHTML = "<span class='subtle'>Click a country tile to load key economic indicators and news.</span>";
    return;
  }
  if (!row.iso2) {
    els.indicators.innerHTML = "<span class='subtle'>No ISO2 mapping available for this country.</span>";
    return;
  }

  const key = row.iso3;
  els.indicators.innerHTML = "<span class='subtle'>Loading indicators and news...</span>";

  try {
    const [summary, news] = await Promise.all([
      loadIndicators(row.iso2, key).catch(() => null),
      loadNews(row.iso2, key).catch(() => null),
    ]);
    if (!summary && !news) {
      els.indicators.innerHTML = "<span class='subtle'>No indicator or news data available.</span>";
      return;
    }
    els.indicators.innerHTML = renderSnapshotHTML(summary, news);
  } catch (err) {
    console.error(err);
    els.indicators.innerHTML = "<span class='subtle'>Failed to load indicators or news.</span>";
  }
}

async function loadIndicators(iso2, key){
  if (indicatorCache[key]) return indicatorCache[key];
  if (indicatorPromises[key]) return indicatorPromises[key];

  const promise = (async () => {
    const items = [];
    for (const indicator of INDICATORS) {
      const result = await fetchIndicator(iso2, indicator.id);
      items.push({
        id: indicator.id,
        label: indicator.label,
        format: indicator.format,
        value: result.value,
        year: result.year
      });
    }
    const summary = { iso2, items };
    indicatorCache[key] = summary;
    return summary;
  })();

  indicatorPromises[key] = promise;
  try {
    return await promise;
  } finally {
    delete indicatorPromises[key];
  }
}

async function fetchIndicator(iso2, indicatorId){
  const url = `https://api.worldbank.org/v2/country/${iso2}/indicator/${indicatorId}?format=json`;
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) return { value: null, year: null };
  const data = await res.json();
  const series = Array.isArray(data) ? data[1] : null;
  if (!Array.isArray(series)) return { value: null, year: null };
  const latest = series.find(item => item && item.value != null);
  return {
    value: latest ? latest.value : null,
    year: latest ? latest.date : null
  };
}

function renderIndicatorHTML(summary){
  const rows = summary.items.map(item => {
    const value = formatIndicatorValue(item.value, item.format);
    const year = escapeHTML(item.year || "-");
    return `<div class="kv"><span>${escapeHTML(item.label)} (${year})</span><b>${escapeHTML(value)}</b></div>`;
  });
  return rows.join("");
}

function formatIndicatorValue(value, format){
  if (value == null || !isFinite(value)) return "-";
  if (format === "usd") return "$" + fmt(value);
  return fmt(value);
}

async function loadNews(iso2, key){
  if (newsCache[key]) return newsCache[key];
  if (newsPromises[key]) return newsPromises[key];

  const promise = (async () => {
    const url = buildGdeltURL(iso2, { windowDays: NEWS_WINDOW_DAYS, maxRecords: 50 });
    if (!url) return null;
    let summary;
    try {
      const res = await fetch(url, { cache: "no-store" });
      if (!res.ok) {
        summary = { iso2: iso2.toUpperCase(), items: [], windowDays: NEWS_WINDOW_DAYS, status: "unavailable" };
      } else {
        const data = await res.json();
        const articles = Array.isArray(data?.articles) ? data.articles : [];
        const items = curateNewsArticles(articles, {
          windowDays: NEWS_WINDOW_DAYS,
          maxItems: NEWS_MAX,
        });
        summary = { iso2: iso2.toUpperCase(), items, windowDays: NEWS_WINDOW_DAYS, status: items.length ? "ok" : "empty" };
      }
    } catch {
      summary = { iso2: iso2.toUpperCase(), items: [], windowDays: NEWS_WINDOW_DAYS, status: "unavailable" };
    }
    newsCache[key] = summary;
    return summary;
  })();

  newsPromises[key] = promise;
  try {
    return await promise;
  } finally {
    delete newsPromises[key];
  }
}

function renderSnapshotHTML(indicators, news){
  const sections = [];
  if (indicators) {
    sections.push(renderIndicatorHTML(indicators));
    sections.push(`<div class="subtle" style="margin-top:8px;font-size:11px;">Source: World Bank Open Data</div>`);
  } else {
    sections.push(`<div class="subtle">No indicator data available.</div>`);
  }

  const selected = state.selectedRow;
  const usaPeriod = selected?.usa?.period ? `${selected.usa.period_type || "?"} ${selected.usa.period}` : "unavailable";
  const chnPeriod = selected?.chn?.period ? `${selected.chn.period_type || "?"} ${selected.chn.period}` : "unavailable";
  const tradeClock = usaPeriod === chnPeriod ? usaPeriod : `USA ${usaPeriod} · China ${chnPeriod}`;
  const newsWindow = Number(news?.windowDays) || NEWS_WINDOW_DAYS;
  sections.push(`<div class="temporalNotice"><b>Different clocks</b><span>Trade observation: ${escapeHTML(tradeClock)}. Headlines: rolling ${newsWindow}-day window at the time this panel loads. They are not same-period evidence.</span></div>`);
  sections.push(`<div class="subSectionTitle">Trade &amp; supply-chain headlines <span class="experimentalBadge">Experimental</span></div>`);
  sections.push(renderNewsHTML(news));

  return sections.join("");
}

function renderNewsHTML(news){
  if (news?.status === "unavailable") {
    return `<div class="subtle newsNote">Trade-focused headline context is temporarily unavailable from GDELT. This optional panel does not affect TradeGravity metrics.</div>`;
  }
  if (!news || !Array.isArray(news.items) || news.items.length === 0) {
    return `<div class="subtle newsNote">No recent trade-focused headlines met the ${NEWS_WINDOW_DAYS}-day relevance filter. This optional panel does not affect TradeGravity metrics.</div>`;
  }
  const rows = news.items.map(item => {
    const title = escapeHTML(item.title || "Untitled");
    const domain = item.domain ? `<span class="subtle"> · ${escapeHTML(item.domain)}</span>` : "";
    const seen = item.seen ? `<span class="subtle"> · ${escapeHTML(item.seen)}</span>` : "";
    const url = safeHTTPSURL(item.url);
    const headline = url
      ? `<a href="${escapeHTML(url)}" target="_blank" rel="noopener noreferrer nofollow">${title}</a>`
      : `<span>${title}</span>`;
    return `<div class="newsItem">${headline}${domain}${seen}</div>`;
  });
  const scope = escapeHTML(news.iso2 || "selected");
  const windowDays = Number(news.windowDays) || NEWS_WINDOW_DAYS;
  rows.push(`<div class="subtle newsNote">GDELT DOC 2.0 · publisher source country: ${scope} · ${windowDays}-day window · keyword-filtered and deduplicated. Headlines may still be misclassified; verify the publisher article. This optional context does not affect trade metrics.</div>`);
  return rows.join("");
}

async function main(){
  try {
    const res = await fetch("./iso3_to_iso2.json", { cache: "no-store" });
    if (res.ok) {
      const fullMap = await res.json();
      if (fullMap && typeof fullMap === "object") {
        ISO3_TO_ISO2 = fullMap;
      }
    }
  } catch (err) {
    console.warn("[TradeGravity] iso3_to_iso2.json not loaded, using fallback map.", err);
  }

  const [res, metaRes, seriesRes, qualityRes, productIndexRes, strategicIndexRes, tariffIndexRes, matrixIndexRes, catalogRes] = await Promise.all([
    fetch(DATA_URL, { cache: "no-store" }),
    fetch(META_URL, { cache: "no-store" }).catch(() => null),
    fetch(SERIES_URL, { cache: "no-store" }).catch(() => null),
    fetch(QUALITY_URL, { cache: "no-store" }).catch(() => null),
    fetch(PRODUCTS_INDEX_URL, { cache: "no-store" }).catch(() => null),
    fetch(STRATEGIC_INDEX_URL, { cache: "no-store" }).catch(() => null),
    fetch(TARIFF_INDEX_URL, { cache: "no-store" }).catch(() => null),
	  fetch(MATRIX_INDEX_URL, { cache: "no-store" }).catch(() => null),
    fetch(CATALOG_URL, { cache: "no-store" }).catch(() => null),
  ]);
  if (!res.ok) throw new Error(`Dataset request failed (${res.status})`);
  const data = await res.json();
  let metadata = null;
  if (metaRes?.ok) {
    metadata = await metaRes.json().catch(() => null);
  }
  const series = seriesRes?.ok ? await seriesRes.json().catch(() => null) : null;
  const quality = qualityRes?.ok ? await qualityRes.json().catch(() => null) : null;
  const productIndex = productIndexRes?.ok ? await productIndexRes.json().catch(() => null) : null;
  const strategicIndex = strategicIndexRes?.ok ? await strategicIndexRes.json().catch(() => null) : null;
  const tariffIndex = tariffIndexRes?.ok ? await tariffIndexRes.json().catch(() => null) : null;
	const matrixIndex = matrixIndexRes?.ok ? await matrixIndexRes.json().catch(() => null) : null;
  const catalog = catalogRes?.ok ? await catalogRes.json().catch(() => null) : null;

  state.generatedAt = data.generated_at || data.generatedAt || "-";
  state.schemaVersion = String(metadata?.schema_version || data.schema_version || "");
  state.provider = String(metadata?.provider || data.provider || "").trim().toLowerCase();
  state.latestRows = normalizeRows(data.rows || []);
  state.seriesRows = Array.isArray(series?.rows) ? series.rows : [];
  state.quality = quality;
  state.productIndex = productIndex;
  state.strategicIndex = strategicIndex;
  state.tariffIndex = tariffIndex;
	state.matrixIndex = matrixIndex;
  state.catalog = catalog;
  state.meta = metadata;
  state.resourceStates = [
    { label: "metadata", ready: Boolean(metadata) },
    { label: "time series", ready: Boolean(series) },
    { label: "quality report", ready: Boolean(quality) },
    { label: "HS2 index", ready: Boolean(productIndex) },
    { label: "strategic HS6 index", ready: Boolean(strategicIndex) },
    { label: "tariff index", ready: Boolean(tariffIndex) },
    { label: "bilateral matrix index", ready: Boolean(matrixIndex) },
    { label: "data catalog", ready: Boolean(catalog) },
  ];
  const initialView = parseViewState(window.location.search);
  state.metric = initialView.metric;
  state.colorMode = initialView.color;
  state.topN = initialView.top;
  state.period = initialView.period;
  state.comparisonMode = initialView.mode;
  state.region = initialView.region;
  state.income = initialView.income;
  state.group = initialView.group;
  state.normalization = initialView.normalization;
  state.tableQuery = initialView.query;
  state.tab = initialView.tab;
  state.strategicSector = initialView.sector;
  populateStrategicControls();
  syncScenarioControls(initialView);
  initializeTabs();
  setActiveTab(state.tab, { syncURL: false });
  populateExplorerControls();
  reconcileExplorerState();
  syncExplorerControls();
  refreshRows({ syncURL: false });
  if (initialView.country) {
    const selected = state.rows.find(row => row.iso3 === initialView.country);
    if (selected) {
      state.selectedRow = selected;
      state.highlightKey = selected.iso3;
    }
  }
  renderDatasetStatus(data, metadata);
  renderDataHealth(true);
  initializeExperienceControls();
  console.info('[TradeGravity] loaded rows=', state.latestRows.length, 'generated_at=', state.generatedAt);

  if (els.metric) {
    els.metric.addEventListener("change", () => {
      state.metric = els.metric.value;
      syncMetricButtons();
      syncURL();
      renderAll();
    });
  }
  if (els.metricGroup) {
    els.metricGroup.addEventListener("click", (ev) => {
      const btn = ev.target.closest(".segBtn");
      if (!btn) return;
      const value = btn.getAttribute("data-value");
      if (!value) return;
      state.metric = value;
      syncMetricButtons();
      syncURL();
      renderAll();
    });
  }
  if (els.colorGroup) {
    els.colorGroup.addEventListener("click", (ev) => {
      const btn = ev.target.closest(".segBtn");
      if (!btn) return;
      const value = btn.getAttribute("data-value");
      if (!value) return;
      state.colorMode = value;
      syncColorButtons();
      syncColorLegend();
      syncURL();
      renderAll();
    });
  }

  // Top N grouping
  if (els.topN){
    els.topN.addEventListener("input", () => {
      const v = parseInt(els.topN.value, 10);
      if (Number.isFinite(v)) {
        state.topN = Math.max(5, Math.min(200, v));
        if (String(state.topN) !== els.topN.value) els.topN.value = String(state.topN);
      }
      syncURL();
      renderAll();
    });
  }
  if (els.tableSearch) {
    els.tableSearch.addEventListener("input", () => {
      state.tableQuery = els.tableSearch.value;
      refreshRows();
    });
  }
  if (els.downloadCSV) {
    els.downloadCSV.addEventListener("click", downloadTableCSV);
  }
  if (els.downloadJSON) {
    els.downloadJSON.addEventListener("click", downloadFilteredJSON);
  }
  for (const [element, property] of [
    [els.periodFilter, "period"],
    [els.comparisonMode, "comparisonMode"],
    [els.regionFilter, "region"],
    [els.incomeFilter, "income"],
    [els.groupFilter, "group"],
    [els.normalization, "normalization"],
  ]) {
    if (!element) continue;
    element.addEventListener("change", () => {
      state[property] = element.value;
      refreshRows();
    });
  }
  if (els.copyShareURL) {
    els.copyShareURL.addEventListener("click", async () => {
      syncURL();
      const copied = await copyTextToClipboard(window.location.href);
      els.copyShareURL.textContent = copied ? "Copied" : "URL ready in address bar";
      setTimeout(() => { els.copyShareURL.textContent = "Copy view URL"; }, 1500);
    });
  }
  if (els.scenarioPartner) {
    els.scenarioPartner.addEventListener("change", () => {
      syncURL();
      renderScenarioBaseline();
    });
  }
  if (els.scenarioProduct) {
    els.scenarioProduct.addEventListener("change", () => {
      syncURL();
      renderScenarioBaseline();
    });
  }
  for (const [element, property] of [
    [els.scenarioTariffBase, "tariffBase"],
    [els.scenarioTariffChange, "tariffChange"],
    [els.scenarioElasticity, "elasticity"],
    [els.scenarioPassThrough, "passThrough"],
  ]) {
    element?.addEventListener("input", syncURL);
    element?.addEventListener("change", () => {
      element.value = String(currentViewState()[property]);
      syncURL();
    });
  }
  if (els.scenarioForm) {
    els.scenarioForm.addEventListener("submit", event => {
      event.preventDefault();
      runScenario();
    });
  }
  if (els.strategicSectorFilter) {
    els.strategicSectorFilter.addEventListener("change", () => {
      state.strategicSector = els.strategicSectorFilter.value;
      syncURL();
      renderStrategicProducts();
    });
  }
  window.addEventListener("popstate", () => {
    const view = parseViewState(window.location.search);
    state.metric = view.metric;
    state.colorMode = view.color;
    state.topN = view.top;
    state.period = view.period;
    state.comparisonMode = view.mode;
    state.region = view.region;
    state.income = view.income;
    state.group = view.group;
    state.normalization = view.normalization;
    state.tableQuery = view.query;
    state.tab = view.tab;
    state.strategicSector = view.sector;
    state.selectedRow = null;
    state.highlightKey = null;
    syncExplorerControls();
    populateStrategicControls();
    syncScenarioControls(view);
    refreshRows({ syncURL: false });
    setActiveTab(view.tab, { syncURL: false });
    const selected = state.rows.find(row => row.iso3 === view.country);
    if (selected) selectCountry(selected);
  });

  window.addEventListener("resize", () => {
    clearTimeout(window.__tmResize);
    window.__tmResize = setTimeout(() => renderAll(), 100);
  });

  syncMetricButtons();
  syncColorButtons();
  renderAll();
  syncColorLegend();
	if (state.selectedRow) selectCountry(state.selectedRow);
	else setIndicators(null);
}

function syncMetricButtons(){
  if (!els.metricGroup) return;
  const buttons = els.metricGroup.querySelectorAll(".segBtn");
  buttons.forEach(btn => {
    const value = btn.getAttribute("data-value");
    btn.classList.toggle("is-active", value === state.metric);
    btn.setAttribute("aria-pressed", String(value === state.metric));
  });
}

function syncColorButtons(){
  if (!els.colorGroup) return;
  const buttons = els.colorGroup.querySelectorAll(".segBtn");
  buttons.forEach(btn => {
    const value = btn.getAttribute("data-value");
    btn.classList.toggle("is-active", value === state.colorMode);
    btn.setAttribute("aria-pressed", String(value === state.colorMode));
  });
}

function syncColorLegend(){
  if (!els.growthLegend) return;
  els.growthLegend.classList.toggle("is-hidden", state.colorMode !== "growth");
}

main().catch(err => {
  console.error(err);
  renderDataHealth(false);
  els.retryData?.addEventListener("click", () => window.location.reload(), { once: true });
  if (els.dataStatus) {
    els.dataStatus.textContent = "Headline trade dataset unavailable.";
  }
  if (els.indicators) {
    els.indicators.textContent = "Failed to load data: " + String(err);
  }
  if (els.tableSummary) {
    els.tableSummary.textContent = "Failed to load the trade dataset.";
  }
});
