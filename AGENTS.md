# Repository Guidelines

## Purpose of This File
- This file defines mandatory rules for AI agents and humans using AI while working in this repository.
- Follow these rules unless a repository maintainer explicitly overrides them.
- The OpenSpec block above applies only when OpenSpec triggers are met; otherwise follow the repository rules below.

## Agent Operating Principles
- You MUST preserve existing behavior unless a change is explicitly requested.
- You MUST keep changes minimal and targeted to the task.
- You SHOULD prefer small, understandable edits over broad refactors.
- You MUST keep the codebase maintainable and readable.

## Allowed and Forbidden Actions
- You MAY add or edit files needed to complete the requested task.
- You MAY run tests and formatting tools relevant to the changed code.
- You MUST NOT delete files, change licenses, or modify CI/CD workflows without explicit approval.
- You MUST NOT modify `dashboard/dist` unless the UI source has been updated and rebuilt.
- You MUST NOT introduce new dependencies without explicit approval.

## Code Quality Standards
- Go code MUST be `gofmt`-formatted and use idiomatic Go structure.
- Keep packages short and lowercase (e.g., `pkg/utils`, `pkg/handlers`).
- Prefer clear function names and avoid overly clever abstractions.
- Update comments only when behavior changes or when clarity improves.

## Change Scope & Discipline
- Stay within the scope of the request; avoid opportunistic cleanup.
- If a refactor is needed, ask before doing it unless the change is trivial and isolated.
- Do not alter public APIs or configuration formats unless explicitly requested. See `docs/api.md`, `docs/api.yaml`, and `docs/additional-config.md` for references.

## Testing & Validation
- You MUST run tests for touched Go packages when feasible (e.g., `go test ./...`).
- If tests are not run, you MUST state why and what was skipped.
- If tests fail, you MUST report failures and avoid masking them.
For docs-only changes, tests may be skipped but must be stated explicitly.

## Security & Safety Rules
- You MUST NOT add secrets, credentials, tokens, or private URLs to the repo.
- Configuration examples MUST use placeholders (e.g., `YOUR_TOKEN`).
- You MUST NOT weaken authentication, authorization, or TLS defaults.
- Dependency updates MUST be approved by a maintainer. If required, justify risk.

## Documentation Responsibilities
- Update `README.md` and/or `docs/` when interfaces, flags, or behavior change.
- For documentation changes, validate with `mkdocs serve` when feasible.
- Keep documentation concise and aligned with actual behavior.

## Local Testing (kind)
- If you need to manually update the deployment image, run: `kubectl set image deployment/oscar -n oscar oscar=localhost:5001/oscar:devel`.
- To rebuild and redeploy the OSCAR manager image for local testing, run `make deploy`.
- This builds and pushes `localhost:5001/oscar:devel` and restarts the `oscar` deployment.
- Ensure your kind cluster is running and the local registry is reachable.

## Commit & PR Expectations (if applicable)
- Follow short, imperative commit messages; use `fix:`/`docs:` prefixes when appropriate.
- PRs MUST include a clear description and any relevant links to issues.
- Follow `CONTRIBUTING.md` requirements for version bumps and reviewer approvals.

## When to Ask for Clarification
- Requirements conflict with this file or are ambiguous.
- The change requires new dependencies, CI/CD edits, or license changes.
- The change affects external interfaces or deployment behavior.
- Tests cannot be run or consistently fail.
If AGENTS.md and OpenSpec instructions conflict, follow AGENTS.md unless a maintainer says otherwise.

## Active Technologies
- Go 1.25 + gin-gonic, client-go, metrics.k8s.io clien (001-metrics-collection)
- N/A (aggregation from existing data sources) (001-metrics-collection)

## Recent Changes
- 001-metrics-collection: Added Go 1.25 + gin-gonic, client-go, metrics.k8s.io clien
