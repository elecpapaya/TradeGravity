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
};

let state = {
  latestRows: [],
  rows: [],
  seriesRows: [],
  quality: null,
  productIndex: null,
  productCache: {},
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
};

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
const NEWS_MAX = 5;
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
  applyHighlight(row.iso3);
  setSelection(row);
  setIndicators(row);
  renderTimeSeries();
  renderProducts();
  renderExplanation();
  renderDataTable();
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
  const x = Math.min(window.innerWidth - 340, ev.clientX + pad);
  const y = Math.min(window.innerHeight - 180, ev.clientY + pad);
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
  svg.selectAll("*").remove();

  const { width, height } = svgEl.getBoundingClientRect();
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
      return iso2 ? flagURL(iso2) : null;
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
    .text(d => labelSet.has(d.data.iso3) ? d.data.iso3 : "")
    .attr("fill", "rgba(255,255,255,.78)")
    .attr("font-size", 12)
    .attr("font-family", "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace")
    .style("pointer-events","none")
    .attr("dx", d => {
      const w = (d.x1 - d.x0), h = (d.y1 - d.y0);
      return (d.data.iso2 && w >= 32 && h >= 22) ? 24 : 0;
    });

  nodes.append("text")
    .attr("class","tileValue")
    .attr("x", 6)
    .attr("y", 34)
    .text(d => labelSet.has(d.data.iso3) ? formatMetricValue(d.data.value) : "")
    .attr("fill", "rgba(255,255,255,.55)")
    .attr("font-size", 11)
    .attr("font-family", "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace")
    .style("pointer-events","none")
    .attr("dx", d => {
      const w = (d.x1 - d.x0), h = (d.y1 - d.y0);
      return (d.data.iso2 && w >= 32 && h >= 22) ? 24 : 0;
    });

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
  };
}

function syncURL(){
  const query = serializeViewState(currentViewState());
  const next = `${window.location.pathname}${query ? `?${query}` : ""}${window.location.hash}`;
  window.history.replaceState(null, "", next);
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

function renderAll(){
  const rows = state.rows;
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
  renderQualityDashboard();
  renderExplanation();
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
    const query = encodeURIComponent(`sourcecountry:${iso2.toUpperCase()}`);
    const url = `https://api.gdeltproject.org/api/v2/doc/doc?query=${query}&mode=ArtList&maxrecords=${NEWS_MAX}&format=json`;
    const res = await fetch(url, { cache: "no-store" });
    if (!res.ok) return null;
    const data = await res.json();
    const articles = Array.isArray(data?.articles) ? data.articles : [];
    const items = articles.map(article => ({
      title: article.title || "Untitled",
      url: article.url,
      domain: article.domain || "",
      seen: formatGdeltDate(article.seendate || "")
    }));
    const summary = { iso2, items };
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

  sections.push(`<div class="subSectionTitle">Latest news</div>`);
  sections.push(renderNewsHTML(news));

  return sections.join("");
}

function renderNewsHTML(news){
  if (!news || !Array.isArray(news.items) || news.items.length === 0) {
    return `<div class="subtle">No recent news found. (Source: GDELT, by source country)</div>`;
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
  rows.push(`<div class="subtle" style="margin-top:6px;font-size:11px;">Source: GDELT (by source country)</div>`);
  return rows.join("");
}

function formatGdeltDate(value){
  if (!value) return "";
  const match = String(value).match(/^(\d{4})(\d{2})(\d{2})/);
  if (!match) return "";
  return `${match[1]}-${match[2]}-${match[3]}`;
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

  const [res, metaRes, seriesRes, qualityRes, productIndexRes] = await Promise.all([
    fetch(DATA_URL, { cache: "no-store" }),
    fetch(META_URL, { cache: "no-store" }).catch(() => null),
    fetch(SERIES_URL, { cache: "no-store" }).catch(() => null),
    fetch(QUALITY_URL, { cache: "no-store" }).catch(() => null),
    fetch(PRODUCTS_INDEX_URL, { cache: "no-store" }).catch(() => null),
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

  state.generatedAt = data.generated_at || data.generatedAt || "-";
  state.schemaVersion = String(metadata?.schema_version || data.schema_version || "");
  state.provider = String(metadata?.provider || data.provider || "").trim().toLowerCase();
  state.latestRows = normalizeRows(data.rows || []);
  state.seriesRows = Array.isArray(series?.rows) ? series.rows : [];
  state.quality = quality;
  state.productIndex = productIndex;
  state.meta = metadata;
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
  populateExplorerControls();
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
      if (Number.isFinite(v)) state.topN = v;
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
      try {
        await navigator.clipboard.writeText(window.location.href);
        els.copyShareURL.textContent = "Copied";
      } catch {
        window.prompt("Copy this view URL", window.location.href);
      }
      setTimeout(() => { els.copyShareURL.textContent = "Copy view URL"; }, 1500);
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
    state.selectedRow = null;
    state.highlightKey = null;
    syncExplorerControls();
    refreshRows({ syncURL: false });
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
  setIndicators(null);
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
  if (els.indicators) {
    els.indicators.textContent = "Failed to load data: " + String(err);
  }
  if (els.tableSummary) {
    els.tableSummary.textContent = "Failed to load the trade dataset.";
  }
});
