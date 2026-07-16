const test = require("node:test");
const assert = require("node:assert/strict");
const {
  buildSummaryReport,
  deriveDataHealth,
  metricDefinition,
  tabLimitation,
} = require("./experience-tools.js");

test("metric and tab guidance states definitions and decision limits", () => {
  assert.match(metricDefinition("trade", "raw"), /Exports plus imports/);
  assert.match(metricDefinition("import", "gdp_share"), /divided by.*GDP/i);
  assert.match(tabLimitation("intelligence"), /not shipment routes/i);
  assert.match(tabLimitation("lab"), /not GDP/i);
});

test("data health distinguishes current, partial, and failed publications", () => {
  const current = deriveDataHealth({
    coreReady: true,
    metadata: { generated_at: "2026-07-15T00:00:00Z", provider: "wits", available_partner_blocks: 6, expected_partner_blocks: 6 },
    quality: { collection_runs: [{ provider: "wits", mode: "totals", status: "success", failure_count: 0 }] },
    now: "2026-07-16T00:00:00Z",
  });
  assert.equal(current.level, "current");

  const partial = deriveDataHealth({
    coreReady: true,
    metadata: { generated_at: "2026-07-15T00:00:00Z", missing_partner_blocks: 2, available_partner_blocks: 4, expected_partner_blocks: 6 },
    resources: [{ label: "quality report", ready: false }],
    now: "2026-07-16T00:00:00Z",
  });
  assert.equal(partial.level, "partial");
  assert.match(partial.details.join(" "), /2 partner blocks/);
  assert.match(partial.details.join(" "), /quality report/);

  const failed = deriveDataHealth({ coreReady: false });
  assert.equal(failed.level, "failed");
});

test("summary report preserves view provenance, selection, rows, and limitations", () => {
  const report = buildSummaryReport({
    exportedAt: "2026-07-16T00:00:00Z",
    generatedAt: "2026-07-15T00:00:00Z",
    provider: "wits",
    tabLabel: "Intelligence",
    metricLabel: "Total trade",
    periodLabel: "Y 2023",
    comparisonLabel: "Same-period only",
    filterLabel: "ASEAN",
    metricDefinition: metricDefinition("trade", "raw"),
    health: { label: "Data current", summary: "6/6 blocks available.", details: [] },
    selected: {
      name: "Viet Nam", iso3: "VNM", usaValue: "$1", chnValue: "$2", combinedValue: "$3",
      usaPeriod: "2023", chnPeriod: "2023", chinaShare: "66.7%", comparisonQuality: "Same period (2023)",
    },
    topRows: [{ name: "Viet Nam", iso3: "VNM", usaValue: "$1", chnValue: "$2", combinedValue: "$3", periodQuality: "Same period (2023)" }],
    limit: tabLimitation("intelligence"),
    viewURL: "https://example.test/?group=ASEAN&country=VNM&tab=intelligence",
  });
  assert.match(report, /# TradeGravity analysis summary/);
  assert.match(report, /Viet Nam \(VNM\)/);
  assert.match(report, /group=ASEAN/);
  assert.match(report, /not shipment routes/i);
});

test("summary report adds an evidence-grounded semiconductor pulse when active", () => {
	const report = buildSummaryReport({
		exportedAt: "2026-07-16T00:00:00Z",
		generatedAt: "2026-07-16T00:00:00Z",
		provider: "wits",
		tabLabel: "Chip Lens",
		health: { label: "Data current", summary: "ready", details: [] },
		semiconductor: {
			country: "KOR",
			stage: "Fabrication",
			annual: { period: "2025", usaValue: "$10", chinaValue: "$20", position: "China-leaning", direction: "toward USA", balanceShift: "+2.0pp" },
			monthly: { period: "2026-06", previousPeriod: "2026-05", usaValue: "$3", chinaValue: "$4", combinedGrowth: "+5.0%", usaGrowth: "+10.0%", chinaGrowth: "+1.0%", position: "balanced", balanceShift: "+1.0pp" },
			publication: { status: "changed", previousGeneratedAt: "2026-07-09T00:00:00Z", newPeriods: ["2026-06"], newReporters: [], removedReporters: [], addedRows: 2, removedRows: 0, revisedRows: 1, observationDelta: 4, selectedRevisions: [{ reporterISO3: "KOR", period: "2026-05", code: "854231", label: "Processors", previousTotal: "$1", currentTotal: "$2", delta: "+$1" }] },
			evidence: { reference: "https://example.test/reference.json", monthlyIndex: "https://example.test/monthly/index.json", changes: "https://example.test/changes.json" },
		},
		viewURL: "https://example.test/?tab=semiconductors&chipCountry=KOR",
	});
	assert.match(report, /## Semiconductor pulse/);
	assert.match(report, /Latest monthly observation: 2026-06 versus 2026-05/);
	assert.match(report, /### Publication changes/);
	assert.match(report, /New months: 2026-06/);
	assert.match(report, /changes\.json/);
	assert.match(report, /different comparisons/i);
});
