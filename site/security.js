(function initTradeGravitySecurity(root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) {
    module.exports = api;
  }
  root.TradeGravitySecurity = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function buildSecurityHelpers() {
  "use strict";

  const HTML_ESCAPES = {
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  };

  function escapeHTML(value) {
    return String(value ?? "").replace(/[&<>"']/g, character => HTML_ESCAPES[character]);
  }

  function safeHTTPSURL(value) {
    if (typeof value !== "string" || value.trim() === "") return "";
    try {
      const parsed = new URL(value.trim());
      return parsed.protocol === "https:" ? parsed.href : "";
    } catch {
      return "";
    }
  }

  function normalizeISO2(value) {
    const normalized = String(value ?? "").trim().toUpperCase();
    return /^[A-Z]{2}$/.test(normalized) ? normalized : "";
  }

  function normalizeISO3(value) {
    const normalized = String(value ?? "").trim().toUpperCase();
    return /^[A-Z]{3}$/.test(normalized) ? normalized : "";
  }

  return {
    escapeHTML,
    normalizeISO2,
    normalizeISO3,
    safeHTTPSURL,
  };
});
