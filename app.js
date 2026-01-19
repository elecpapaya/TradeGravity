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
  search: document.getElementById("search"),
  metric: document.getElementById("metric"),
  labelMode: document.getElementById("labelMode"),
  selection: document.getElementById("selection"),
  dataInfo: document.getElementById("dataInfo"),
  tooltip: document.getElementById("tooltip"),
  fitBtn: document.getElementById("fitBtn"),
  topN: document.getElementById("topN"),
};

let state = {
  rows: [],
  metric: "trade",
  labelMode: "top",
  highlightKey: null, // ISO3
  topN: 25,
};

// Minimal ISO3->ISO2 map. For full coverage, add iso2 in latest.json or expand this table.
const ISO3_TO_ISO2 = {
  KOR:"KR", JPN:"JP", CHN:"CN", USA:"US", DEU:"DE", FRA:"FR", GBR:"GB", ITA:"IT", ESP:"ES",
  CAN:"CA", MEX:"MX", BRA:"BR", IND:"IN", IDN:"ID", VNM:"VN", AUS:"AU", RUS:"RU", TUR:"TR",
  SAU:"SA", ARE:"AE", ZAF:"ZA", EGY:"EG", NGA:"NG", ARG:"AR", CHL:"CL", COL:"CO", PER:"PE",
  NLD:"NL", BEL:"BE", SWE:"SE", NOR:"NO", DNK:"DK", FIN:"FI", POL:"PL", CZE:"CZ", HUN:"HU",
  ISR:"IL", IRL:"IE", PRT:"PT", CHE:"CH", AUT:"AT", GRC:"GR", UKR:"UA", THA:"TH", MYS:"MY",
  SGP:"SG", PHL:"PH", PAK:"PK", BGD:"BD", NZL:"NZ", KAZ:"KZ"
};

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
    els.selection.innerHTML = "<span class='subtle'>Hover or search a country.</span>";
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

  const mode = state.labelMode;
  let labelSet = new Set();
  if (mode === "all"){
    labelSet = new Set(children.map(d => d.iso3));
  } else if (mode === "top"){
    const topN = 20;
    children.sort((a,b)=>b.value-a.value).slice(0, topN).forEach(d => labelSet.add(d.iso3));
  }

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

  nodes.append("rect")
    .attr("rx", 6)
    .attr("ry", 6)
    .attr("width", d => Math.max(0, d.x1 - d.x0))
    .attr("height", d => Math.max(0, d.y1 - d.y0))
    .attr("fill", baseFill)
    .attr("stroke", stroke)
    .attr("stroke-width", 1);

  // Clip path per tile so flag doesn't spill out
  nodes.each(function(d){
    const id = `clip-${side}-${d.data.iso3}`;
    defs.append("clipPath")
      .attr("id", id)
      .append("rect")
      .attr("rx", 6)
      .attr("ry", 6)
      .attr("x", d.x0)
      .attr("y", d.y0)
      .attr("width", Math.max(0, d.x1 - d.x0))
      .attr("height", Math.max(0, d.y1 - d.y0));
    d.__clipId = id;
  });

  // Flag image (only if enough area + iso2 available)
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
    .attr("clip-path", d => `url(#${d.__clipId})`)
    .attr("display", d => {
      const w = (d.x1 - d.x0), h = (d.y1 - d.y0);
      return (d.data.iso2 && w >= 32 && h >= 22) ? null : "none";
    });

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
      setSelection(row);
      showTooltip(ev, row, side);
    })
    .on("mouseleave", () => {
      hideTooltip();
      applyHighlight(state.highlightKey);
      if (!state.highlightKey) setSelection(null);
    })
    .on("click", (ev, d) => {
      state.highlightKey = d.data.row.iso3;
      applyHighlight(state.highlightKey);
    });
}

function renderAll(){
  const rows = state.rows;
  buildTreemap(els.svgUSA, "usa", rows);
  buildTreemap(els.svgCHN, "chn", rows);

  const totals = rows.map(r => r.total).filter(x=>x>0);
  const extent = totals.length ? d3.extent(totals) : [0,0];
  els.dataInfo.textContent = `rows: ${rows.length} | generated_at: ${state.generatedAt || "-"} | total extent: ${extent[0] ?? 0} .. ${extent[1] ?? 0} | flags: CDN | topN: ${state.topN}`;

  applyHighlight(state.highlightKey);
}

function fit(){
  renderAll();
}

async function main(){
  const res = await fetch(DATA_URL, { cache: "no-store" });
  const data = await res.json();

  state.generatedAt = data.generated_at || data.generatedAt || "-";
  state.rows = normalizeRows(data.rows || []);
  console.info('[TradeGravity] loaded rows=', state.rows.length, 'generated_at=', state.generatedAt);

  els.metric.addEventListener("change", () => {
    state.metric = els.metric.value;
    renderAll();
  });
  els.labelMode.addEventListener("change", () => {
    state.labelMode = els.labelMode.value;
    renderAll();
  });


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

  els.search.addEventListener("input", () => {
    const q = (els.search.value || "").trim().toLowerCase();
    if (!q){
      state.highlightKey = null;
      applyHighlight(null);
      setSelection(null);
      return;
    }
    const hit = state.rows.find(r =>
      (r.iso3 || "").toLowerCase() === q ||
      (r.name || "").toLowerCase().includes(q)
    );
    if (hit){
      state.highlightKey = hit.iso3;
      applyHighlight(hit.iso3);
      setSelection(hit);
    } else {
      state.highlightKey = null;
      applyHighlight(null);
      setSelection(null);
    }
  });

  els.fitBtn.addEventListener("click", fit);
  window.addEventListener("resize", () => {
    clearTimeout(window.__tmResize);
    window.__tmResize = setTimeout(() => renderAll(), 100);
  });

  renderAll();
}

main().catch(err => {
  console.error(err);
  els.dataInfo.textContent = "Failed to load data: " + String(err);
});
