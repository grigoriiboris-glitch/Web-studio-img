# Security baseline

The API follows a private-by-default security model.

## Secrets

Production secrets are supplied through environment variables or a secret manager. They are never stored in frontend code, Git, or checked-in configuration. JWT_SECRET must be at least 32 characters in production.

## HTTP security

The API applies:
- strict security response headers;
- an explicit CORS allowlist;
- request IDs;
- a configurable in-memory request rate limit;
- a 10 MiB request body limit;
- JSON APIs without reflected arbitrary origins.

The default rate limit is 120 requests per minute per client address and is configurable with RATE_LIMIT_REQUESTS and RATE_LIMIT_WINDOW.

The in-memory limiter is a single-process baseline. A distributed deployment must move rate-limit state to a shared store such as Redis.

## Upload validation

Image uploads must use JPEG, PNG, or WebP. The declared media type is normalized and compared with http.DetectContentType, and the body is bounded before it is read.

Image upload endpoints must call security.ValidateImage before persisting bytes.

## Object storage

Storage keys reject absolute paths, traversal segments, empty path components, dot and dot-dot segments, and Windows path separators. Presigned URLs are limited to 15 minutes.

Object storage credentials remain backend-only.

## CORS and CSRF

CORS uses an explicit origin allowlist and credentials are enabled only for listed origins.

The current authentication foundation does not yet issue cookie-based sessions. When cookie authentication is introduced, all state-changing requests must require a CSRF token in a custom header and same-site cookie configuration. Bearer-token APIs do not rely on CORS as an authorization boundary.

## Audit logging

HTTP request logs include request IDs. Domain-level security/audit events should be recorded separately from HTTP access logs when authentication and project authorization are implemented.

## Observability integration

Security middleware remains part of the API request chain when OpenTelemetry request metrics and tracing are enabled. The observability middleware wraps the secured handler rather than bypassing CORS, security headers, request IDs, rate limiting, logging, or upload size limits.

## Remaining acceptance checks

The security baseline is designed to support tests for unauthorized resource access, invalid and oversized uploads, rate limiting, secret handling, path traversal, SQL injection prevention through parameterized repositories, and signed URL expiry. Resource ownership checks become enforceable when the corresponding domain endpoints are introduced.
