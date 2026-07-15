const test = require("node:test");
const assert = require("node:assert/strict");

const {
  buildGdeltURL,
  curateNewsArticles,
  parseGdeltDate,
  relevanceScore,
} = require("./news-tools.js");

test("buildGdeltURL limits the request to trade topics, source country, and a recent window", () => {
  const url = new URL(buildGdeltURL("mx", { windowDays: 14, maxRecords: 40 }));
  assert.equal(url.protocol, "https:");
  assert.match(url.searchParams.get("query"), /sourcecountry:MX/);
  assert.match(url.searchParams.get("query"), /supply chain/);
  assert.equal(url.searchParams.get("timespan"), "14d");
  assert.equal(url.searchParams.get("maxrecords"), "40");
  assert.equal(buildGdeltURL("MEX"), "");
});

test("curateNewsArticles rejects noise, stale and unsafe links, and removes duplicates", () => {
  const now = Date.UTC(2026, 6, 16, 12);
  const articles = [
    { title: "Mexico tariff talks reshape auto supply chain", url: "https://example.com/a?utm_source=x", domain: "example.com", seendate: "20260716100000" },
    { title: "Mexico tariff talks reshape auto supply chain", url: "https://mirror.example/a", seendate: "20260716090000" },
    { title: "Exports rise as customs delays ease", url: "https://example.com/b", seendate: "20260715100000" },
    { title: "Football club wins final", url: "https://example.com/sport", seendate: "20260716100000" },
    { title: "Old trade story", url: "https://example.com/old", seendate: "20260601100000" },
    { title: "Imports update", url: "http://example.com/insecure", seendate: "20260716100000" },
  ];
  assert.deepEqual(curateNewsArticles(articles, { now, windowDays: 14, maxItems: 5 }), [
    { title: "Mexico tariff talks reshape auto supply chain", url: "https://example.com/a", domain: "example.com", seen: "2026-07-16" },
    { title: "Exports rise as customs delays ease", url: "https://example.com/b", domain: "example.com", seen: "2026-07-15" },
  ]);
});

test("relevance matching supports selected non-English trade vocabulary and validates dates", () => {
  assert.ok(relevanceScore("한국 수출 공급망 점검") >= 2);
  assert.ok(relevanceScore("México anuncia nuevos aranceles a importaciones") >= 1);
  assert.equal(relevanceScore("Local team wins championship"), 0);
  assert.equal(parseGdeltDate("20260230000000"), null);
});
