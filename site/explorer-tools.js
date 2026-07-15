(function initTradeGravityExplorerTools(root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  root.TradeGravityExplorerTools = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function buildExplorerTools() {
  "use strict";

  const METRICS = new Set(["trade", "export", "import"]);
  const COLORS = new Set(["value", "growth"]);
  const MODES = new Set(["comparable", "all"]);
  const NORMALIZATIONS = new Set(["raw", "per_capita", "gdp_share"]);
  const TABS = new Set(["overview", "intelligence", "products", "quality", "lab"]);
  const SCENARIO_PARTNERS = new Set(["usa", "chn"]);

  function clean(value, max = 100) {
    return String(value || "").trim().slice(0, max);
  }

  function defaultViewState() {
    return {
      metric: "trade", color: "value", top: 25, mode: "comparable",
      period: "latest", region: "", income: "", group: "",
      normalization: "raw", country: "", query: "", tab: "overview", sector: "all",
      scenarioPartner: "usa", scenarioProduct: "", tariffBase: 0,
      tariffChange: 10, elasticity: -1.5, passThrough: 1,
    };
  }

  function boundedNumber(params, key, fallback, minimum, maximum) {
    if (!params.has(key)) return fallback;
    const value = Number(params.get(key));
    return Number.isFinite(value) ? Math.max(minimum, Math.min(maximum, value)) : fallback;
  }

  function parseViewState(search) {
    const defaults = defaultViewState();
    const params = new URLSearchParams(String(search || "").replace(/^\?/, ""));
    const metric = clean(params.get("metric"));
    const color = clean(params.get("color"));
    const mode = clean(params.get("mode"));
    const normalization = clean(params.get("normalize"));
    const period = clean(params.get("period"), 16);
    const top = Number.parseInt(params.get("top"), 10);
    const scenarioPartner = clean(params.get("scenario_partner"), 3);
    const scenarioProduct = clean(params.get("scenario_product"), 6);
    return {
      metric: METRICS.has(metric) ? metric : defaults.metric,
      color: COLORS.has(color) ? color : defaults.color,
      top: Number.isFinite(top) ? Math.max(5, Math.min(200, top)) : defaults.top,
      mode: MODES.has(mode) ? mode : defaults.mode,
      period: period === "latest" || /^(Y:\d{4}|Q:\d{4}-Q[1-4]|M:\d{4}-(0[1-9]|1[0-2]))$/.test(period) ? period : defaults.period,
      region: clean(params.get("region")),
      income: clean(params.get("income")),
      group: clean(params.get("group"), 30).toUpperCase(),
      normalization: NORMALIZATIONS.has(normalization) ? normalization : defaults.normalization,
      country: clean(params.get("country"), 3).toUpperCase(),
      query: clean(params.get("q")),
      tab: TABS.has(clean(params.get("tab"))) ? clean(params.get("tab")) : defaults.tab,
      sector: /^(all|[a-z0-9_-]{1,40})$/.test(clean(params.get("sector"))) ? clean(params.get("sector")) : defaults.sector,
      scenarioPartner: SCENARIO_PARTNERS.has(scenarioPartner) ? scenarioPartner : defaults.scenarioPartner,
      scenarioProduct: /^\d{6}$/.test(scenarioProduct) ? scenarioProduct : defaults.scenarioProduct,
      tariffBase: boundedNumber(params, "tariff_base", defaults.tariffBase, 0, 300),
      tariffChange: boundedNumber(params, "tariff_change", defaults.tariffChange, -100, 300),
      elasticity: boundedNumber(params, "elasticity", defaults.elasticity, -10, -0.05),
      passThrough: boundedNumber(params, "pass_through", defaults.passThrough, 0, 1),
    };
  }

  function serializeViewState(view) {
    const defaults = defaultViewState();
    const params = new URLSearchParams();
    const values = { ...defaults, ...(view || {}) };
    if (values.metric !== defaults.metric) params.set("metric", values.metric);
    if (values.color !== defaults.color) params.set("color", values.color);
    if (values.top !== defaults.top) params.set("top", String(values.top));
    if (values.mode !== defaults.mode) params.set("mode", values.mode);
    if (values.period !== defaults.period) params.set("period", values.period);
    if (values.region) params.set("region", clean(values.region));
    if (values.income) params.set("income", clean(values.income));
    if (values.group) params.set("group", clean(values.group, 30).toUpperCase());
    if (values.normalization !== defaults.normalization) params.set("normalize", values.normalization);
    if (values.country) params.set("country", clean(values.country, 3).toUpperCase());
    if (values.query) params.set("q", clean(values.query));
    if (values.tab !== defaults.tab && TABS.has(values.tab)) params.set("tab", values.tab);
    if (values.sector !== defaults.sector && /^(all|[a-z0-9_-]{1,40})$/.test(values.sector)) params.set("sector", values.sector);
    if (values.scenarioPartner !== defaults.scenarioPartner && SCENARIO_PARTNERS.has(values.scenarioPartner)) params.set("scenario_partner", values.scenarioPartner);
    if (values.scenarioProduct && /^\d{6}$/.test(values.scenarioProduct)) params.set("scenario_product", values.scenarioProduct);
    if (Number(values.tariffBase) !== defaults.tariffBase) params.set("tariff_base", String(Number(values.tariffBase)));
    if (Number(values.tariffChange) !== defaults.tariffChange) params.set("tariff_change", String(Number(values.tariffChange)));
    if (Number(values.elasticity) !== defaults.elasticity) params.set("elasticity", String(Number(values.elasticity)));
    if (Number(values.passThrough) !== defaults.passThrough) params.set("pass_through", String(Number(values.passThrough)));
    return params.toString();
  }

  function finite(value, fallback = 0) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function pointBlock(point, side) {
    const value = point?.[side] || {};
    return {
      period_type: clean(point?.period_type, 1),
      period: clean(point?.period, 16),
      export: finite(value.export), import: finite(value.import), trade: finite(value.trade),
      growth: null, growth_basis: "",
    };
  }

  function growth(current, previous, metric) {
    const now = finite(current?.[metric]);
    const before = finite(previous?.[metric]);
    if (!(before > 0)) return null;
    return (now - before) / before;
  }

  function deriveRowsForPeriod(latestRows, seriesRows, periodKey) {
    if (!periodKey || periodKey === "latest") return Array.isArray(latestRows) ? latestRows.slice() : [];
    const [periodType, ...periodParts] = periodKey.split(":");
    const period = periodParts.join(":");
    const metadata = new Map((latestRows || []).map(row => [row.iso3, row]));
    const output = [];
    for (const reporter of seriesRows || []) {
      const points = Array.isArray(reporter?.points) ? reporter.points : [];
      const index = points.findIndex(point => point.period_type === periodType && point.period === period);
      if (index < 0) continue;
      const point = points[index];
      const previous = index > 0 ? points[index - 1] : null;
      const base = metadata.get(reporter.iso3) || { iso3: reporter.iso3 };
      const usa = pointBlock(point, "usa");
      const chn = pointBlock(point, "chn");
      if (previous && previous.period_type === periodType) {
        usa.prev_period = previous.period;
        chn.prev_period = previous.period;
        usa.growth = {};
        chn.growth = {};
        for (const metric of METRICS) {
          usa.growth[metric] = growth(point.usa, previous.usa, metric);
          chn.growth[metric] = growth(point.chn, previous.chn, metric);
        }
        usa.growth_basis = "previous period";
        chn.growth_basis = "previous period";
      }
      const total = usa.trade + chn.trade;
      output.push({
        ...base, usa, chn, total, share_cn: total > 0 ? chn.trade / total : 0,
        same_period: Boolean(point.comparable), comparison_period: point.comparable ? period : "",
      });
    }
    return output;
  }

  function filterExplorerRows(rows, filters = {}) {
    const query = clean(filters.query).toLocaleLowerCase("en");
    return (Array.isArray(rows) ? rows : []).filter(row => {
      if (filters.mode !== "all" && !row?.same_period) return false;
      if (filters.region && row?.region !== filters.region) return false;
      if (filters.income && row?.income_group !== filters.income) return false;
      if (filters.group && !(row?.groups || []).includes(filters.group)) return false;
      if (query) {
        const haystack = `${row?.iso3 || ""} ${row?.name || ""}`.toLocaleLowerCase("en");
        if (!haystack.includes(query)) return false;
      }
      return true;
    });
  }

  function normalizedMetricValue(row, side, metric, normalization) {
    const raw = finite(row?.[side]?.[metric]);
    if (normalization === "per_capita") {
      const population = finite(row?.population?.value);
      return population > 0 ? raw / population : null;
    }
    if (normalization === "gdp_share") {
      const gdp = finite(row?.gdp?.value);
      return gdp > 0 ? raw / gdp : null;
    }
    return raw;
  }

  function availablePeriods(seriesRows) {
    const counts = new Map();
    for (const reporter of seriesRows || []) {
      for (const point of reporter?.points || []) {
        const key = `${point.period_type}:${point.period}`;
        const current = counts.get(key) || { key, reporters: 0, comparable: 0 };
        current.reporters += 1;
        if (point.comparable) current.comparable += 1;
        counts.set(key, current);
      }
    }
    return Array.from(counts.values()).sort((a, b) => b.key.localeCompare(a.key));
  }

  function buildFilteredJSON(rows, context = {}) {
    return {
      schema_version: context.schemaVersion || "",
      generated_at: context.generatedAt || "",
      provider: context.provider || "",
      filters: context.filters || {},
      rows: Array.isArray(rows) ? rows : [],
    };
  }

  return {
    availablePeriods,
    buildFilteredJSON,
    defaultViewState,
    deriveRowsForPeriod,
    filterExplorerRows,
    normalizedMetricValue,
    parseViewState,
    serializeViewState,
  };
});
