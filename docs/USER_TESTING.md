# External user testing protocol

TradeGravity needs evidence that intended users can complete a real analytical task and interpret periods and provenance correctly. Maintainers must not invent participants, quotes, completion times, or findings.

Recruitment copy and the maintainer checklist are in [USER_RECRUITMENT.md](USER_RECRUITMENT.md). The public coordination record is [issue #3](https://github.com/elecpapaya/TradeGravity/issues/3).

## Target participants

Recruit at least three people outside the implementation team, ideally one student, one researcher or analyst, and one developer or data-tool user. Participation is voluntary. Do not collect names, employers, recordings, or sensitive information unless the participant explicitly consents to publication.

## Task

Without coaching beyond the prompt, ask each participant to:

> Compare ASEAN reporters' same-period USA/China trade for 2023, select Viet Nam, identify whether its relationship is larger with the USA or China, inspect its recent trend and leading HS2 chapter, then share the exact view and export its evidence.

The expected product flow is: choose `Y:2023` → `Same-period only` → `ASEAN` → select `VNM` → inspect trend/product/quality/explanation → copy URL → export filtered JSON or CSV.

## Measures

Record only consented, non-identifying results:

- completion or point of abandonment;
- task time rounded to the nearest minute;
- whether the participant noticed the observation period and provider separation;
- whether their USA/China conclusion matches the displayed evidence;
- usability problems stated in their own words, quoted only with permission;
- severity: blocks task, causes wrong interpretation, slows task, or cosmetic;
- the tested release/commit, browser, and viewport class.
- whether the trade-focused headline panel, if opened, was understood as experimental context rather than an input to the published trade metrics.

## Public record

Create one issue per participant with the **User-study feedback** form. Use aliases such as `P1`; do not publish identity or contact details. Link follow-up bugs or feature requests. Close a feedback issue only after findings are triaged, not because every suggestion was implemented.

Summary table (fill only after real sessions):

| Participant | Public issue | Completed | Minutes | Correct interpretation | Highest severity |
| --- | --- | --- | --- | --- | --- |
| P1 | pending | pending | pending | pending | pending |
| P2 | pending | pending | pending | pending | pending |
| P3 | pending | pending | pending | pending | pending |

## Success threshold

Before citing external validation in an application, require all three public feedback issues, at least two unaided task completions, no unresolved issue that causes a wrong-period or mixed-provider interpretation, and a documented response to every blocking finding.
