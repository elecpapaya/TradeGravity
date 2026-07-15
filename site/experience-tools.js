(function initTradeGravityExperienceTools(root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  root.TradeGravityExperienceTools = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function buildExperienceTools() {
  "use strict";

  const METRIC_DEFINITIONS = Object.freeze({
    trade: "Exports plus imports reported by each country for the selected partner.",
    export: "Goods exports reported by each country to the selected partner.",
    import: "Goods imports reported by each country from the selected partner.",
  });

  const NORMALIZATION_NOTES = Object.freeze({
    raw: "Values are nominal current US dollars; they are not inflation-adjusted.",
    per_capita: "Trade is divided by the latest published population context for that reporter.",
    gdp_share: "Trade is divided by the latest published current-US-dollar GDP context for that reporter.",
  });

  const TAB_LIMITS = Object.freeze({
    overview: "Bilateral observations cover the United States and China anchors only; they are not a country's complete world trade.",
    intelligence: "Exposure signals are descriptive screening indicators, not shipment routes, causal estimates, fraud findings, or investment advice.",
    products: "HS2 and curated HS6 partitions may use a different provider or observation period from headline totals; compare their labels before interpreting them together.",
    quality: "Provider differences can reflect reporter, classification, valuation, revision, and period choices; a delta alone does not identify an error.",
    lab: "Scenario results are transparent sensitivity arithmetic, not GDP, welfare, trade-diversion, or causal forecasts.",
  });

  function clean(value, max = 500) {
    return String(value == null ? "" : value).trim().slice(0, max);
  }

  function metricDefinition(metric, normalization = "raw") {
    const definition = METRIC_DEFINITIONS[metric] || METRIC_DEFINITIONS.trade;
    const note = NORMALIZATION_NOTES[normalization] || NORMALIZATION_NOTES.raw;
    return `${definition} ${note}`;
  }

  function tabLimitation(tab) {
    return TAB_LIMITS[tab] || TAB_LIMITS.overview;
  }

  function ageInHours(value, now) {
    const generated = new Date(value).getTime();
    const current = new Date(now || Date.now()).getTime();
    if (!Number.isFinite(generated) || !Number.isFinite(current)) return null;
    return Math.max(0, (current - generated) / 3600000);
  }

  function deriveDataHealth(input = {}) {
    const metadata = input.metadata || {};
    const quality = input.quality || {};
    const resources = Array.isArray(input.resources) ? input.resources : [];
    const details = [];
    let level = "current";

    if (input.coreReady === false) {
      return {
        level: "failed",
        label: "Data unavailable",
        summary: "The headline trade dataset could not be loaded.",
        details: ["Retry the publication or open Data & Quality for recovery information."],
      };
    }

    const missing = Number(metadata.missing_partner_blocks || quality?.summary?.missing_partner_blocks || 0);
    const stale = Number(metadata.stale_partner_blocks || quality?.summary?.stale_partner_blocks || 0);
    if (missing > 0) details.push(`${missing} partner block${missing === 1 ? " is" : "s are"} missing.`);
    if (stale > 0) details.push(`${stale} partner block${stale === 1 ? " is" : "s are"} stale.`);

    const primary = clean(metadata.provider || quality.primary_provider, 30).toLowerCase();
    const runs = Array.isArray(quality.collection_runs) ? quality.collection_runs : [];
    const primaryRun = runs.find(run => clean(run?.provider, 30).toLowerCase() === primary && run?.mode === "totals")
      || runs.find(run => clean(run?.provider, 30).toLowerCase() === primary)
      || null;
    if (primaryRun) {
      const runStatus = clean(primaryRun.status, 20).toLowerCase();
      const failures = Number(primaryRun.failure_count || 0);
      if (runStatus === "failed") {
        level = "failed";
        details.push(`Latest ${primary || "primary"} collection run failed.`);
      } else if (runStatus === "partial" || failures > 0) {
        details.push(`Latest ${primary || "primary"} collection run completed with ${failures} failed request${failures === 1 ? "" : "s"}.`);
      }
    }

    const unavailable = resources.filter(resource => resource && resource.ready === false);
    if (unavailable.length > 0) {
      details.push(`Optional publication resources unavailable: ${unavailable.map(resource => clean(resource.label, 60)).filter(Boolean).join(", ")}.`);
    }

    const age = ageInHours(metadata.generated_at || input.generatedAt, input.now);
    if (age != null && age > 72) {
      details.push(`The publication refresh is ${Math.floor(age / 24)} days old.`);
    }

    if (level !== "failed" && details.length > 0) level = "partial";
    const labels = { current: "Data current", partial: "Partial data", failed: "Data degraded" };
    const available = Number(metadata.available_partner_blocks || 0);
    const expected = Number(metadata.expected_partner_blocks || 0);
    const coverage = expected > 0 ? `${available}/${expected} headline partner blocks available.` : "Headline dataset loaded.";
    const summary = level === "current"
      ? coverage
      : `${coverage} ${details[0] || "Check Data & Quality before interpretation."}`;

    return { level, label: labels[level], summary, details };
  }

  function markdownCell(value) {
    return clean(value, 200).replaceAll("|", "\\|").replaceAll("\n", " ") || "—";
  }

  function buildSummaryReport(model = {}) {
    const selected = model.selected || null;
    const topRows = Array.isArray(model.topRows) ? model.topRows.slice(0, 10) : [];
    const health = model.health || {};
    const lines = [
      "# TradeGravity analysis summary",
      "",
      `Generated: ${clean(model.exportedAt) || "unknown"}`,
      `Published data refresh: ${clean(model.generatedAt) || "unknown"}`,
      `Provider: ${clean(model.provider).toUpperCase() || "UNKNOWN"}`,
      "",
      "## Active view",
      "",
      `- Section: ${clean(model.tabLabel) || "Overview"}`,
      `- Metric: ${clean(model.metricLabel) || "Total trade"}`,
      `- Observation period: ${clean(model.periodLabel) || "Latest by reporter"}`,
      `- Comparison: ${clean(model.comparisonLabel) || "Same-period only"}`,
      `- Filters: ${clean(model.filterLabel) || "All reporters"}`,
      `- Definition: ${clean(model.metricDefinition) || METRIC_DEFINITIONS.trade}`,
      "",
      "## Data status",
      "",
      `- ${clean(health.label) || "Unknown"}: ${clean(health.summary) || "No status summary available."}`,
    ];
    for (const detail of health.details || []) lines.push(`- ${clean(detail)}`);

    lines.push("", "## Selected country", "");
    if (selected) {
      lines.push(
        `- Reporter: ${clean(selected.name)} (${clean(selected.iso3)})`,
        `- USA: ${clean(selected.usaValue)} · observation ${clean(selected.usaPeriod) || "unknown"}`,
        `- China: ${clean(selected.chnValue)} · observation ${clean(selected.chnPeriod) || "unknown"}`,
        `- Combined selected metric: ${clean(selected.combinedValue)}`,
        `- China share within the two-anchor scope: ${clean(selected.chinaShare)}`,
        `- Comparison quality: ${clean(selected.comparisonQuality)}`,
      );
    } else {
      lines.push("No country was selected in this view.");
    }

    lines.push("", "## Leading observations in the filtered view", "");
    if (topRows.length > 0) {
      lines.push("| Reporter | USA | China | Combined | Period quality |", "|---|---:|---:|---:|---|");
      for (const row of topRows) {
        lines.push(`| ${markdownCell(`${row.name} (${row.iso3})`)} | ${markdownCell(row.usaValue)} | ${markdownCell(row.chnValue)} | ${markdownCell(row.combinedValue)} | ${markdownCell(row.periodQuality)} |`);
      }
    } else {
      lines.push("No observations match the active filters.");
    }

    lines.push(
      "",
      "## Interpretation limits",
      "",
      `- ${clean(model.limit) || TAB_LIMITS.overview}`,
      "- Pipeline refresh time and source observation period are different clocks.",
      "- Recent headlines are optional context and must not be treated as same-period evidence for historical trade observations.",
      "",
      "## Reproduce this view",
      "",
      clean(model.viewURL) || "View URL unavailable.",
      "",
      "Generated by TradeGravity. Verify source observations and methodology before making decisions.",
      "",
    );
    return lines.join("\n");
  }

  return {
    buildSummaryReport,
    deriveDataHealth,
    metricDefinition,
    tabLimitation,
  };
});
