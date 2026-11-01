# Security Policy

## Supported versions

Security fixes go into the latest release only. Please update to the newest version before you report an issue.

| Version         | Supported |
| --------------- | --------- |
| latest (1.6.x)  | ✅        |
| older than 1.6  | ❌        |

## Reporting a vulnerability

**Please do not report security vulnerabilities in public issues, discussions or pull requests.**

Report them privately by email to **alireza7@gmail.com**, with `[s-ui security]` in the subject.

Please include:

- the s-ui version (and how it is installed: script, Docker, or binary)
- the affected component (admin panel, API / API v2, subscription server, or something else)
- steps to reproduce, or a proof of concept
- the impact you see, and whether it needs a logged-in admin

## What to expect

- I aim to reply within 7 days.
- Once the issue is confirmed, a fix is released as soon as possible and credited in the release notes, unless you prefer to stay anonymous.
- Please give a reasonable time for a fix to ship before you disclose the issue publicly.

## Scope

In scope: the s-ui panel, its API, the subscription server, and the installer and scripts in this repository, together with the [s-ui-frontend](https://github.com/alireza0/s-ui-frontend) submodule.

Out of scope:

- vulnerabilities in sing-box itself; report those to [SagerNet/sing-box](https://github.com/SagerNet/sing-box)
- the default `admin`/`admin` login on a fresh install (it is meant to be changed during setup)
- the panel being served over plain HTTP when TLS has not been configured
- problems that require an attacker who already has an admin session or API token, unless they lead to code execution or file access on the host
