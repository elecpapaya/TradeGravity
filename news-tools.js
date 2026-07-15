(function initTradeGravityNewsTools(root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  root.TradeGravityNewsTools = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function buildNewsTools() {
  "use strict";

  const DEFAULT_WINDOW_DAYS = 14;
  const DEFAULT_MAX_ITEMS = 5;
  const QUERY_TERMS = Object.freeze([
    "trade", "tariff", "export", "import", "\"supply chain\"", "logistics",
    "shipping", "customs", "sanctions", "semiconductor",
  ]);
  const LATIN_RELEVANCE = /(?:^|[^\p{L}])(?:trade|trading|tariffs?|exports?|imports?|logistics?|shipping|customs|sanctions?|semiconductors?|commerce|comercio|arancel(?:es)?|exporta(?:tion|tions|ci[oó]n|ciones|[cç][aã]o|[cç][oõ]es)?|importa(?:tion|tions|ci[oó]n|ciones|[cç][aã]o|[cç][oõ]es)?|aduana|douane|handel|zoll|lieferkette|perdagangan|ekspor|impor)(?=$|[^\p{L}])/giu;
  const PHRASES = Object.freeze([
    "supply chain", "supply-chain", "cadena de suministro", "cadeia de suprimentos",
    "chaîne d’approvisionnement", "chaine d'approvisionnement", "rantai pasok",
    "comercio exterior", "comércio exterior", "global trade",
    "무역", "관세", "수출", "수입", "공급망",
    "贸易", "貿易", "关税", "關稅", "出口", "进口", "進口", "供应链", "供應鏈",
    "関税", "輸出", "輸入", "供給網",
    "торгов", "тариф", "экспорт", "импорт", "цепочк поставок",
  ]);

  function normalizeISO2(value) {
    const iso2 = String(value || "").trim().toUpperCase();
    return /^[A-Z]{2}$/.test(iso2) ? iso2 : "";
  }

  function buildGdeltURL(iso2, options = {}) {
    const country = normalizeISO2(iso2);
    if (!country) return "";
    const windowDays = boundedInteger(options.windowDays, 1, 30, DEFAULT_WINDOW_DAYS);
    const maxRecords = boundedInteger(options.maxRecords, 10, 250, 50);
    const query = `sourcecountry:${country} (${QUERY_TERMS.join(" OR ")})`;
    const params = new URLSearchParams({
      query,
      mode: "ArtList",
      maxrecords: String(maxRecords),
      timespan: `${windowDays}d`,
      sort: "DateDesc",
      format: "json",
    });
    return `https://api.gdeltproject.org/api/v2/doc/doc?${params.toString()}`;
  }

  function boundedInteger(value, min, max, fallback) {
    const parsed = Number(value);
    if (!Number.isInteger(parsed)) return fallback;
    return Math.min(max, Math.max(min, parsed));
  }

  function relevanceScore(title) {
    const text = String(title || "").normalize("NFKC").toLocaleLowerCase("en");
    if (!text) return 0;
    const matches = text.match(LATIN_RELEVANCE) || [];
    let score = new Set(matches.map(match => match.trim())).size;
    for (const phrase of PHRASES) {
      if (text.includes(phrase)) score += 1;
    }
    return score;
  }

  function parseGdeltDate(value) {
    const match = String(value || "").match(/^(\d{4})(\d{2})(\d{2})(?:(\d{2})(\d{2})(\d{2}))?/);
    if (!match) return null;
    const timestamp = Date.UTC(
      Number(match[1]), Number(match[2]) - 1, Number(match[3]),
      Number(match[4] || 0), Number(match[5] || 0), Number(match[6] || 0),
    );
    if (!Number.isFinite(timestamp)) return null;
    const date = new Date(timestamp);
    if (
      date.getUTCFullYear() !== Number(match[1]) ||
      date.getUTCMonth() !== Number(match[2]) - 1 ||
      date.getUTCDate() !== Number(match[3])
    ) return null;
    return timestamp;
  }

  function formatDate(timestamp) {
    if (!Number.isFinite(timestamp)) return "";
    return new Date(timestamp).toISOString().slice(0, 10);
  }

  function canonicalArticleURL(value) {
    try {
      const url = new URL(String(value || ""));
      if (url.protocol !== "https:") return null;
      url.hash = "";
      for (const key of Array.from(url.searchParams.keys())) {
        if (/^(?:utm_.+|fbclid|gclid)$/i.test(key)) url.searchParams.delete(key);
      }
      url.hostname = url.hostname.toLowerCase();
      if (url.pathname.length > 1) url.pathname = url.pathname.replace(/\/+$/, "");
      return url;
    } catch {
      return null;
    }
  }

  function normalizedTitleKey(value) {
    return String(value || "")
      .normalize("NFKC")
      .toLocaleLowerCase("en")
      .replace(/[^\p{L}\p{N}]+/gu, " ")
      .trim();
  }

  function curateNewsArticles(articles, options = {}) {
    const windowDays = boundedInteger(options.windowDays, 1, 30, DEFAULT_WINDOW_DAYS);
    const maxItems = boundedInteger(options.maxItems, 1, 20, DEFAULT_MAX_ITEMS);
    const now = Number.isFinite(options.now) ? options.now : Date.now();
    const oldest = now - windowDays * 24 * 60 * 60 * 1000;
    const newest = now + 24 * 60 * 60 * 1000;
    const seenTitles = new Set();
    const seenURLs = new Set();
    const curated = [];

    for (const article of Array.isArray(articles) ? articles : []) {
      const title = String(article?.title || "").trim().replace(/\s+/g, " ").slice(0, 300);
      const titleKey = normalizedTitleKey(title);
      const score = relevanceScore(title);
      const timestamp = parseGdeltDate(article?.seendate);
      const url = canonicalArticleURL(article?.url);
      if (!titleKey || score < 1 || timestamp == null || timestamp < oldest || timestamp > newest || !url) continue;
      const urlKey = `${url.hostname}${url.pathname}`;
      if (seenTitles.has(titleKey) || seenURLs.has(urlKey)) continue;
      seenTitles.add(titleKey);
      seenURLs.add(urlKey);
      curated.push({
        title,
        url: url.toString(),
        domain: String(article?.domain || url.hostname.replace(/^www\./, "")).trim().slice(0, 100),
        seen: formatDate(timestamp),
        timestamp,
        relevance: score,
      });
    }

    return curated
      .sort((a, b) => b.timestamp - a.timestamp || b.relevance - a.relevance || a.title.localeCompare(b.title))
      .slice(0, maxItems)
      .map(({ timestamp, relevance, ...item }) => item);
  }

  return {
    DEFAULT_MAX_ITEMS,
    DEFAULT_WINDOW_DAYS,
    buildGdeltURL,
    curateNewsArticles,
    parseGdeltDate,
    relevanceScore,
  };
});
