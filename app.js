// Treemap MVP (v4): two anchor rectangles (USA/CHN).
// Inside each: sub-rectangles per country sized by selected metric (trade/export/import).
// Adds optional flag icons via CDN.
// - If row.iso2 exists, use it.
// - Else try ISO3->ISO2 map (partial).
// If no ISO2 found, skip flag.

const DATA_URL = "./data/latest.json";

const els = {
  svgUSA: document.getElementById("svg-usa"),
  svgCHN: document.getElementById("svg-chn"),
  metric: document.getElementById("metric"),
  metricGroup: document.getElementById("metricGroup"),
  selection: document.getElementById("selection"),
  indicators: document.getElementById("indicators"),
  tooltip: document.getElementById("tooltip"),
  topN: document.getElementById("topN"),
};

let state = {
  rows: [],
  metric: "trade",
  highlightKey: null, // ISO3
  selectedRow: null,
  topN: 25,
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

function iso2FromRow(row){
  const iso2 = (row.iso2 || row.ISO2 || "").trim();
  if (iso2) return iso2.toUpperCase();
  const iso3 = (row.iso3 || row.ISO3 || "").toUpperCase();
  return ISO3_TO_ISO2[iso3] || "";
}

// Flag CDN URL. Uses PNG (20px height).
// Docs: https://flagcdn.com/
function flagURL(iso2){
  if (!iso2) return "";
  const cc = iso2.toLowerCase();
  return `https://flagcdn.com/h20/${cc}.png`;
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

function getMetricValue(row, side){
  const o = row[side] || {};
  const m = state.metric;
  const v = o[m] ?? 0;
  return +v || 0;
}

function normalizeRows(rows){
  return (rows || []).map(r => {
    const iso3 = (r.iso3 || r.ISO3 || "").toUpperCase();
    const usa = r.usa || {};
    const chn = r.chn || {};
    const trade_us = +(usa.trade ?? (+(usa.export||0) + +(usa.import||0))) || 0;
    const trade_cn = +(chn.trade ?? (+(chn.export||0) + +(chn.import||0))) || 0;
    const total = +(r.total ?? (trade_us + trade_cn)) || 0;
    const share_cn = +(r.share_cn ?? (total ? trade_cn/total : 0)) || 0;
    const iso2 = iso2FromRow(r);
    return {
      iso3,
      name: r.name || iso3,
      iso2,
      usa: { ...usa, trade: trade_us },
      chn: { ...chn, trade: trade_cn },
      total,
      share_cn
    };
  });
}

function setSelection(row){
  if (!row){
    els.selection.innerHTML = "<span class='subtle'>Click a country tile to view details.</span>";
    return;
  }
  const us = row.usa || {};
  const cn = row.chn || {};
  const html = `
    <div style="font-weight:800; margin-bottom:8px; display:flex; align-items:center; gap:10px;">
      ${row.iso2 ? `<img alt="" src="${flagURL(row.iso2)}" style="width:22px;height:16px;border-radius:4px;border:1px solid rgba(255,255,255,.12)"/>` : ""}
      <div>${row.name} <span style="color:rgba(255,255,255,.55); font-family:var(--mono); font-size:12px;">(${row.iso3})</span></div>
    </div>
    <div class="kv"><span>USA period</span><b>${us.period || "-"}</b></div>
    <div class="kv"><span>USA ${state.metric}</span><b>${fmt(us[state.metric] ?? 0)}</b></div>
    <div style="height:8px"></div>
    <div class="kv"><span>CHN period</span><b>${cn.period || "-"}</b></div>
    <div class="kv"><span>CHN ${state.metric}</span><b>${fmt(cn[state.metric] ?? 0)}</b></div>
    <div style="height:10px"></div>
    <div class="kv"><span>share_cn</span><b>${(row.share_cn*100).toFixed(1)}%</b></div>
    <div class="kv"><span>total(trade_us+trade_cn)</span><b>${fmt(row.total)}</b></div>
  `;
  els.selection.innerHTML = html;
}

function showTooltip(ev, row, side){
  const o = row[side] || {};
  els.tooltip.style.display = "block";
  els.tooltip.innerHTML = `
    <div class="t1" style="display:flex;align-items:center;gap:10px;">
      ${row.iso2 ? `<img alt="" src="${flagURL(row.iso2)}" style="width:20px;height:14px;border-radius:4px;border:1px solid rgba(255,255,255,.10)"/>` : ""}
      <div>${row.name} <span style="color:rgba(255,255,255,.55); font-family:var(--mono); font-size:12px;">(${row.iso3})</span></div>
    </div>
    <div class="t2">
      <div class="kv"><span>${side.toUpperCase()} period</span><b>${o.period || "-"}</b></div>
      <div class="kv"><span>${side.toUpperCase()} ${state.metric}</span><b>${fmt(o[state.metric] ?? 0)}</b></div>
      <div class="kv"><span>share_cn</span><b>${(row.share_cn*100).toFixed(1)}%</b></div>
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

  const defs = svg.append("defs");

  const g = svg.append("g");

  const nodes = g.selectAll("g.tile")
    .data(root.leaves())
    .enter()
    .append("g")
    .attr("class","tile")
    .attr("data-iso3", d => d.data.iso3)
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
    .attr("fill", baseFill)
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
    .text(d => labelSet.has(d.data.iso3) ? fmt(d.data.value) : "")
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
      const row = d.data.row;
      state.selectedRow = row;
      state.highlightKey = row.iso3;
      applyHighlight(state.highlightKey);
      setSelection(row);
      setIndicators(row);
    });
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
    const year = item.year || "-";
    return `<div class="kv"><span>${item.label} (${year})</span><b>${value}</b></div>`;
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
    const domain = item.domain ? `<span class="subtle"> · ${item.domain}</span>` : "";
    const seen = item.seen ? `<span class="subtle"> · ${item.seen}</span>` : "";
    return `<div class="newsItem"><a href="${item.url}" target="_blank" rel="noopener">${item.title}</a>${domain}${seen}</div>`;
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

  const res = await fetch(DATA_URL, { cache: "no-store" });
  const data = await res.json();

  state.generatedAt = data.generated_at || data.generatedAt || "-";
  state.rows = normalizeRows(data.rows || []);
  console.info('[TradeGravity] loaded rows=', state.rows.length, 'generated_at=', state.generatedAt);

  if (els.metric) {
    els.metric.addEventListener("change", () => {
      state.metric = els.metric.value;
      syncMetricButtons();
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
      renderAll();
    });
  }

  // Top N grouping
  if (els.topN){
    els.topN.addEventListener("input", () => {
      const v = parseInt(els.topN.value, 10);
      if (Number.isFinite(v)) state.topN = v;
      renderAll();
    });
    // initialize
    const initV = parseInt(els.topN.value, 10);
    if (Number.isFinite(initV)) state.topN = initV;
  }

  window.addEventListener("resize", () => {
    clearTimeout(window.__tmResize);
    window.__tmResize = setTimeout(() => renderAll(), 100);
  });

  renderAll();
  setIndicators(null);
}

function syncMetricButtons(){
  if (!els.metricGroup) return;
  const buttons = els.metricGroup.querySelectorAll(".segBtn");
  buttons.forEach(btn => {
    const value = btn.getAttribute("data-value");
    btn.classList.toggle("is-active", value === state.metric);
  });
}

main().catch(err => {
  console.error(err);
  if (els.indicators) {
    els.indicators.textContent = "Failed to load data: " + String(err);
  }
});
