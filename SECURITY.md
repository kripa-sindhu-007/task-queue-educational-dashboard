# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in this project, please report it
**privately** — do not open a public issue, pull request, or discussion.

Preferred channels:

1. **GitHub private advisory** — open a report via
   **Security → Advisories → Report a vulnerability** on this repository
   ([how to](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)).
2. **Email** — **sindhukripa007@gmail.com** with the details below.

Please include:

- A description of the vulnerability and its impact
- Steps to reproduce (a proof of concept is ideal)
- Affected component(s) and version/commit
- Any suggested remediation

**Response targets** (best effort — this is a solo-maintained, educational project):

- Acknowledgement within **5 business days**
- An assessment and remediation plan within **15 business days** of triage

Please give us reasonable time to investigate and release a fix before any public
disclosure. Reporters who wish to be acknowledged will be credited.

## Supported versions

This is an actively developed, educational project without formal release
versioning. Security fixes land on the **`main`** branch; please test against the
latest `main` before reporting.

| Version | Supported          |
|---------|--------------------|
| `main`  | :white_check_mark: |
| older commits / tags | :x:   |

## Security notes for operators

This project is built to **teach** distributed-systems internals and to run on a
trusted local machine (`docker compose up`). It is **not hardened for public
exposure**. Before running it anywhere beyond localhost, be aware:

- **The dashboard API is unauthenticated and uses permissive CORS.** There is no
  login or API key — anyone who can reach the API can enqueue tasks and read
  queue state. Do not expose it directly to the internet; put it behind your own
  authentication/network controls if you must.
- **Redis has no password by default.** `REDIS_PASSWORD` defaults to empty. Set a
  strong password (and don't publish the Redis port) in any shared environment.
- **No secrets live in this repository.** The only credentials are the CI Docker
  Hub publish tokens, held as GitHub Actions secrets (`DOCKERHUB_USERNAME`,
  `DOCKERHUB_TOKEN`) — never commit real tokens to the repo.
- **Default configuration favors observability over lockdown** (open metrics
  endpoints, verbose internals) — that's intentional for learning, and another
  reason to keep deployments on a trusted network.

Thank you for helping keep this project and its users safe.
