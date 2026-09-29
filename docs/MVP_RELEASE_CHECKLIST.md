# MVP Release Checklist

## Scope

This checklist is the release gate for the MVP defined by `docs/TECHNICAL_SPECIFICATION.md`.
Advanced PSD/non-destructive export remains a post-MVP capability; the MVP export contract is PNG/JPEG, masks, source assets and JSON manifest.

## Functional acceptance

- [x] Projects and ownership checks
- [x] Immutable iterations and restore-as-new-iteration
- [x] Prompt versions
- [x] References with rights metadata
- [x] Reference influence scores and persisted usage links
- [x] Generations store provider, model, model version, seed and parameters
- [x] Reproducibility metadata explicitly records that provider determinism is not guaranteed
- [x] Human actions and provenance hash chain
- [x] Similarity/composition analysis with explicit uncertainty
- [x] AI critic / iteration comparison
- [x] Explore / Develop / Finalize workflow
- [x] Professional export: final image, source assets, masks, manifest and hashes
- [x] Archive/delete lifecycle
- [x] Usage/quota/idempotency
- [x] Search
- [x] Layer/manual-edit foundation

## Security gate

- [ ] Production secrets are supplied only through environment/secret manager
- [ ] No credentials are committed to Git
- [ ] Project/resource ownership checks are verified
- [ ] Upload MIME/size/path validation is enabled
- [ ] Signed object URLs have bounded lifetime
- [ ] Production JWT secret meets the documented minimum
- [ ] HTTPS and secure cookies are enabled in production
- [ ] Dependency scan is green

## Data integrity

- [ ] Apply all versioned migrations from an empty database
- [ ] Roll back and re-apply migrations in CI
- [ ] Verify provenance detects payload/hash/sequence tampering
- [ ] Verify backup and restore procedure against a disposable environment
- [ ] Confirm object-storage backup coverage

## CI/CD release gate

A release is blocked unless all required GitHub Actions checks are green:

- [ ] gofmt / format check
- [ ] go vet
- [ ] unit tests
- [ ] race tests
- [ ] Go build
- [ ] backend lint
- [ ] frontend typecheck
- [ ] frontend lint
- [ ] frontend tests
- [ ] frontend build
- [ ] SQLC validation
- [ ] migration validation
- [ ] Docker builds
- [ ] Compose validation
- [ ] dependency scan

## Smoke test

1. Register/login.
2. Create a project.
3. Create an iteration and prompt.
4. Upload an asset/reference.
5. Queue a mock generation with an idempotency key.
6. Confirm generation metadata contains provider/model/seed/parameters and determinism note.
7. Link a reference to an iteration/generation and verify it appears in the Creation Report.
8. Run similarity/composition/critic tools.
9. Verify provenance.
10. Export the project and verify manifest hashes, source assets and masks.
11. Archive the project.
12. Restore/open the project without losing immutable history.

## Explicit non-goals for MVP

- PSD/non-destructive professional editing.
- Guaranteed deterministic output across providers.
- Legal originality/authorship conclusions.
- Internet-wide similarity guarantees.
