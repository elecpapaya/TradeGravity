const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "briefing-tools.js"), "utf8");
const context = { globalThis: {} };
vm.createContext(context);
vm.runInContext(source, context);
const tools = context.globalThis.TradeGravityBriefingTools;

function readyBriefing() {
  const signals = ["reporter_total_change", "anchor_share_shift", "product_total_change"].map((kind, index) => ({
    id: `signal-${index}`,
    kind,
    title: `Signal ${index}`,
    summary: "Summary",
    evidence: ["./one.json", "./two.json"],
  }));
  const roles = ["cover", "scale", "anchor_balance", "product", "method", "cta"];
  return {
    schema_version: "1.0",
    generated_at: "2026-07-17T00:00:00Z",
    edition_id: "semiconductor-pulse-2026-05-20260717T000000Z",
    status: "ready",
    review_required: true,
    signals,
    email: { send_policy: "manual_review_required", markdown: "[Evidence]({{BASE_URL}}?tab=semiconductors)" },
    social_carousel: {
      aspect_ratio: "4:5",
      review_policy: "manual_review_required",
      slides: roles.map((role, index) => ({ order: index + 1, role, headline: "Headline", body: "Body", evidence: ["./one.json"] })),
    },
    caveats: ["Caveat"],
  };
}

test("normalizes only review-gated briefing contracts", () => {
  const briefing = readyBriefing();
  assert.equal(tools.normalizeBriefing(briefing), briefing);
  briefing.email.send_policy = "automatic";
  assert.equal(tools.normalizeBriefing(briefing), null);
});

test("materializes the public evidence URL without changing canonical copy", () => {
  const briefing = readyBriefing();
  const draft = tools.materializeEmailMarkdown(briefing, "https://example.test/TradeGravity/");
  assert.equal(draft, "[Evidence](https://example.test/TradeGravity?tab=semiconductors)");
  assert.match(briefing.email.markdown, /\{\{BASE_URL\}\}/);
});

test("builds a cited carousel bundle and a filesystem-safe filename", () => {
  const briefing = readyBriefing();
  const bundle = tools.buildCarouselBundle(briefing, "https://example.test/TradeGravity/data/");
  assert.equal(bundle.review_required, true);
  assert.equal(bundle.social_carousel.slides.length, 6);
  assert.equal(bundle.evidence_base_url, "https://example.test/TradeGravity/data/");
  assert.equal(tools.briefingFilename(briefing, "Instagram draft", "JSON"), "semiconductor-pulse-2026-05-20260717t000000z-instagram-draft.json");
});
