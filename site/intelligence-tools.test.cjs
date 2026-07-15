const test = require("node:test");
const assert = require("node:assert/strict");
const {
  buildAnchorNetwork,
	buildPartnerNetwork,
  buildIntelligenceProfile,
  estimateTariffScenario,
  rankExposureRows,
  selectPreferredTariffs,
} = require("./intelligence-tools.js");

const rows = [{
  iso3: "KOR", name: "Korea", same_period: true,
  usa: { export: 80, import: 20, trade: 100, growth: { trade: 0.05 } },
  chn: { export: 60, import: 240, trade: 300, growth: { trade: 0.30 } },
}, {
  iso3: "JPN", name: "Japan", same_period: false,
  usa: { export: 10, import: 10, trade: 20 }, chn: { export: 5, import: 5, trade: 10 },
}];

test("profile derives transparent two-partner exposure signals", () => {
  const profile = buildIntelligenceProfile(rows[0]);
  assert.equal(profile.total, 400);
  assert.equal(profile.chinaShare, 0.75);
  assert.equal(profile.concentration, 0.625);
  assert.equal(profile.netBalance, -120);
  assert.equal(profile.growthDivergence, 0.25);
  assert.match(profile.scope, /USA and China/);
  assert.equal(profile.signals.length, 2);
});

test("ranking and anchor network retain values without claiming routes", () => {
  assert.deepEqual(rankExposureRows(rows).map(row => row.iso3), ["KOR", "JPN"]);
  const network = buildAnchorNetwork(rows, "trade", 1);
  assert.equal(network.nodes.length, 3);
  assert.equal(network.links.length, 2);
  assert.match(network.scope, /not a shipment route/);
});

test("partner network ranks reported bilateral totals without treating World as a partner", () => {
  const network = buildPartnerNetwork([
	{ partner_iso3: "USA", export_usd: 70, import_usd: 30, trade_usd: 100 },
	{ partner_iso3: "CHN", export_usd: 20, import_usd: 60, trade_usd: 80 },
	{ partner_iso3: "WLD", export_usd: 900, import_usd: 100, trade_usd: 1000 },
  ], "KOR", "trade", 1);
  assert.deepEqual(network.nodes.map(node => node.id), ["KOR", "USA"]);
  assert.equal(network.links[0].value, 100);
  assert.match(network.scope, /not a shipment route/);
});

test("tariff scenario exposes its assumptions and response arithmetic", () => {
  const result = estimateTariffScenario({
    baselineImport: 1000, existingTariffPct: 10, tariffChangePct: 10,
    elasticity: -2, passThrough: 0.5,
  });
  assert.ok(Math.abs(result.responseRatio - (-0.0909090909)) < 1e-8);
  assert.ok(result.projectedImport < 1000);
  assert.equal(result.projectedTariffRate, 20);
  assert.match(result.warning, /not a SMART/);
});

test("tariff selection prefers world MFN AVE rows without mixing product codes", () => {
  const selected = selectPreferredTariffs([
    { code: "854231", exporter_iso3: "USA", data_type: "reported", rate_type: "preferential", rate_percent: 1 },
    { code: "854231", exporter_iso3: "WLD", data_type: "reported", rate_type: "mfn_applied", rate_percent: 3 },
    { code: "854231", exporter_iso3: "WLD", data_type: "ave_estimated", rate_type: "mfn_applied", rate_percent: 4 },
    { code: "850760", exporter_iso3: "WLD", data_type: "reported", rate_type: "mfn_applied", rate_percent: 2 },
    { code: "bad", exporter_iso3: "WLD", data_type: "ave_estimated", rate_type: "mfn_applied", rate_percent: 99 },
  ]);
  assert.equal(selected.size, 2);
  assert.equal(selected.get("854231").rate_percent, 4);
  assert.equal(selected.get("850760").rate_percent, 2);
});
