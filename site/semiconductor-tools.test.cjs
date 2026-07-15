const test = require("node:test");
const assert = require("node:assert/strict");
const tools = require("./semiconductor-tools.js");

const reference = {
  stages: [
    { id: "equipment", order: 1, label: "Equipment", codes: ["848620"] },
    { id: "packaging", order: 2, label: "Packaging", codes: ["848620", "854290"] },
  ],
  country_roles: [{ iso3: "KOR", name: "Korea", roles: ["packaging"], evidence: "contextual" }],
  policy_events: [
    { date: "2026-01-01", title: "New", stages: ["equipment"], source_id: "a" },
    { date: "2025-01-01", title: "Old", stages: ["packaging"], source_id: "b" },
  ],
  sources: [{ id: "a", url: "https://example.com/a" }, { id: "b", url: "https://example.com/b" }],
  publication: { registered_code_count: 30, observed_reporter_count: 1, observed_period_count: 2 },
};

const files = [
  { reporter_iso3: "KOR", period: "2023", rows: [
    { code: "848620", usa: { available: true, trade: 60 }, chn: { available: true, trade: 40 } },
    { code: "854290", usa: { available: true, trade: 20 }, chn: { available: true, trade: 30 } },
  ] },
  { reporter_iso3: "JPN", period: "2023", rows: [
    { code: "848620", usa: { available: true, trade: 50 }, chn: { available: true, trade: 50 } },
  ] },
  { reporter_iso3: "KOR", period: "2022", rows: [
    { code: "848620", usa: { available: true, trade: 50 }, chn: { available: true, trade: 30 } },
  ] },
];

test("stage summaries retain overlap disclosure and calculate observed distribution", () => {
  const summary = tools.summarizeStages(reference, files, "trade");
  assert.equal(summary.period, "2023");
  const equipment = summary.stages.find(stage => stage.id === "equipment");
  const packaging = summary.stages.find(stage => stage.id === "packaging");
  assert.equal(equipment.total, 200);
  assert.equal(equipment.usaValue, 110);
  assert.equal(equipment.chinaValue, 90);
  assert.ok(Math.abs(equipment.exposureBalance - 0.1) < 1e-12);
  assert.ok(Math.abs(equipment.directionShift - (-0.15)) < 1e-12);
  assert.equal(equipment.direction, "toward China");
  assert.ok(Math.abs(equipment.growthDivergence - (-0.8)) < 1e-12);
  assert.equal(equipment.reporterCount, 2);
  assert.equal(equipment.hhi, 0.5);
  assert.equal(equipment.growth, 1.5);
  assert.equal(packaging.total, 250);
});

test("country profile separates contextual role from observed trade", () => {
  const profile = tools.countryProfile(reference, tools.summarizeStages(reference, files, "trade"), "KOR");
  assert.equal(profile.role.name, "Korea");
  assert.equal(profile.observedStageCount, 2);
  assert.equal(profile.stages.find(stage => stage.id === "packaging").role, true);
  assert.equal(profile.stages.find(stage => stage.id === "equipment").role, false);
  assert.equal(profile.stages.find(stage => stage.id === "packaging").usaValue, 80);
  assert.equal(profile.stages.find(stage => stage.id === "packaging").chinaValue, 70);
  assert.equal(profile.stages.find(stage => stage.id === "packaging").position, "balanced");
});

test("policy filter is stage aware and newest first", () => {
  assert.deepEqual(tools.filterPolicyEvents(reference, "equipment").map(event => event.title), ["New"]);
  assert.deepEqual(tools.filterPolicyEvents(reference).map(event => event.title), ["New", "Old"]);
  assert.equal(tools.sourceIndex(reference).get("a").url, "https://example.com/a");
});

test("shock sensitivity is bounded and transparent", () => {
  const result = tools.estimateStageShock({ baseline: 1000, disruptionPercent: 20, substitutionPercent: 25 });
  assert.equal(result.disruptedAmount, 200);
  assert.equal(result.substitutedAmount, 50);
  assert.equal(result.residualExposure, 150);
  assert.equal(result.retainedAmount, 850);
  assert.match(result.warning, /not a capacity/i);
});

test("coverage gate does not claim readiness below the reporter and period thresholds", () => {
  const limited = tools.coverageGate(reference, {});
  assert.equal(limited.status, "limited");
  assert.equal(limited.ready, false);
  const ready = tools.coverageGate({ ...reference, publication: { registered_code_count: 30, observed_reporter_count: 15, observed_period_count: 5 } }, {});
  assert.equal(ready.status, "research_ready");
});

test("monthly summary exposes recent US-China position movement for a selected stage", () => {
  const file = { reporter_iso3: "KOR", rows: [
    { period: "2024-01", code: "848620", usa: { available: true, trade: 40 }, chn: { available: true, trade: 60 } },
    { period: "2024-02", code: "848620", usa: { available: true, trade: 60 }, chn: { available: true, trade: 40 } },
    { period: "2024-02", code: "999999", usa: { available: true, trade: 999 }, chn: { available: true, trade: 999 } },
  ] };
  const summary = tools.summarizeMonthly(reference, file, "trade", "equipment");
  assert.equal(summary.rows.length, 2);
  assert.ok(Math.abs(summary.windowShift - 0.4) < 1e-12);
  assert.equal(summary.direction, "toward USA");
  assert.equal(summary.latest.position, "US-leaning");
});
