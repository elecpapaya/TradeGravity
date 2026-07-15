const test = require("node:test");
const assert = require("node:assert/strict");

const {
  escapeHTML,
  normalizeISO2,
  normalizeISO3,
  safeHTTPSURL,
} = require("./security.js");

test("escapeHTML neutralizes markup and attribute delimiters", () => {
  assert.equal(
    escapeHTML(`<img src=x onerror="alert('x')"> & text`),
    "&lt;img src=x onerror=&quot;alert(&#39;x&#39;)&quot;&gt; &amp; text",
  );
});

test("safeHTTPSURL accepts HTTPS and rejects executable or insecure schemes", () => {
  assert.equal(safeHTTPSURL("https://example.com/story?q=1"), "https://example.com/story?q=1");
  assert.equal(safeHTTPSURL("javascript:alert(1)"), "");
  assert.equal(safeHTTPSURL("data:text/html,hello"), "");
  assert.equal(safeHTTPSURL("http://example.com/story"), "");
  assert.equal(safeHTTPSURL("not a URL"), "");
});

test("ISO normalizers accept only exact alphabetic country codes", () => {
  assert.equal(normalizeISO2(" kr "), "KR");
  assert.equal(normalizeISO2("K1"), "");
  assert.equal(normalizeISO3(" kor "), "KOR");
  assert.equal(normalizeISO3("KOR<script>"), "");
});
