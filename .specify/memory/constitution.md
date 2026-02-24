<!--
Sync Impact Report
- Version change: 1.0.1 -> 1.0.2
- Modified principles: None
- Modified sections: Repository Constraints; Workflow & Review Expectations
- Added sections: None
- Removed sections: None
- Templates requiring updates:
  - .specify/templates/plan-template.md ✅ updated
  - .specify/templates/tasks-template.md ✅ updated
  - .specify/templates/spec-template.md ✅ reviewed, no changes needed
  - .specify/templates/commands/*.md ⚠ not present in repository
- Follow-up TODOs:
  - None
-->
# OSCAR Constitution

## Core Principles

### I. Preserve Behavior & Scope Discipline
Changes MUST preserve existing behavior unless explicitly requested and approved.
Edits MUST be minimal and targeted; avoid refactors unless trivial and isolated or
approved. Public APIs, configuration formats, and deployment behavior MUST NOT
change without explicit request. Rationale: protects existing users and
deployments.

### II. Dependency, CI, and Build Guardrails
New dependencies, license changes, and CI/CD workflow edits are forbidden without
explicit approval. `dashboard/dist` MUST NOT be modified unless the UI source is
updated and rebuilt. Rationale: avoids unreviewed supply-chain and build risk.

### III. Go Code Quality & Clarity
Go code MUST be gofmt-formatted, idiomatic, and organized with short lowercase
package names. Function and type names MUST be clear; avoid clever abstractions.
Comments only when behavior changes or clarity improves. Rationale: keeps the
codebase maintainable.

### IV. Testing & Verification
For touched Go packages, tests MUST be run when feasible (e.g., `go test ./...`).
If tests are skipped, record why; if failures occur, report them and do not mask
issues. Docs-only changes may skip tests with a note. Rationale: maintains
reliability and confidence.

### V. Security & Documentation Integrity
Never add secrets, credentials, tokens, or private URLs; examples use
placeholders. Authentication, authorization, and TLS defaults MUST NOT be
weakened. Documentation MUST be updated when interfaces, flags, or behavior
change; validate docs with `mkdocs serve` when feasible. Rationale: protects
users and keeps documentation accurate.

## Repository Constraints

- Follow `AGENTS.md` for repository guidance.
- Do not delete files, change licenses, or modify CI/CD workflows without
  explicit approval.
- Do not modify `dashboard/dist` unless the UI source has been updated and
  rebuilt.
- Avoid destructive commands unless explicitly requested.

## Workflow & Review Expectations

- Ask for clarification when requirements conflict, scope is ambiguous, or
  approvals are needed.
- Each PR/review MUST confirm constitution compliance, record test status, and
  note any skipped validation.

## Governance
The constitution supersedes all other practices in this repository. Amendments
require updating this file, the Sync Impact Report, and any affected templates or
documentation. Versioning follows semantic versioning: MAJOR for backward
incompatible governance changes or removals, MINOR for new principles/sections
or material expansions, PATCH for clarifications and wording fixes. Compliance
review is required for every change, with reviewers confirming principle
adherence and test/documentation status.

**Version**: 1.0.2 | **Ratified**: 2026-01-13 | **Last Amended**: 2026-01-13
