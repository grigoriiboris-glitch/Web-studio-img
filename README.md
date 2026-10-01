# Web Studio IMG

## Docker

Create the local environment file once:

```bash
cp .env.example .env
```

Development keeps the Vue/Vite frontend as a separate container with hot reload:

```bash
docker compose -f docker-compose.dev.yml up --build
```

The browser uses `http://localhost:5173`; Vite proxies `/api` to the backend container. Development keeps Postgres, Redis and MinIO ports available on the host for local tooling.

Production builds the frontend first, then copies `frontend/dist` into the backend image. The Go API serves the SPA and API from the same container:

```bash
# Set these in .env before starting production.
APP_ENV=production
WEB_STATIC_DIR=/app/web
STORAGE_PROVIDER=s3
docker compose -f docker-compose.prod.yml up --build -d
```

Production exposes only the application port. Postgres, Redis and MinIO stay on the internal Compose network. ClamAV runs as an internal service and the backend/worker use `clamav:3310` for malware scanning. The project uses the official ClamAV image and a pinned MinIO release in the production Compose file. citeturn746611search10turn499177search1

### Local filesystem storage

For a single-host/private deployment without S3/MinIO, set:

```env
APP_ENV=production
WEB_STATIC_DIR=/app/web
STORAGE_PROVIDER=local
STORAGE_LOCAL_DIR=/data/assets
STORAGE_SIGNING_SECRET=change-this-to-a-random-secret
```

The API and worker share the `assets_data` volume. Asset transfer still uses short-lived signed URLs, but the bytes are stored directly in the mounted filesystem. No MinIO credentials are required for this mode.

Prometheus and Grafana are no longer part of the application stack. Runtime configuration belongs in `.env`; `.env.example` is the template and contains no real credentials.
