const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const {
  availablePeriods, deriveRowsForPeriod, filterExplorerRows,
  normalizedMetricValue, parseViewState, serializeViewState,
} = require("./explorer-tools.js");

const latest = [{
  iso3: "KOR", name: "Korea", region: "East Asia & Pacific", income_group: "High income",
  groups: [], population: { value: 50 }, gdp: { value: 1000 }, same_period: true,
  usa: { trade: 100 }, chn: { trade: 200 }, total: 300, share_cn: 2 / 3,
}];
const series = [{ iso3: "KOR", points: [
  { period_type: "Y", period: "2022", comparable: true, usa: { available: true, export: 30, import: 20, trade: 50 }, chn: { available: true, export: 40, import: 60, trade: 100 } },
  { period_type: "Y", period: "2023", comparable: true, usa: { available: true, export: 60, import: 40, trade: 100 }, chn: { available: true, export: 80, import: 120, trade: 200 } },
]}];

test("view state round-trips supported filters and rejects unsafe values", () => {
  const parsed = parseViewState("?metric=export&period=Y%3A2023&mode=all&group=asean&top=40&country=kor&tab=lab&sector=semiconductors&scenario_partner=chn&scenario_product=854231&tariff_base=7.5&tariff_change=25&elasticity=-2&pass_through=.75");
  assert.equal(parsed.metric, "export");
  assert.equal(parsed.period, "Y:2023");
  assert.equal(parsed.group, "ASEAN");
  assert.equal(parsed.country, "KOR");
  assert.equal(parsed.tab, "lab");
  assert.equal(parsed.sector, "semiconductors");
  assert.equal(parsed.scenarioPartner, "chn");
  assert.equal(parsed.scenarioProduct, "854231");
  assert.equal(parsed.tariffBase, 7.5);
  assert.equal(parsed.tariffChange, 25);
  assert.equal(parsed.elasticity, -2);
  assert.equal(parsed.passThrough, 0.75);
  assert.match(serializeViewState(parsed), /period=Y%3A2023/);
  assert.match(serializeViewState(parsed), /scenario_product=854231/);
  assert.equal(parseViewState("?metric=bogus&period=javascript:alert(1)").metric, "trade");
  assert.equal(parseViewState("?metric=bogus&period=javascript:alert(1)").period, "latest");
  assert.equal(parseViewState("?tab=javascript:alert(1)").tab, "overview");
  assert.equal(parseViewState("?sector=../../secret").sector, "all");
  const chipView = parseViewState("?tab=semiconductors&chip_stage=memory_hbm&chip_country=kor");
  assert.equal(chipView.tab, "semiconductors");
  assert.equal(chipView.chipStage, "memory_hbm");
  assert.equal(chipView.chipCountry, "KOR");
  assert.match(serializeViewState(chipView), /chip_stage=memory_hbm/);
  const bounded = parseViewState("?scenario_partner=evil&scenario_product=../../x&tariff_base=999&tariff_change=-999&elasticity=2&pass_through=9");
  assert.equal(bounded.scenarioPartner, "usa");
  assert.equal(bounded.scenarioProduct, "");
  assert.equal(bounded.tariffBase, 300);
  assert.equal(bounded.tariffChange, -100);
  assert.equal(bounded.elasticity, -0.05);
  assert.equal(bounded.passThrough, 1);
});

test("period derivation calculates comparable rows and previous-period growth", () => {
  const rows = deriveRowsForPeriod(latest, series, "Y:2023");
  assert.equal(rows.length, 1);
  assert.equal(rows[0].total, 300);
  assert.equal(rows[0].comparison_period, "2023");
  assert.equal(rows[0].usa.growth.trade, 1);
  assert.deepEqual(availablePeriods(series), [
    { key: "Y:2023", reporters: 1, comparable: 1 },
    { key: "Y:2022", reporters: 1, comparable: 1 },
  ]);
});

test("filters and normalization use published country context", () => {
  assert.equal(filterExplorerRows(latest, { mode: "comparable", region: "East Asia & Pacific" }).length, 1);
  assert.equal(filterExplorerRows(latest, { mode: "comparable", group: "ASEAN" }).length, 0);
  assert.equal(normalizedMetricValue(latest[0], "usa", "trade", "per_capita"), 2);
  assert.equal(normalizedMetricValue(latest[0], "chn", "trade", "gdp_share"), 0.2);
});

test("schema 2 sample supports exact-period filtering and normalization end to end", () => {
  const sampleDir = path.join(__dirname, "..", "examples", "sample-data");
  const latest = JSON.parse(fs.readFileSync(path.join(sampleDir, "latest.json"), "utf8"));
  const series = JSON.parse(fs.readFileSync(path.join(sampleDir, "series.json"), "utf8"));
  assert.equal(latest.schema_version, "2.0");
  const period = availablePeriods(series.rows).find(item => item.key === "Y:2023");
  assert.deepEqual(period, { key: "Y:2023", reporters: 3, comparable: 3 });
  const exactRows = deriveRowsForPeriod(latest.rows, series.rows, "Y:2023");
  const euRows = filterExplorerRows(exactRows, { mode: "comparable", group: "EU" });
  assert.equal(euRows.length, 1);
  assert.equal(euRows[0].iso3, "DEU");
  assert.ok(normalizedMetricValue(euRows[0], "usa", "trade", "per_capita") > 0);
  assert.ok(normalizedMetricValue(euRows[0], "chn", "trade", "gdp_share") > 0);
});
