# Release Strategy (Railzway)

Railzway releases are tag-driven, human-curated, and reproducible.
This policy mirrors Grafana OSS: trust the tag, trust the release.

## Versioning

We use Semantic Versioning: `vMAJOR.MINOR.PATCH`.

- MAJOR: breaking domain or API changes
- MINOR: new features, backward compatible
- PATCH: bug fixes only

## Source of Truth

- The Git tag is the only source of version.
- No hardcoded version strings.
- CI extracts the version from the tag.

## Tagging Flow

1) Merge changes into `main`
2) Create an annotated tag:
   `git tag -a v0.1.0 -m "Railzway v0.1.0"`
3) Push the tag:
   `git push origin v0.1.0`

No releases from branches. No releases from commit hashes.

## Changelog

We maintain `CHANGELOG.md` at repo root, inspired by Keep a Changelog.

Rules:
- `## [Unreleased]` must exist
- Each release has sections: Added, Changed, Fixed, Security
- `CHANGELOG.md` is updated before tagging
- Changelog is human-curated (no auto-generation in CI)

## GitHub Releases

On tag push:
- GitHub Release is created
- Title: `Railzway vX.Y.Z`
- Body: release notes from `CHANGELOG.md`
- Include Docker image references:
  - `ghcr.io/<org>/railzway:vX.Y.Z` (Monolith)
  - `ghcr.io/<org>/railzway-admin:vX.Y.Z` (Control Plane)
  - `ghcr.io/<org>/railzway-invoice:vX.Y.Z` (Customer Plane)
  - `ghcr.io/<org>/railzway-scheduler:vX.Y.Z` (Background Plane)
  - `ghcr.io/<org>/railzway-api:vX.Y.Z` (Data Plane)

## Guarantees

- Every Docker image has a GitHub Release
- Every GitHub Release maps to exactly one tag
- Tag = behavior

## What We Don’t Do

- No auto-bumping versions
- No releases without a tag
- No noisy or auto-generated changelogs
- No hiding breaking changes
