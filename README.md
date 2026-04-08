# AI-Powered Land Risk Assessment System

Production-grade, fully containerized land risk pipeline with:

- Go Fiber backend
- Python XGBoost ML microservice
- React + Leaflet frontend
- PostgreSQL + Redis
- Docker Compose + Makefile orchestration

## One-command startup

```bash
make up
```

This command:

1. Builds services (if needed)
2. Starts PostgreSQL, Redis, ML service, backend, frontend
3. Runs database migrations
4. Seeds sample data

## Service URLs

- Frontend: [http://127.0.0.1:13000](http://127.0.0.1:13000)
- Backend API: [http://127.0.0.1:18080](http://127.0.0.1:18080)
- ML service: [http://127.0.0.1:18000](http://127.0.0.1:18000)
- PostgreSQL: 127.0.0.1:15432

## Makefile commands

```bash
make build
make up
make down
make restart
make logs
make clean
make migrate
make seed
make test
make integration-test
```

## Backend APIs

- `POST /analyze`
- `GET /report/:id`
- `GET /report/:id?format=pdf`
- `GET /trace/:id`
- `GET /health`

`POST /analyze` now includes a non-technical explanation at `report.citizenSummary`.

`GET /report/:id` (JSON) and `GET /report/:id?format=pdf` include the same non-technical explanation.

## ML APIs

- `POST /predict`
- `POST /satellite-features`
- `GET /health`

## Testing

- Go unit tests: preprocessing, validation, handler APIs
- Python unit tests: NDVI computation, XGBoost inference
- Integration smoke test: `make integration-test`

## Notes

- KGIS requests run concurrently with retries, context timeouts, caching, and trace capture.
- Satellite features are fetched from Sentinel-2/Landsat STAC assets.
- Validation layer applies cross-source confidence and deterministic risk override for high water overlap.
- Reports are persisted to PostgreSQL and retrievable as JSON/PDF.
- If `OPENROUTER_API_KEY` is set, the backend generates plain-language summaries for non-technical users. If unavailable, it falls back to deterministic wording.

## Optional OpenRouter settings

- `OPENROUTER_API_KEY`
- `OPENROUTER_MODEL` (default: `openai/gpt-oss-120b:free`)
- `OPENROUTER_BASE_URL` (default: `https://openrouter.ai/api/v1`)
- `OPENROUTER_TIMEOUT_SECONDS` (default: `8`)
