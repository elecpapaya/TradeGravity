const test = require("node:test");
const assert = require("node:assert/strict");

const {
  buildCSVMatrix,
  combinedMetricValue,
  filterAndSortRows,
} = require("./data-tools.js");

const rows = [
  {
    iso3: "JPN",
    name: "Japan",
    usa: { period_type: "Y", period: "2023", export: 2, import: 3, trade: 5, growth_basis: "yoy", growth: { trade: 0.1 } },
    chn: { period_type: "Y", period: "2023", export: 4, import: 6, trade: 10, growth_basis: "yoy", growth: { trade: null } },
    total: 15,
    share_cn: 2 / 3,
  },
  {
    iso3: "KOR",
    name: "South Korea",
    usa: { export: 20, import: 10, trade: 30 },
    chn: { export: 5, import: 5, trade: 10 },
    total: 40,
    share_cn: 0.25,
  },
];

test("filterAndSortRows filters names and sorts by the selected combined metric", () => {
  assert.deepEqual(filterAndSortRows(rows, "", "trade").map(row => row.iso3), ["KOR", "JPN"]);
  assert.deepEqual(filterAndSortRows(rows, "jap", "trade").map(row => row.iso3), ["JPN"]);
  assert.deepEqual(filterAndSortRows(rows, "kor", "export").map(row => row.iso3), ["KOR"]);
  assert.equal(combinedMetricValue(rows[0], "import"), 9);
});

test("buildCSVMatrix flattens partner blocks and repeats provenance metadata", () => {
  const matrix = buildCSVMatrix([rows[0]], {
    schemaVersion: "1.0",
    generatedAt: "2026-07-15T00:00:00Z",
    provider: "wits",
  });
  assert.equal(matrix.length, 2);
  assert.equal(matrix[0].length, matrix[1].length);
  assert.equal(matrix[1][0], "1.0");
  assert.equal(matrix[1][2], "wits");
  assert.equal(matrix[1][3], "JPN");
  assert.equal(matrix[1][10], 5);
  assert.equal(matrix[1][24], "");
  assert.equal(matrix[1][26], 2 / 3);
});
