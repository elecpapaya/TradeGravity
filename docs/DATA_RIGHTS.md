# Data sources, attribution, and reuse

TradeGravity separates the license for this repository from the terms that govern upstream observations and linked content.

## Repository license

The Apache License 2.0 applies to TradeGravity source code and original project documentation unless a file says otherwise. It does not grant rights in third-party datasets, API responses, trademarks, flags, news articles, or other externally supplied material.

Generated JSON, charts, explanations, and CSV exports may contain transformed or summarized third-party observations. Anyone reusing those outputs is responsible for checking the current terms and metadata of the selected provider and underlying dataset.

## Open-data boundary

TradeGravity's published analytical metrics use only free/public inputs that another user can access without buying a market-data subscription. The semiconductor reference validates this as `data_policy.mode = free_public_only`. Paid market databases, licensed proprietary fab-capacity files, undisclosed vendor estimates, and a paywalled source required to reproduce a metric are out of scope. Public access does not mean unrestricted copyright: the provider-specific terms and attribution below still apply.

## Provider-specific references

| Provider or service | How TradeGravity uses it | Terms and attribution reference |
| --- | --- | --- |
| WITS | Default source for bilateral trade observations | [WITS legal page](https://wits.worldbank.org/wits/legal.html); WITS notes that content rights can belong to the respective content owner. Check the specific database metadata exposed through WITS. |
| UN Comtrade | HS2 and strategic HS6 products, focused monthly semiconductor observations, bilateral matrices, and mirror-reporting diagnostics | [UN Comtrade usage agreement](https://comtrade.un.org/licenseagreement.html) and [UN Comtrade data-use explanation](https://comtrade.un.org/labs/data-explorer/About.html). UN Comtrade data are copyrighted by the United Nations and reuse is governed by its policy. |
| World Bank Open Data | Region, income, population, and GDP context published in `context.json` and joined into `latest.json`; optional live snapshot fields | [World Bank dataset terms summary](https://data.worldbank.org/summary-terms-of-use). Dataset-specific metadata can add or change conditions, especially for third-party indicators. |
| GDELT | Recent headline metadata fetched in the browser; not included in `latest.json` | [GDELT terms of use](https://www.gdeltproject.org/about.html). GDELT asks users and redistributors to cite and link to the GDELT Project. Linked news articles remain on their publishers' sites. |
| OECD ICIO | Declared, lagged industry-level input-output/value-added context; not substituted for HS6 and not yet computed into the viewer | [OECD ICIO dataset page](https://www.oecd.org/en/data/datasets/inter-country-input-output-tables.html) and the attribution/reuse information accompanying the download. Preserve edition, industry aggregation, country coverage, and reference year. |
| NIST, BIS, European Commission, METI, China NDRC/MOFCOM, and other official links | Dated, paraphrased semiconductor policy/project context and source links | Follow each publisher's terms. TradeGravity stores short original summaries and factual metadata, not copies of reports, and preserves the source's announced/planned/operating status. |
| SEMI public releases | Dated industry context that is freely visible on the public web | Follow the linked publisher's terms. Forecasts remain external context rather than TradeGravity observations, and no paid SEMI dataset is required. |

The experimental headline panel requests recent trade and supply-chain terms, filters and deduplicates titles in the browser, and limits article links to HTTPS. Its `sourcecountry` scope describes the country assigned to the publishing outlet; it is not proof that the selected country is the subject of an article. API failure, rate limiting, and zero relevant results are displayed separately. Headline metadata is optional context and is never an input to TradeGravity's published trade values or analytical signals.

Terms can change. These links are references, not a replacement for reading the controlling terms for an intended use.

Mirror artifacts reproduce two reported perspectives and a transparent difference calculation. They do not designate either report as truth or create a new reconciled dataset. Reusers must retain both source identities, period, flow direction, and the CIF/FOB/timing/classification caveats.

## Recommended attribution record

For a reproducible use of TradeGravity output, record:

- TradeGravity repository URL and release or commit, when available;
- `schema_version`, `provider`, and `generated_at` from `meta.json`;
- reporter, partner, flow, observation period, and period type for every cited value;
- the upstream provider and dataset attribution required by its current terms; and
- any transformation or filtering performed after download.

Do not imply that the World Bank, United Nations, GDELT, a reporting country, or another provider endorses TradeGravity or a conclusion drawn from it. This document describes project handling and is not legal advice.
