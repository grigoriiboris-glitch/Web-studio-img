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

The browser uses `http://localhost:5173`; Vite proxies `/api` to the backend container.

Production builds the frontend first, then copies `frontend/dist` into the backend image. The Go API serves the SPA and API from the same container:

```bash
# Set APP_ENV=production and WEB_STATIC_DIR=/app/web in .env first.
docker compose -f docker-compose.prod.yml up --build -d
```

Prometheus and Grafana are no longer part of the application stack. Runtime configuration belongs in `.env`; `.env.example` is the template and contains no real credentials.
