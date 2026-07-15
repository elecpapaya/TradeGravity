# Security policy

## Supported versions

Security fixes are applied to the default branch. After tagged releases begin, the most recent release may receive backports; pre-release snapshots and older tags may not.

## Reporting a vulnerability

Please do not disclose a suspected vulnerability in a public issue, pull request, discussion, log, or screenshot.

Use GitHub's private vulnerability reporting flow:

1. Open the repository's **Security** tab.
2. Select **Advisories**.
3. Choose **Report a vulnerability**.
4. Include affected files or versions, reproduction steps, impact, and any suggested mitigation.

If private reporting is temporarily unavailable, open a public issue that only asks the maintainer to establish a private contact channel. Do not include exploit details or secrets.

The maintainer will acknowledge a complete report as capacity allows, assess severity and affected versions, coordinate a fix, and publish an advisory when disclosure is appropriate.

## Scope

Useful reports include:

- injection or unsafe rendering of upstream data;
- credential or secret exposure;
- dependency or GitHub Actions supply-chain issues;
- unauthorized modification of generated datasets;
- validation bypasses that could publish corrupt data.

TradeGravity consumes public third-party APIs. Incorrect upstream data that is faithfully displayed is a data-quality issue rather than a security vulnerability, unless it can be used to execute code or cross a trust boundary.
