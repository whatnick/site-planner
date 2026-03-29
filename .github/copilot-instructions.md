# Project Guidelines

## Overview

SA Site Plan Generator — a Go web application that generates development application site plan PDFs from aerial imagery for South Australian properties. Enter an address, get a PDF with cadastral boundaries, dimensions, satellite imagery, north arrow, and scale bar. Optionally uses GPT-4o vision + SAM to detect existing structures and supports adding proposed buildings.

## Architecture

```
cmd/server/main.go          → HTTP entrypoint, graceful shutdown
internal/config/             → Environment variable loading
internal/models/             → Shared types (Parcel, SitePlan, Detection, ProposedBuilding)
internal/geocode/            → Google Geocoding API client
internal/cadastre/           → SA ArcGIS REST cadastre provider + Provider interface
internal/imagery/            → Google Static Maps satellite image + coordinate transforms
internal/detect/             → AI structure detection (OpenAI GPT-4o vision + SAM segmentation)
internal/planner/            → PDF composition (gofpdf): layout, overlays, annotations
internal/handler/            → HTTP handlers (HTMX partials), PDF lifecycle management
templates/                   → html/template files (index.html, result.html)
static/                      → Static assets
```

- **HTTP**: stdlib `net/http` with `http.ServeMux` — no external web framework
- **Frontend**: HTMX 2.0 via CDN — no JS build toolchain
- **Templating**: `html/template` — context-aware escaping by default
- **PDF**: `github.com/jung-kurt/gofpdf` — A3 landscape with vector drawing + image embedding
- **Spatial**: `github.com/paulmach/orb` for GeoJSON parsing and polygon operations

## Code Style

- Go standard formatting (`gofmt`/`goimports`)
- Errors wrap with `fmt.Errorf("context: %w", err)` — always add context
- All external API calls take `context.Context` as first parameter
- Use `internal/` packages — nothing is exported outside the module
- `Provider` interface pattern for multi-state extensibility (see `cadastre.Provider`)

## Build and Test

```bash
go build ./...          # Build all packages
go vet ./...            # Static analysis
go test ./...           # Run tests
go run ./cmd/server     # Start dev server (requires GOOGLE_API_KEY env var)
```

## Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `GOOGLE_API_KEY` | Yes (for generation) | — | Google Maps Geocoding + Static Maps |
| `OPENAI_API_KEY` | No | — | OpenAI GPT-4o vision for AI structure detection |
| `SAM_API_URL` | No | — | Segment Anything Model segmentation endpoint |
| `PORT` | No | `8080` | HTTP server port |

## Conventions

- **Cadastre providers**: Implement `cadastre.Provider` interface. SA provider queries ArcGIS REST at `lsa4.geohub.sa.gov.au` layer 124. New states add new provider implementations.
- **Coordinate system**: All internal coordinates are WGS84 (EPSG:4326). Distances computed via Haversine.
- **PDF layout**: Title block (top, dark), map area (center ~80%), AI detection overlays, proposed building overlays, north arrow (top-right of map), scale bar (bottom-left of map), detection legend (top-left of map when detections present), disclaimer footer.
- **AI detection**: GPT-4o vision identifies structures in satellite imagery. SAM endpoint (optional) provides precise polygon outlines. Detection is non-fatal — failures are logged and the plan generates without detection overlays.
- **Proposed buildings**: User-specified buildings rendered with dashed outlines, cross-hatch fill, and dimension labels. Position is a 0–1 fraction of the image dimensions.
- **Generated PDFs**: Stored as temp files with UUID names, auto-cleaned after 1 hour.
- **HTMX pattern**: `POST /generate` returns an HTML partial (`result.html`) swapped into `#result` div. No full page reloads.
- **No secrets in code**: API keys come from environment variables only. Never log or expose them.
