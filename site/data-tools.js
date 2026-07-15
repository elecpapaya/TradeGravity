(function initTradeGravityDataTools(root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) {
    module.exports = api;
  }
  root.TradeGravityDataTools = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function buildDataTools() {
  "use strict";

  const CSV_HEADER = Object.freeze([
    "schema_version", "generated_at", "provider", "reporter_iso3", "reporter_name",
    "usa_period_type", "usa_period", "usa_prev_period", "usa_export_usd", "usa_import_usd", "usa_trade_usd", "usa_growth_basis", "usa_export_growth_yoy", "usa_import_growth_yoy", "usa_trade_growth_yoy",
    "chn_period_type", "chn_period", "chn_prev_period", "chn_export_usd", "chn_import_usd", "chn_trade_usd", "chn_growth_basis", "chn_export_growth_yoy", "chn_import_growth_yoy", "chn_trade_growth_yoy",
    "total_trade_usd", "share_cn",
  ]);

  function numeric(value) {
    if (value == null || value === "") return 0;
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function csvNumber(value) {
    if (value == null || value === "") return "";
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : "";
  }

  function metricValue(row, side, metric) {
    return numeric(row?.[side]?.[metric]);
  }

  function combinedMetricValue(row, metric) {
    return metricValue(row, "usa", metric) + metricValue(row, "chn", metric);
  }

  function filterAndSortRows(rows, query, metric) {
    const normalizedQuery = String(query || "").trim().toLocaleLowerCase("en");
    return (Array.isArray(rows) ? rows : [])
      .filter(row => {
        if (!normalizedQuery) return true;
        const iso3 = String(row?.iso3 || "").toLocaleLowerCase("en");
        const name = String(row?.name || "").toLocaleLowerCase("en");
        return iso3.includes(normalizedQuery) || name.includes(normalizedQuery);
      })
      .slice()
      .sort((a, b) => combinedMetricValue(b, metric) - combinedMetricValue(a, metric) || String(a?.iso3 || "").localeCompare(String(b?.iso3 || "")));
  }

  function buildCSVMatrix(rows, context = {}) {
    const body = (Array.isArray(rows) ? rows : []).map(row => [
      context.schemaVersion || "",
      context.generatedAt || "",
      context.provider || "",
      row?.iso3 || "",
      row?.name || "",
      row?.usa?.period_type || "",
      row?.usa?.period || "",
      row?.usa?.prev_period || "",
      csvNumber(row?.usa?.export),
      csvNumber(row?.usa?.import),
      csvNumber(row?.usa?.trade),
      row?.usa?.growth_basis || "",
      csvNumber(row?.usa?.growth?.export),
      csvNumber(row?.usa?.growth?.import),
      csvNumber(row?.usa?.growth?.trade),
      row?.chn?.period_type || "",
      row?.chn?.period || "",
      row?.chn?.prev_period || "",
      csvNumber(row?.chn?.export),
      csvNumber(row?.chn?.import),
      csvNumber(row?.chn?.trade),
      row?.chn?.growth_basis || "",
      csvNumber(row?.chn?.growth?.export),
      csvNumber(row?.chn?.growth?.import),
      csvNumber(row?.chn?.growth?.trade),
      csvNumber(row?.total),
      csvNumber(row?.share_cn),
    ]);
    return [Array.from(CSV_HEADER), ...body];
  }

  return {
    buildCSVMatrix,
    combinedMetricValue,
    filterAndSortRows,
    metricValue,
  };
});
