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

  function escapeCSVCell(value) {
    let text = String(value ?? "");
    if (typeof value !== "number" && /^[=+\-@\t\r]/.test(text)) {
      text = "'" + text;
    }
    return `"${text.replace(/"/g, '""')}"`;
  }

  function encodeCSV(rows) {
    if (!Array.isArray(rows)) return "";
    return rows
      .map(row => (Array.isArray(row) ? row : [row]).map(escapeCSVCell).join(","))
      .join("\r\n");
  }

  return {
    encodeCSV,
    escapeHTML,
    normalizeISO2,
    normalizeISO3,
    safeHTTPSURL,
  };
});
