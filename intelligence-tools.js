(function initTradeGravityIntelligenceTools(root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  root.TradeGravityIntelligenceTools = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function buildIntelligenceTools() {
  "use strict";

  function finite(value, fallback = 0) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : fallback;
  }

  function metricValue(row, side, metric) {
    return Math.max(0, finite(row?.[side]?.[metric]));
  }

  function growthValue(row, side, metric) {
    const value = Number(row?.[side]?.growth?.[metric]);
    return Number.isFinite(value) ? value : null;
  }

  function previousValue(current, growth) {
    if (!(current >= 0) || growth == null || growth <= -1) return null;
    return current / (1 + growth);
  }

  function exposureMetrics(usaValue, chinaValue, previousUSAValue = null, previousChinaValue = null) {
    const total = Math.max(0, finite(usaValue)) + Math.max(0, finite(chinaValue));
    const usaShare = total > 0 ? Math.max(0, finite(usaValue)) / total : 0;
    const chinaShare = total > 0 ? Math.max(0, finite(chinaValue)) / total : 0;
    const exposureBalance = total > 0 ? usaShare - chinaShare : 0;
    const dualDependency = total > 0 ? 2 * Math.min(usaShare, chinaShare) : 0;
    const previousTotal = previousUSAValue != null && previousChinaValue != null
      ? Math.max(0, finite(previousUSAValue)) + Math.max(0, finite(previousChinaValue))
      : 0;
    const previousBalance = previousTotal > 0
      ? (Math.max(0, finite(previousUSAValue)) - Math.max(0, finite(previousChinaValue))) / previousTotal
      : null;
    const directionShift = previousBalance == null ? null : exposureBalance - previousBalance;
    const position = total <= 0 ? "unavailable"
      : exposureBalance >= 0.1 ? "US-leaning"
        : exposureBalance <= -0.1 ? "China-leaning" : "balanced";
    const direction = directionShift == null ? "unavailable"
      : directionShift >= 0.03 ? "toward USA"
        : directionShift <= -0.03 ? "toward China" : "stable";
    return {
      total,
      usaShare,
      chinaShare,
      exposureBalance,
      dualDependency,
      previousBalance,
      directionShift,
      position,
      direction,
    };
  }

  function buildIntelligenceProfile(row, metric = "trade") {
    const usaValue = metricValue(row, "usa", metric);
    const chinaValue = metricValue(row, "chn", metric);
    const usaGrowth = growthValue(row, "usa", metric);
    const chinaGrowth = growthValue(row, "chn", metric);
    const exposure = exposureMetrics(
      usaValue,
      chinaValue,
      previousValue(usaValue, usaGrowth),
      previousValue(chinaValue, chinaGrowth),
    );
    const { total, usaShare, chinaShare } = exposure;
    const concentration = total > 0 ? (usaShare * usaShare) + (chinaShare * chinaShare) : 0;
    const exports = metricValue(row, "usa", "export") + metricValue(row, "chn", "export");
    const imports = metricValue(row, "usa", "import") + metricValue(row, "chn", "import");
    const signals = [];

    if (!row?.same_period) signals.push({ level: "warning", label: "Mixed observation periods" });
    if (total <= 0) signals.push({ level: "neutral", label: "No anchor-partner value" });
    if (total > 0 && Math.max(usaShare, chinaShare) >= 0.7) {
      signals.push({
        level: "attention",
        label: `${chinaShare > usaShare ? "China" : "USA"} exceeds 70% of observed anchor trade`,
      });
    }
    if (usaGrowth != null && chinaGrowth != null && Math.abs(chinaGrowth - usaGrowth) >= 0.2) {
      signals.push({ level: "attention", label: "Partner growth paths diverge by at least 20pp" });
    }
    if (exposure.directionShift != null && Math.abs(exposure.directionShift) >= 0.1) {
      signals.push({ level: "attention", label: `Two-anchor position moved ${exposure.direction}` });
    }
    if (signals.length === 0) signals.push({ level: "neutral", label: "No threshold signal in the current view" });

    return {
      iso3: String(row?.iso3 || ""),
      name: String(row?.name || row?.iso3 || "Unknown"),
      metric,
      total,
      usaValue,
      chinaValue,
      usaShare,
      chinaShare,
      exposureBalance: exposure.exposureBalance,
      dualDependency: exposure.dualDependency,
      previousBalance: exposure.previousBalance,
      directionShift: exposure.directionShift,
      position: exposure.position,
      direction: exposure.direction,
      concentration,
      netBalance: exports - imports,
      usaGrowth,
      chinaGrowth,
      growthDivergence: usaGrowth != null && chinaGrowth != null ? usaGrowth - chinaGrowth : null,
      signals,
      scope: "USA and China partner observations only",
    };
  }

  function rankExposureRows(rows, metric = "trade") {
    return (Array.isArray(rows) ? rows : [])
      .map(row => buildIntelligenceProfile(row, metric))
      .sort((a, b) => b.total - a.total || b.chinaShare - a.chinaShare || a.iso3.localeCompare(b.iso3));
  }

  function buildAnchorNetwork(rows, metric = "trade", limit = 25) {
    const ranked = rankExposureRows(rows, metric).filter(item => item.total > 0).slice(0, Math.max(1, limit));
    const nodes = [
      { id: "USA", kind: "anchor", label: "United States" },
      { id: "CHN", kind: "anchor", label: "China" },
      ...ranked.map(item => ({ id: item.iso3, kind: "reporter", label: item.name, total: item.total })),
    ];
    const links = [];
    for (const item of ranked) {
      if (item.usaValue > 0) links.push({ source: item.iso3, target: "USA", value: item.usaValue });
      if (item.chinaValue > 0) links.push({ source: item.iso3, target: "CHN", value: item.chinaValue });
    }
    return { nodes, links, scope: "Two-anchor observed network; not a shipment route" };
  }

  function buildPartnerNetwork(rows, reporterISO3, metric = "trade", limit = 30) {
	const reporter = String(reporterISO3 || "").trim().toUpperCase();
	const valueKey = `${metric}_usd`;
	const ranked = (Array.isArray(rows) ? rows : [])
	  .map(row => ({
		id: String(row?.partner_iso3 || "").trim().toUpperCase(),
		value: Math.max(0, finite(row?.[valueKey])),
		exportUSD: Math.max(0, finite(row?.export_usd)),
		importUSD: Math.max(0, finite(row?.import_usd)),
	  }))
	  .filter(item => /^[A-Z]{3}$/.test(item.id) && item.id !== reporter && item.id !== "WLD" && item.value > 0)
	  .sort((a, b) => b.value - a.value || a.id.localeCompare(b.id))
	  .slice(0, Math.max(1, limit));
	const nodes = [
	  { id: reporter, kind: "reporter", label: reporter },
	  ...ranked.map(item => ({ id: item.id, kind: "partner", label: item.id, total: item.value, exportUSD: item.exportUSD, importUSD: item.importUSD })),
	];
	const links = ranked.map(item => ({ source: reporter, target: item.id, value: item.value }));
	return { nodes, links, scope: "Reported bilateral TOTAL values; not a shipment route" };
  }

  function estimateTariffScenario(input = {}) {
    const baselineImport = Math.max(0, finite(input.baselineImport));
    const existingTariffPct = Math.max(0, finite(input.existingTariffPct));
    const tariffChangePct = finite(input.tariffChangePct);
    const elasticity = Math.min(-0.05, Math.max(-10, finite(input.elasticity, -1.5)));
    const passThrough = Math.min(1, Math.max(0, finite(input.passThrough, 1)));
    const relativePriceShock = ((tariffChangePct / 100) / (1 + existingTariffPct / 100)) * passThrough;
    const unconstrainedResponse = elasticity * relativePriceShock;
    const responseRatio = Math.max(-0.95, Math.min(5, unconstrainedResponse));
    const projectedImport = baselineImport * (1 + responseRatio);
    const baselineRevenue = baselineImport * (existingTariffPct / 100);
    const projectedTariffRate = Math.max(0, existingTariffPct + tariffChangePct);
    const projectedRevenue = projectedImport * (projectedTariffRate / 100);
    return {
      baselineImport,
      existingTariffPct,
      tariffChangePct,
      projectedTariffRate,
      elasticity,
      passThrough,
      relativePriceShock,
      responseRatio,
      projectedImport,
      importChange: projectedImport - baselineImport,
      baselineRevenue,
      projectedRevenue,
      revenueChange: projectedRevenue - baselineRevenue,
      method: "constant-elasticity illustration",
      warning: "Illustrative sensitivity result, not a SMART, GDP, welfare, or causal forecast.",
    };
  }

  function selectPreferredTariffs(rows) {
    const selected = new Map();
    for (const row of Array.isArray(rows) ? rows : []) {
      const code = String(row?.code || "");
      const rate = Number(row?.rate_percent);
      if (!/^\d{6}$/.test(code) || !Number.isFinite(rate) || rate < 0) continue;
      const score = (row.exporter_iso3 === "WLD" ? 4 : 0)
        + (row.data_type === "ave_estimated" ? 2 : 0)
        + (row.rate_type === "mfn_applied" ? 1 : 0);
      const current = selected.get(code);
      if (!current || score > current.score) selected.set(code, { row, score });
    }
    return new Map(Array.from(selected, ([code, value]) => [code, value.row]));
  }

  return {
    buildAnchorNetwork,
	buildPartnerNetwork,
    buildIntelligenceProfile,
    exposureMetrics,
    estimateTariffScenario,
    rankExposureRows,
    selectPreferredTariffs,
  };
});
