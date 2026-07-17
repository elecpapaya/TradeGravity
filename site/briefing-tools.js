(function attachBriefingTools(global) {
  "use strict";

  const READY_SIGNAL_KINDS = ["reporter_total_change", "anchor_share_shift", "product_total_change"];
  const READY_SLIDE_ROLES = ["cover", "scale", "anchor_balance", "product", "method", "cta"];

  function normalizeBriefing(candidate) {
    if (!candidate || candidate.schema_version !== "1.0") return null;
    if (!['ready', 'unavailable'].includes(candidate.status)) return null;
    if (typeof candidate.edition_id !== "string" || typeof candidate.generated_at !== "string") return null;
    if (candidate.review_required !== true || candidate.email?.send_policy !== "manual_review_required") return null;
    if (candidate.social_carousel?.review_policy !== "manual_review_required" || candidate.social_carousel?.aspect_ratio !== "4:5") return null;
    const signals = Array.isArray(candidate.signals) ? candidate.signals : [];
    const slides = Array.isArray(candidate.social_carousel?.slides) ? candidate.social_carousel.slides : [];
    if (candidate.status === "unavailable") {
      return signals.length === 0 && slides.length === 0 ? candidate : null;
    }
    if (signals.length !== READY_SIGNAL_KINDS.length || slides.length !== READY_SLIDE_ROLES.length) return null;
    if (!READY_SIGNAL_KINDS.every((kind, index) => signals[index]?.kind === kind)) return null;
    if (!READY_SLIDE_ROLES.every((role, index) => slides[index]?.role === role && slides[index]?.order === index + 1)) return null;
    if (!signals.every(signal => typeof signal.id === "string" && typeof signal.title === "string" && typeof signal.summary === "string" && Array.isArray(signal.evidence) && signal.evidence.length >= 2)) return null;
    if (!slides.every(slide => typeof slide.headline === "string" && typeof slide.body === "string" && Array.isArray(slide.evidence) && slide.evidence.length > 0)) return null;
    return candidate;
  }

  function materializeEmailMarkdown(briefing, baseURL) {
    const normalized = normalizeBriefing(briefing);
    if (!normalized || typeof normalized.email?.markdown !== "string") return "";
    const base = String(baseURL || "").trim().replace(/\/+$/, "");
    if (!/^https?:\/\//i.test(base)) return normalized.email.markdown;
    return normalized.email.markdown.replaceAll("{{BASE_URL}}", base);
  }

  function buildCarouselBundle(briefing, evidenceBaseURL) {
    const normalized = normalizeBriefing(briefing);
    if (!normalized || normalized.status !== "ready") return null;
    return {
      schema_version: "1.0",
      generated_at: normalized.generated_at,
      edition_id: normalized.edition_id,
      review_required: true,
      evidence_base_url: String(evidenceBaseURL || ""),
      social_carousel: normalized.social_carousel,
      caveats: Array.isArray(normalized.caveats) ? normalized.caveats : [],
    };
  }

  function briefingFilename(briefing, suffix, extension) {
    const edition = typeof briefing?.edition_id === "string" ? briefing.edition_id : "tradegravity-briefing";
    const safeEdition = edition.toLowerCase().replace(/[^a-z0-9-]+/g, "-").replace(/^-+|-+$/g, "") || "tradegravity-briefing";
    const safeSuffix = String(suffix || "draft").toLowerCase().replace(/[^a-z0-9-]+/g, "-").replace(/^-+|-+$/g, "") || "draft";
    const safeExtension = String(extension || "txt").toLowerCase().replace(/[^a-z0-9]+/g, "") || "txt";
    return `${safeEdition}-${safeSuffix}.${safeExtension}`;
  }

  global.TradeGravityBriefingTools = Object.freeze({
    normalizeBriefing,
    materializeEmailMarkdown,
    buildCarouselBundle,
    briefingFilename,
  });
})(globalThis);
