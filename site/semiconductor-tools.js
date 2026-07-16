(function initTradeGravitySemiconductorTools(root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  root.TradeGravitySemiconductorTools = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function buildSemiconductorTools() {
  "use strict";

  const METRICS = new Set(["trade", "export", "import"]);

  function finite(value, fallback = 0) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : fallback;
  }

  function cleanISO3(value) {
    const iso3 = String(value || "").trim().toUpperCase();
    return /^[A-Z]{3}$/.test(iso3) ? iso3 : "";
  }

  function stageMap(reference) {
    return new Map((reference?.stages || []).map(stage => [String(stage.id || ""), stage]));
  }

  function codeStages(reference) {
    const result = new Map();
    for (const stage of reference?.stages || []) {
      for (const code of stage?.codes || []) {
        const normalized = String(code || "");
        if (!/^\d{6}$/.test(normalized)) continue;
        if (!result.has(normalized)) result.set(normalized, []);
        result.get(normalized).push(String(stage.id || ""));
      }
    }
    return result;
  }

  function rowMetric(row, metric) {
    const selected = METRICS.has(metric) ? metric : "trade";
    const usa = row?.usa?.available === false ? 0 : Math.max(0, finite(row?.usa?.[selected]));
    const chn = row?.chn?.available === false ? 0 : Math.max(0, finite(row?.chn?.[selected]));
    return usa + chn;
  }

  function rowAnchorMetrics(row, metric) {
    const selected = METRICS.has(metric) ? metric : "trade";
    const usaValue = row?.usa?.available === false ? 0 : Math.max(0, finite(row?.usa?.[selected]));
    const chinaValue = row?.chn?.available === false ? 0 : Math.max(0, finite(row?.chn?.[selected]));
    return { usaValue, chinaValue, value: usaValue + chinaValue };
  }

  function exposureMetrics(usaValue, chinaValue, previousUSAValue = null, previousChinaValue = null) {
    const helper = globalThis.TradeGravityIntelligenceTools?.exposureMetrics;
    if (typeof helper === "function") return helper(usaValue, chinaValue, previousUSAValue, previousChinaValue);
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
    return {
      total, usaShare, chinaShare, exposureBalance, dualDependency, previousBalance, directionShift,
      position: total <= 0 ? "unavailable" : exposureBalance >= 0.1 ? "US-leaning" : exposureBalance <= -0.1 ? "China-leaning" : "balanced",
      direction: directionShift == null ? "unavailable" : directionShift >= 0.03 ? "toward USA" : directionShift <= -0.03 ? "toward China" : "stable",
    };
  }

  function summarizeStages(reference, files, metric = "trade", selectedPeriod = "latest") {
    const stages = stageMap(reference);
    const mapping = codeStages(reference);
    const cube = new Map();
    const periods = new Set();
    for (const stageID of stages.keys()) cube.set(stageID, new Map());

    for (const file of Array.isArray(files) ? files : []) {
      const reporter = cleanISO3(file?.reporter_iso3);
      const period = String(file?.period || "").trim();
      if (!reporter || !/^\d{4}$/.test(period)) continue;
      for (const row of Array.isArray(file?.rows) ? file.rows : []) {
        const anchors = rowAnchorMetrics(row, metric);
        const value = anchors.value;
        if (!(value > 0)) continue;
        for (const stageID of mapping.get(String(row?.code || "")) || []) {
          if (!cube.has(stageID)) continue;
          periods.add(period);
          if (!cube.get(stageID).has(period)) cube.get(stageID).set(period, new Map());
          const byReporter = cube.get(stageID).get(period);
          const current = byReporter.get(reporter) || { value: 0, usaValue: 0, chinaValue: 0 };
          byReporter.set(reporter, {
            value: current.value + value,
            usaValue: current.usaValue + anchors.usaValue,
            chinaValue: current.chinaValue + anchors.chinaValue,
          });
        }
      }
    }

    const availablePeriods = Array.from(periods).sort().reverse();
    const activePeriod = selectedPeriod !== "latest" && availablePeriods.includes(String(selectedPeriod))
      ? String(selectedPeriod)
      : (availablePeriods[0] || "");
    const output = [];
    for (const stage of Array.from(stages.values()).sort((a, b) => finite(a.order) - finite(b.order))) {
      const byPeriod = cube.get(stage.id) || new Map();
      const stagePeriods = Array.from(byPeriod.keys()).sort().reverse();
      const previousPeriod = stagePeriods.find(period => period < activePeriod) || "";
      const previousByReporter = byPeriod.get(previousPeriod) || new Map();
      const values = Array.from((byPeriod.get(activePeriod) || new Map()).entries())
        .map(([iso3, anchors]) => {
          const previous = previousByReporter.get(iso3);
          return {
            iso3,
            ...anchors,
            ...exposureMetrics(anchors.usaValue, anchors.chinaValue, previous?.usaValue ?? null, previous?.chinaValue ?? null),
          };
        })
        .sort((a, b) => b.value - a.value || a.iso3.localeCompare(b.iso3));
      const total = values.reduce((sum, item) => sum + item.value, 0);
      const shares = values.map(item => total > 0 ? item.value / total : 0);
      const hhi = shares.reduce((sum, share) => sum + share * share, 0);
      const top3Share = shares.slice(0, 3).reduce((sum, share) => sum + share, 0);
      const previousRows = Array.from((byPeriod.get(previousPeriod) || new Map()).values());
      const previousTotal = previousRows.reduce((sum, item) => sum + item.value, 0);
      const usaValue = values.reduce((sum, item) => sum + item.usaValue, 0);
      const chinaValue = values.reduce((sum, item) => sum + item.chinaValue, 0);
      const previousUSAValue = previousRows.reduce((sum, item) => sum + item.usaValue, 0);
      const previousChinaValue = previousRows.reduce((sum, item) => sum + item.chinaValue, 0);
      const exposure = exposureMetrics(
        usaValue,
        chinaValue,
        previousPeriod ? previousUSAValue : null,
        previousPeriod ? previousChinaValue : null,
      );
      const usaGrowth = previousUSAValue > 0 ? (usaValue - previousUSAValue) / previousUSAValue : null;
      const chinaGrowth = previousChinaValue > 0 ? (chinaValue - previousChinaValue) / previousChinaValue : null;
      output.push({
        ...stage,
        period: activePeriod,
        previousPeriod,
        total,
        previousTotal,
        growth: previousTotal > 0 ? (total - previousTotal) / previousTotal : null,
        usaValue,
        chinaValue,
        usaShare: exposure.usaShare,
        chinaShare: exposure.chinaShare,
        exposureBalance: exposure.exposureBalance,
        dualDependency: exposure.dualDependency,
        previousBalance: exposure.previousBalance,
        directionShift: exposure.directionShift,
        position: exposure.position,
        direction: exposure.direction,
        usaGrowth,
        chinaGrowth,
        growthDivergence: usaGrowth != null && chinaGrowth != null ? usaGrowth - chinaGrowth : null,
        reporterCount: values.length,
        hhi,
        top3Share,
        reporters: values.map(item => ({ ...item, share: total > 0 ? item.value / total : 0 })),
        periodCount: stagePeriods.length,
      });
    }
    return { period: activePeriod, periods: availablePeriods, stages: output };
  }

  function countryProfile(reference, summary, iso3) {
    const country = cleanISO3(iso3);
    const role = (reference?.country_roles || []).find(item => cleanISO3(item?.iso3) === country) || null;
    const stages = (summary?.stages || []).map(stage => {
      const observation = (stage.reporters || []).find(item => item.iso3 === country);
      return {
        id: stage.id,
        label: stage.label,
        role: Boolean(role?.roles?.includes(stage.id)),
        value: observation?.value || 0,
        share: observation?.share || 0,
        usaValue: observation?.usaValue || 0,
        chinaValue: observation?.chinaValue || 0,
        usaShare: observation?.usaShare || 0,
        chinaShare: observation?.chinaShare || 0,
        exposureBalance: observation?.exposureBalance || 0,
        dualDependency: observation?.dualDependency || 0,
        position: observation?.position || "unavailable",
        previousBalance: observation?.previousBalance ?? null,
        directionShift: observation?.directionShift ?? null,
        direction: observation?.direction || "unavailable",
        observed: Boolean(observation),
        period: stage.period,
      };
    });
    return { iso3: country, role, stages, observedStageCount: stages.filter(stage => stage.observed).length };
  }

  function filterPolicyEvents(reference, stageID = "all") {
    return (reference?.policy_events || [])
      .filter(event => stageID === "all" || (event?.stages || []).includes(stageID))
      .slice()
      .sort((a, b) => String(b.date || "").localeCompare(String(a.date || "")));
  }

  function sourceIndex(reference) {
    return new Map((reference?.sources || []).map(source => [String(source?.id || ""), source]));
  }

  function estimateStageShock(input = {}) {
    const baseline = Math.max(0, finite(input.baseline));
    const disruptionPercent = Math.min(100, Math.max(0, finite(input.disruptionPercent, 20)));
    const substitutionPercent = Math.min(100, Math.max(0, finite(input.substitutionPercent, 25)));
    const disruptedAmount = baseline * disruptionPercent / 100;
    const substitutedAmount = disruptedAmount * substitutionPercent / 100;
    const residualExposure = disruptedAmount - substitutedAmount;
    return {
      baseline,
      disruptionPercent,
      substitutionPercent,
      disruptedAmount,
      substitutedAmount,
      residualExposure,
      retainedAmount: Math.max(0, baseline - residualExposure),
      warning: "Exposure sensitivity only; not a capacity, price, GDP, welfare or causal forecast.",
    };
  }

  function coverageGate(reference, strategicIndex) {
    const publication = reference?.publication || {};
    const registeredCodeCount = finite(publication.registered_code_count, codeStages(reference).size);
    const reporters = Array.isArray(publication.observed_reporters)
      ? publication.observed_reporters
      : (Array.isArray(strategicIndex?.reporters) ? strategicIndex.reporters : []);
    const periods = Array.isArray(publication.observed_periods)
      ? publication.observed_periods
      : (Array.isArray(strategicIndex?.periods) ? strategicIndex.periods : []);
    const reporterCount = finite(publication.observed_reporter_count, reporters.length);
    const periodCount = finite(publication.observed_period_count, periods.length);
    const targets = {
      reporters: finite(publication.minimum_reporter_target, 15),
      periods: finite(publication.minimum_period_target, 5),
      codes: finite(publication.minimum_code_target, 30),
    };
    const ready = reporterCount >= targets.reporters && periodCount >= targets.periods && registeredCodeCount >= targets.codes;
    return {
      status: ready ? "research_ready" : reporterCount > 0 ? "limited" : "reference_only",
      ready,
      reporterCount,
      periodCount,
      registeredCodeCount,
      targets,
    };
  }

  function summarizeMonthly(reference, file, metric = "trade", stageID = "all") {
    const selected = METRICS.has(metric) ? metric : "trade";
    const mapping = codeStages(reference);
    const periods = new Map();
    for (const row of Array.isArray(file?.rows) ? file.rows : []) {
      const code = String(row?.code || "");
      const mappedStages = mapping.get(code) || [];
      if (mappedStages.length === 0 || (stageID !== "all" && !mappedStages.includes(stageID))) continue;
      const period = String(row?.period || "");
      if (!/^\d{4}-\d{2}$/.test(period)) continue;
      const current = periods.get(period) || { period, usaValue: 0, chinaValue: 0 };
      current.usaValue += row?.usa?.available === false ? 0 : Math.max(0, finite(row?.usa?.[selected]));
      current.chinaValue += row?.chn?.available === false ? 0 : Math.max(0, finite(row?.chn?.[selected]));
      periods.set(period, current);
    }
    const rows = Array.from(periods.values()).sort((a, b) => a.period.localeCompare(b.period));
    for (let index = 0; index < rows.length; index++) {
      const previous = index > 0 ? rows[index - 1] : null;
      Object.assign(rows[index], exposureMetrics(
        rows[index].usaValue,
        rows[index].chinaValue,
        previous?.usaValue ?? null,
        previous?.chinaValue ?? null,
      ));
    }
    const first = rows[0] || null;
    const latest = rows[rows.length - 1] || null;
	const previous = rows.length > 1 ? rows[rows.length - 2] : null;
    const windowShift = first && latest ? latest.exposureBalance - first.exposureBalance : null;
    const direction = windowShift == null ? "unavailable"
      : windowShift >= 0.03 ? "toward USA"
        : windowShift <= -0.03 ? "toward China" : "stable";
    return {
      reporterISO3: cleanISO3(file?.reporter_iso3),
      stageID,
      metric: selected,
      rows,
      first,
	  previous,
      latest,
	  latestGrowth: previous && previous.total > 0 ? (latest.total - previous.total) / previous.total : null,
	  latestUSAGrowth: previous && previous.usaValue > 0 ? (latest.usaValue - previous.usaValue) / previous.usaValue : null,
	  latestChinaGrowth: previous && previous.chinaValue > 0 ? (latest.chinaValue - previous.chinaValue) / previous.chinaValue : null,
	  latestBalanceShift: latest?.directionShift ?? null,
      windowShift,
      direction,
    };
  }

  function publicationPulse(feed, reporterISO3 = "") {
	const status = ["baseline", "unchanged", "changed"].includes(feed?.status) ? feed.status : "unavailable";
	const reporter = cleanISO3(reporterISO3);
	const summary = feed?.summary || {};
	const revisions = (Array.isArray(feed?.top_revisions) ? feed.top_revisions : [])
	  .filter(item => !reporter || cleanISO3(item?.reporter_iso3) === reporter)
	  .map(item => ({
		reporterISO3: cleanISO3(item?.reporter_iso3),
		period: String(item?.period || ""),
		code: String(item?.code || ""),
		label: String(item?.label || ""),
		previousTotal: Math.max(0, finite(item?.previous_total_usd)),
		currentTotal: Math.max(0, finite(item?.current_total_usd)),
		delta: finite(item?.delta_trade_usd),
		magnitude: Math.max(0, finite(item?.magnitude_trade_usd)),
		changeRatio: item?.change_ratio == null ? null : finite(item.change_ratio),
	  }));
	return {
	  status,
	  generatedAt: String(feed?.generated_at || ""),
	  previousGeneratedAt: String(feed?.previous_generated_at || ""),
	  reporterISO3: reporter,
	  summary: {
		currentObservationCount: Math.max(0, finite(summary.current_observation_count)),
		previousObservationCount: Math.max(0, finite(summary.previous_observation_count)),
		observationDelta: finite(summary.observation_delta),
		addedRows: Math.max(0, finite(summary.added_rows)),
		removedRows: Math.max(0, finite(summary.removed_rows)),
		revisedRows: Math.max(0, finite(summary.revised_rows)),
	  },
	  newPeriods: Array.isArray(feed?.new_periods) ? feed.new_periods.map(String) : [],
	  removedPeriods: Array.isArray(feed?.removed_periods) ? feed.removed_periods.map(String) : [],
	  newReporters: Array.isArray(feed?.new_reporters) ? feed.new_reporters.map(cleanISO3).filter(Boolean) : [],
	  removedReporters: Array.isArray(feed?.removed_reporters) ? feed.removed_reporters.map(cleanISO3).filter(Boolean) : [],
	  revisions,
	};
  }

  return {
    codeStages,
    countryProfile,
    coverageGate,
    estimateStageShock,
    filterPolicyEvents,
    sourceIndex,
    summarizeStages,
    summarizeMonthly,
	publicationPulse,
    exposureMetrics,
  };
});
