# AGENTS.md - dp1-go

Top-level repo contract for coding agents.

Tool-specific adapters live in `.cursor/`, `.codex/`, `opencode.json`.

## Purpose

This repository is the **Go SDK for [DP-1](https://github.com/display-protocol/dp1)** (Display Protocol 1): playlists, playlist-groups (exhibitions), ref manifests, optional registered JSON Schema extensions (e.g. playlists, channels), and helpers for **JCS canonicalization (RFC 8785)**, **payload hashing**, and **Ed25519** verification (legacy single-signature and v1.1+ multi-signature).

**Non-goals:** Implementing a full network player, license server, or UI. Those stay in consuming apps; this library supplies parsing, validation, signing primitives, and display-field merge order consistent with the spec.

**Normative source of truth:** The DP-1 specification and schema files in the upstream spec repository. Embedded copies under `internal/schema/` must stay aligned with that spec; when the spec changes, update schemas, overlays, and any affected Go types/tests in the **same change**.

## Coding defaults

- Prefer the simplest change that preserves correctness, reliability, and debuggability.
- Avoid silent failure paths; errors and degraded states should be explicit and actionable.
- Treat tests, generators, schema updates, and docs updates as part of the same change, not follow-up work.

## Coding style

- Follow standard Go style and Go doc conventions.
- Prefer comments more than usual when they add future maintenance value for later agentic coding sessions.
- Use comments to preserve design intent, trade-offs, invariants, failure modes, operational constraints, and reasons a simpler-looking alternative was not chosen.
- Store important amendment context close to the code when that context would otherwise be lost in a later session.
- Do not add filler comments that only restate obvious syntax or line-by-line behavior.

### Protocol and API invariants (do not break casually)

- **Validation before decode:** Public `ParseAndValidate*` entrypoints validate raw JSON against embedded JSON Schema (draft 2020-12), then `json.Unmarshal` into typed structs. Keep that order so decoded values always reflect schema-valid documents.
- **Stable error semantics:** `ErrValidation` and `ErrSigInvalid` (and related) are re-exported for `errors.Is` / `errors.As`. `CodedError` + `ErrorCode` map to DP-1 §14 where applicable; reserved §14 codes that this SDK does not emit remain documented in `errors.go` for players—do not repurpose them for unrelated failures.
- **Signing and hashing:** Payload canonicalization follows DP-1 §7.1: strip top-level `signature` / `signatures`, JCS the remainder, append a single trailing LF to the octets that are hashed, then SHA-256; Ed25519 signs the **32-byte** digest. Any change here is a **compatibility break** with other implementations—add tests that lock the behavior to the spec wording.
- **Merge package:** `merge` applies resolution order: defaults → ref manifest controls → item `override` → item-local fields. Changes must preserve that ordering and remain consistent with README and spec intent.
- **Test hooks:** `PlaylistCoreSchemaValidate` and the other `*SchemaValidate` vars in `parse.go` exist only for tests; they must not be reassigned concurrently in production.

## Workflow

- **Branching:** Use short-lived branches (e.g. `feature/…`, `fix/…`) and open PRs against the default branch.
- **Spec alignment:** When updating DP-1 behavior or schemas, cite or cross-check the relevant DP-1 section (e.g. §7.1 signing, §12 `dpVersion`, §14 error codes) in the PR description so reviewers can verify against the spec.
- **Dependencies:** Prefer minimal, well-maintained deps. Bumping `jsonschema`, JCS, or crypto-related packages should include a quick run of `go test ./... -race` and a note in the PR if behavior could change.
- **README:** Keep `README.md` usage examples in sync with exported APIs (module path, import paths, function names).

## Repo-specific change rules

- **Schemas:** Files live under `internal/schema/` (`core/`, `extensions/`, `overlay/`) and are embedded via `internal/schema/embed.go`. Every schema JSON must have a correct `$id` matching what `internal/validate` registers. Add or update overlays when combining core + extension validation in one path.
- **New document types or extensions:** Add schema → compiler registration in `validate` → `ParseAndValidate*` in the root package → typed structs in `playlist`, `playlistgroup`, `refmanifest`, or `extension/…` as appropriate. Add `ErrorCode` + `CodeFrom*Validation` when the UI/telemetry contract needs a stable code.
- **Signing:** Keep signing logic in `sign/` and JCS in `jcs/`; avoid duplicating canonicalization in callers.
- **Concurrency:** `jsonschema.Compiler.Compile` is not safe for concurrent use on the same instance; `internal/validate` uses a mutex around compile—preserve that if touching validation.
- **Coverage:** `scripts/check-coverage.sh` merges coverage across packages but excludes the `internal/schema` package (embed-only). New code should include tests; do not drop total coverage below the CI threshold without team agreement.

## Verification

Before considering work complete:

```bash
go test ./... -race -count=1
golangci-lint run   # or rely on CI; config is `.golangci.yml`
bash scripts/check-coverage.sh 80
```

CI runs tests with race and merged coverage (≥ 80%) on push and PRs; lint runs `golangci-lint` v2.8.x (see `.github/workflows/`).

## Review and done

A change is ready when:

- **Correctness:** Behavior matches DP-1 for the touched surface (signing, validation, merge, or versioning).
- **Tests:** New or updated tests cover the change; edge cases for validation, signing, or merge are exercised where risk is high.
- **Schemas/types:** If JSON shape changed, schema files and Go structs stay in sync; `go generate` is not used for schemas here—embeds are committed.
- **Docs:** `README.md` or package docs updated if public API or developer workflow changed.
- **CI:** Local `go test ./... -race` and coverage script pass; no new lint issues.

## Commit style

- Use imperative subject lines (`Add playlist overlay validation`, `Fix payload_hash mismatch error wrapping`).
- Keep the subject ~72 characters or less; put spec references or rationale in the body when helpful.
- One logical concern per commit when possible; avoid mixing unrelated refactors with functional fixes.
