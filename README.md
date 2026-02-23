# jot backend (boilerplate)

Go modular-monolith backend with hexagonal architecture.

## Highlights
- `pages` and `blocks` are separate entities
- block payload stored in `jsonb` (`blocks.data`)
- flat fetch strategy: `SELECT * FROM blocks WHERE page_id = $1 ORDER BY position`
- NATS JetStream events
- gRPC + HTTP bootstrap
- Zap logging + OpenTelemetry tracing
- Podman stack with Postgres, NATS, OTel, Tempo, Loki, Grafana

## Minimal HTTP API
- `POST /v1/media/images` upload image file (multipart `file`) to object storage
- `POST /v1/pages` create a page
- `GET /v1/pages/{pageId}` fetch page with flat ordered blocks
- `PUT /v1/pages/{pageId}/blocks` replace blocks (used for reorder/update)

Example payloads:

`POST /v1/pages`
```json
{
	"title": "My first page",
	"blocks": [
		{
			"id": "b1",
			"type": "paragraph",
			"position": 0,
			"data": {"text": "hello"}
		},
		{
			"id": "b2",
			"parent_id": "b1",
			"type": "image",
			"position": 1,
			"data": {"url": "https://example.com/image.png"}
		}
	]
}
```

`PUT /v1/pages/{pageId}/blocks`
```json
{
	"blocks": [
		{
			"id": "b2",
			"type": "image",
			"position": 0,
			"data": {"url": "https://example.com/image.png"}
		},
		{
			"id": "b1",
			"type": "paragraph",
			"position": 1,
			"data": {"text": "hello"}
		}
	]
}
```

## Start with Podman
```bash
cd /Users/animr/jot/deployments/podman
podman compose -f compose.yml up --build
```

MinIO is exposed at:
- API: `http://localhost:9000`
- Console: `http://localhost:9001`

## Test and build
```bash
cd /Users/animr/jot
go mod tidy
go test ./...
go build ./cmd/jotd
```
