# SA Site Plan Generator

A Go web application that generates development application site plan PDFs from aerial imagery for South Australian properties. Enter an address, get a PDF with cadastral boundaries, dimensions, satellite imagery, north arrow, and scale bar.

## Features

- **Address Geocoding** — Google Maps Geocoding API with AU-country restriction
- **Cadastral Boundary Lookup** — SA government ArcGIS REST API (Location SA layer 124)
- **Satellite Imagery** — Google Static Maps API at 1280×1280px
- **PDF Site Plan** — A3 landscape with:
  - Satellite imagery base
  - Red parcel boundary overlay with transparency
  - Boundary edge dimensions (meters)
  - North arrow
  - Scale bar with ratio
  - Title block (address, parcel ID, area, date)
  - Disclaimer and data source attribution
- **HTMX Frontend** — Single-page form, no JavaScript framework
- **Extensible** — `cadastre.Provider` interface for adding other Australian states

## Quick Start

```bash
# Clone
git clone https://github.com/whatnick/site-planner.git
cd site-planner

# Set your Google Maps API key (needs Geocoding + Static Maps APIs enabled)
export GOOGLE_API_KEY=your-key-here

# Run
go run ./cmd/server
# → http://localhost:8080
```

## Requirements

- Go 1.22+
- Google Maps API key with:
  - Geocoding API enabled
  - Maps Static API enabled

## Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `GOOGLE_API_KEY` | Yes | — | Google Maps Geocoding + Static Maps |
| `PORT` | No | `8080` | HTTP server port |

## Architecture

```
cmd/server/main.go          → HTTP entrypoint, graceful shutdown
internal/config/             → Environment variable loading
internal/models/             → Shared types (Parcel, SitePlan, Coord, ParcelEdge)
internal/geocode/            → Google Geocoding API client
internal/cadastre/           → SA ArcGIS REST cadastre provider + Provider interface
internal/imagery/            → Google Static Maps satellite image + coordinate transforms
internal/planner/            → PDF composition (gofpdf): layout, overlays, annotations
internal/handler/            → HTTP handlers (HTMX partials), PDF lifecycle management
templates/                   → html/template files (index.html, result.html)
static/                      → Static assets
```

## How It Works

1. User enters a South Australian address
2. Address is geocoded to lat/lng via Google Maps
3. Cadastral parcel boundary is fetched from SA government spatial services
4. Satellite imagery is fetched from Google Static Maps, centered on the parcel
5. An A3 landscape PDF is composed with boundary overlay, dimensions, and annotations
6. PDF is served for download (auto-cleaned after 1 hour)

## Data Sources

- **Cadastre**: [Location SA](https://location.sa.gov.au) — Government of South Australia (ArcGIS REST, layer 124)
- **Imagery**: Google Maps Static API
- **Geocoding**: Google Maps Geocoding API

## Build

```bash
go build ./...          # Build all packages
go vet ./...            # Static analysis
go test ./...           # Run tests
```

## License

MIT
