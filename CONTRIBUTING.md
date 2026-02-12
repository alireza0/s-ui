# Contributing to S-UI

Thank you for your interest in contributing to S-UI. This document explains how to set up a development environment, follow project conventions, and submit changes. Your contributions help make the **multi-inbound-per-user** approach and the rest of the project better for everyone.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Development Environment Setup](#development-environment-setup)
- [Coding Conventions and Style Guide](#coding-conventions-and-style-guide)
- [Testing](#testing)
- [Features That Need Help](#features-that-need-help)
- [Pull Request Process](#pull-request-process)
- [Adding This Guide in Your Repository](#adding-this-guide-in-your-repository)
- [Reporting Bugs and Requesting Features](#reporting-bugs-and-requesting-features)

---

## Code of Conduct

Please be respectful and constructive when interacting with maintainers and other contributors. This project is for personal learning and communication; use it responsibly and legally.

---

## Development Environment Setup

### Prerequisites

- **Go**: 1.25 or later (see `go.mod` for the exact version).
- **Git**: For cloning and submodules.
- **C compiler**: Required for CGO (e.g. `gcc`, `musl-dev` on Alpine).
- **Node.js** (optional): Only if you plan to work on or rebuild the frontend. The repo can be run with pre-built frontend assets.

### Clone and Submodules

```bash
git clone https://github.com/alireza0/s-ui
cd s-ui
git submodule update --init --recursive
```

The **frontend** lives in a submodule. If you only work on the backend, you can use the existing `web/html` contents or build the frontend once (see below).

### Backend-Only Development (quickest)

1. Build and run with the provided script (builds backend and runs with debug + local DB):

   ```bash
   ./runSUI.sh
   ```

   This runs `./build.sh` then `SUI_DB_FOLDER="db" SUI_DEBUG=true ./sui`.

2. Or build manually:

   ```bash
   ./build.sh
   SUI_DB_FOLDER=db SUI_DEBUG=true ./sui
   ```

   Default panel: **http://localhost:2095/app/** (user: `admin`, password: `admin` — change in production).

### Full Stack (Backend + Frontend)

1. **Frontend** (separate repo in submodule):

   ```bash
   cd frontend
   npm install
   npm run build
   cd ..
   ```

2. **Replace web assets and build backend**:

   ```bash
   mkdir -p web/html
   rm -rf web/html/*
   cp -R frontend/dist/* web/html/
   go build -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor" -o sui main.go
   ```

3. Run:

   ```bash
   SUI_DB_FOLDER=db SUI_DEBUG=true ./sui
   ```

### Build Tags

The backend is built with these tags for full functionality:

- `with_quic`, `with_grpc`, `with_utls`, `with_acme`, `with_gvisor`

Use the same tags when building locally if you need feature parity with releases.

### Environment Variables (development)

| Variable       | Description                    | Example   |
|----------------|--------------------------------|-----------|
| `SUI_DB_FOLDER`| Directory for SQLite DB files  | `db`      |
| `SUI_DEBUG`    | Enable debug mode              | `true`    |
| `SUI_LOG_LEVEL`| Log level                      | `debug`   |
| `SUI_BIN_FOLDER` | Directory for binaries       | `bin`     |

### Docker (optional)

```bash
git clone https://github.com/alireza0/s-ui
cd s-ui
git submodule update --init --recursive
docker build -t s-ui .
# or: docker compose up -d
```

---

## Coding Conventions and Style Guide

### General

- Write clear, maintainable code. Prefer small, focused functions and packages.
- Comment non-obvious logic and public APIs.
- Handle errors explicitly; avoid ignoring `err` unless intentional.

### Go Style

- Follow **standard Go style** and **[Effective Go](https://go.dev/doc/effective_go)**.
- Run **gofmt** (or **goimports**) before committing:

  ```bash
  gofmt -w .
  # or: goimports -w .
  ```

- Use **camelCase** for unexported names and **PascalCase** for exported names.
- Keep package names short and lowercase (e.g. `api`, `service`, `util`).
- Group imports: standard library, then third-party, then project imports (as in existing files).

### Project Structure Conventions

- **`api/`**: HTTP handlers and API routing (e.g. `apiHandler.go`, `apiV2Handler.go`).
- **`service/`**: Business logic and panel/core operations.
- **`database/model/`**: GORM models and DB entities.
- **`util/`**: Shared utilities (e.g. link/sub conversion, JSON).
- **`core/`**: sing-box integration and core runtime.
- **`sub/`**: Subscription (link/json) handling.

When adding new features, place code in the appropriate layer (handler → service → model/util) and avoid circular dependencies.

### Naming and Patterns

- Handlers: suffix `Handler` (e.g. `APIHandler`, `APIv2Handler`).
- Services: suffix `Service` or use package name (e.g. `ApiService`, `LinkService`).
- Models: clear struct names with JSON/gorm tags (see `database/model/`).

---

## Testing

### Current State

- The project does not yet have a formal test suite (no `*_test.go` files in the repo).
- CI currently focuses on **builds** (e.g. `release.yml`) rather than automated tests.

### What You Can Do Now

1. **Build verification**: Before submitting a PR, ensure the project builds:

   ```bash
   go build -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor" -o sui main.go
   ```

2. **Manual testing**: Run with `./runSUI.sh`, test the changed area (panel, API, subscription, etc.).

3. **Future tests**: Contributions that add **unit tests** (e.g. for `util/`, `service/`, or API handlers) or **integration tests** are very welcome. Prefer the standard library `testing` package and table-driven tests where appropriate.

### Running the Linter (optional)

```bash
go vet ./...
# Optional: staticcheck, golangci-lint, etc.
```

---

## Features That Need Help

Community help is especially valuable in these areas. Check the [Issues](https://github.com/alireza0/s-ui/issues) for current tasks and ideas.

### High-Value Areas

- **Multi-inbound per user**: Core differentiator of S-UI; improvements to UX, docs, and robustness are welcome.
- **API (v1 and v2)**: Completeness, consistency, and documentation (see [API Documentation](https://github.com/alireza0/s-ui/wiki/API-Documentation)).
- **Subscription service**: Link conversion, JSON subscription, and info endpoints (`sub/`, `util/`).
- **Testing**: Adding unit and integration tests for critical paths.
- **Documentation**: User docs, API examples, and contribution docs (like this file).
- **Platform support**: macOS is experimental; Windows and Linux improvements are welcome (see `windows/` and `.github/workflows/`).

### How to Find Tasks

- **Good first issue**: Look for issues labeled `good first issue` or `help wanted`.
- **Feature requests**: [Feature request template](.github/ISSUE_TEMPLATE/feature_request.md).
- **Bugs**: [Bug report template](.github/ISSUE_TEMPLATE/bug_report.md).

If you want to work on a larger feature, open an issue first to discuss approach and avoid duplicate work.

---

## Pull Request Process

1. **Fork and branch**

   - Fork the repository on GitHub.
   - Create a branch from `main`: e.g. `git checkout -b fix/issue-123` or `feature/sub-improvements`.

2. **Make your changes**

   - Follow the [Coding Conventions](#coding-conventions-and-style-guide).
   - Run `gofmt` and ensure the project builds (see [Testing](#testing)).
   - Keep commits focused and messages clear (e.g. "Fix link conversion for VMess", "Add tests for outJson").

3. **Push and open a PR**

   - Push your branch and open a Pull Request against `main`.
   - Use the PR description to explain:
     - What problem or feature the PR addresses.
     - What you changed and how to verify it.
   - Reference any related issue (e.g. "Fixes #123").

4. **Review and CI**

   - Maintainers will review your code. CI (e.g. build workflows) must pass.
   - Address feedback by pushing new commits to the same branch.

5. **Merge**

   - Once approved and CI is green, a maintainer will merge your PR. Thank you for contributing!

### PR Guidelines

- Prefer **small, reviewable PRs**. Split large features into logical steps.
- Avoid unrelated changes (e.g. formatting-only or refactors in a feature PR).
- Ensure your branch is up to date with `main` before submitting (rebase or merge as the project prefers).

---

## Adding This Guide in Your Repository

If you maintain a fork or your own repository and want the contribution guide to be visible and linked properly:

1. **Keep `CONTRIBUTING.md` in the repository root**  
   GitHub automatically discovers a file named `CONTRIBUTING.md` (or `CONTRIBUTING`) in the root. When someone opens a new issue or pull request, GitHub can show a link to it. The community profile also uses it for the “Contributing” section.

2. **Link from README**  
   Add a short line in your main `README.md` so new contributors see it when they land on the repo, for example:
   ```markdown
   **Want to contribute?** See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding conventions, and the pull request process.
   ```

3. **Optional: GitHub “Contributing” link**  
   In the repository **Settings → General → Features**, ensure “Issues” (and optionally “Discussions”) are enabled. The link to `CONTRIBUTING.md` appears when users create a new issue or PR; no extra config is needed as long as the file is in the root.

4. **When forking**  
   If you fork S-UI, `CONTRIBUTING.md` is already in the repo. Update the clone URLs and repo names in this file if you want your fork’s contribution instructions to point to your own repository.

---

## Reporting Bugs and Requesting Features

- **Bugs**: Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md). Include version, OS, steps to reproduce, and expected vs actual behavior.
- **Features**: Use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.md). Describe the use case and, if possible, a proposed approach.
- **Questions**: Use the [question template](.github/ISSUE_TEMPLATE/question-template.md) or discussions if enabled.

---

Thank you for helping S-UI grow. Your contributions make it possible for more users to adopt S-UI in production and benefit from its multi-inbound-per-user design.
