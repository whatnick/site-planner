# SA Site Plan Generator

A Go web application that generates development application site plan PDFs from aerial imagery for South Australian properties. Enter an address, get a PDF with cadastral boundaries, dimensions, satellite imagery, north arrow, and scale bar — with optional AI-powered structure detection and proposed building placement.

## Features

- **Address Geocoding** — Google Maps Geocoding API with AU-country restriction
- **Cadastral Boundary Lookup** — SA government ArcGIS REST API (Location SA layer 124)
- **Satellite Imagery** — Google Static Maps API at 1280×1280px
- **AI Structure Detection** — GPT-4o vision analysis identifies existing structures:
  - Houses, sheds, garages, carports, driveways
  - Pools, decks, pergolas, garden beds, fences
  - Colored bounding boxes / SAM polygon overlays with legend
- **Segment Anything (SAM)** — Optional SAM endpoint for precise polygon outlines of detected structures
- **Proposed Buildings** — Add planned buildings with dimensions (width × depth in meters) at specified positions, rendered with dashed outlines and cross-hatch fill
- **PDF Site Plan** — A3 landscape with:
  - Satellite imagery base
  - Red parcel boundary overlay with transparency
  - Boundary edge dimensions (meters)
  - AI detection overlays with color-coded legend
  - Proposed building overlays with dashed outlines
  - North arrow
  - Scale bar with ratio
  - Title block (address, parcel ID, area, date)
  - Disclaimer and data source attribution
- **HTMX Frontend** — Single-page form with AI toggle and proposed building entry, no JavaScript framework
- **Extensible** — `cadastre.Provider` interface for adding other Australian states

## Quick Start

```bash
# Clone
git clone https://github.com/whatnick/site-planner.git
cd site-planner

# Set your API keys
export GOOGLE_API_KEY=your-google-key    # Required: Geocoding + Static Maps
export OPENAI_API_KEY=your-openai-key    # Optional: AI structure detection
export SAM_API_URL=http://localhost:8000  # Optional: SAM segmentation endpoint

# Run
go run ./cmd/server
# → http://localhost:8080
```

### Demo Mode

Run without any API keys using hardcoded stubs for "12 King William St, Kent Town":

```bash
export DEMO_MODE=true
go run ./cmd/server
# → http://localhost:8080 (no API keys needed)
```

Demo mode provides a generated placeholder satellite image, realistic cadastral parcel boundary, and sample AI structure detections (house, shed, driveway, fence, garden bed) so you can test the full PDF generation pipeline offline.

## Requirements

- Go 1.22+
- Google Maps API key with:
  - Geocoding API enabled
  - Maps Static API enabled
- *(Optional)* OpenAI API key with GPT-4o access — for AI structure detection
- *(Optional)* SAM segmentation service — for precise polygon outlines

## Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `GOOGLE_API_KEY` | Yes | — | Google Maps Geocoding + Static Maps |
| `OPENAI_API_KEY` | No | — | OpenAI GPT-4o vision for structure detection |
| `SAM_API_URL` | No | — | Segment Anything Model API endpoint |
| `PORT` | No | `8080` | HTTP server port |
| `DEMO_MODE` | No | `false` | Set to `true` or `1` for offline demo with hardcoded stubs |

## Architecture

```
cmd/server/main.go          → HTTP entrypoint, graceful shutdown
internal/config/             → Environment variable loading
internal/models/             → Shared types (Parcel, SitePlan, Detection, ProposedBuilding)
internal/geocode/            → Google Geocoding API client
internal/cadastre/           → SA ArcGIS REST cadastre provider + Provider interface
internal/imagery/            → Google Static Maps satellite image + coordinate transforms
internal/detect/             → AI structure detection (OpenAI GPT-4o + SAM)
internal/demo/               → Demo mode stubs (hardcoded geocode, cadastre, imagery, detect)
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
5. *(Optional)* AI detection: satellite image is sent to GPT-4o vision to identify existing structures (sheds, carports, driveways, pools, etc.) with bounding boxes, then optionally refined with SAM segmentation for precise polygon outlines
6. *(Optional)* User-specified proposed buildings are positioned on the plan
7. An A3 landscape PDF is composed with all overlays, dimensions, and annotations
8. PDF is served for download (auto-cleaned after 1 hour)

## AI Detection Details

When `OPENAI_API_KEY` is configured, the UI shows an "AI Structure Detection" toggle. When enabled:

- The satellite image is sent to **GPT-4o** with a structured prompt requesting JSON bounding boxes for each identified structure
- If `SAM_API_URL` is configured, bounding boxes are sent to a **Segment Anything Model** endpoint for precise polygon segmentation
- Detected structures are drawn on the PDF with color-coded overlays and a legend
- Detection is non-blocking — if the AI call fails, the plan is still generated without detection overlays

### SAM API Contract

The SAM endpoint should accept `POST /segment` with:
```json
{
  "image": "<base64 PNG>",
  "width": 1280, "height": 1280,
  "boxes": [[x, y, w, h], ...],
  "labels": ["shed", "carport", ...]
}
```
And return:
```json
{
  "segments": [
    {"label": "shed", "polygon": [[x1,y1], [x2,y2], ...]},
    ...
  ]
}
```

## Data Sources

- **Cadastre**: [Location SA](https://location.sa.gov.au) — Government of South Australia (ArcGIS REST, layer 124)
- **Imagery**: Google Maps Static API
- **Geocoding**: Google Maps Geocoding API
- **AI Detection**: OpenAI GPT-4o Vision API
- **Segmentation**: Segment Anything Model (SAM) — user-hosted

## Build

```bash
go build ./...          # Build all packages
go vet ./...            # Static analysis
go test ./...           # Run tests
```

## License

MIT
