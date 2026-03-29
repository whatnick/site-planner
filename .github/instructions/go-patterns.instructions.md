---
applyTo: "**/*.go"
description: "Go coding patterns for the site-planner project. Use when: writing or modifying Go files, adding new packages, implementing providers, or handling errors."
---

# Go Patterns

## Error Handling

Always wrap errors with context describing the operation:
```go
return nil, fmt.Errorf("querying SA cadastre: %w", err)
```

## HTTP Handlers

- Handlers use `func(w http.ResponseWriter, r *http.Request)` signature
- Register with `mux.HandleFunc("METHOD /path", handler)` (Go 1.22+ routing)
- HTMX endpoints return HTML partials via `templates.ExecuteTemplate(w, "name.html", data)`
- Use `r.PathValue("param")` for path parameters

## External API Clients

- Accept `context.Context` as first parameter
- Use dedicated `http.Client` with explicit timeouts (30s typical)
- Construct requests with `http.NewRequestWithContext`
- Close response bodies with `defer resp.Body.Close()`

## Adding a New State Cadastre Provider

1. Create `internal/cadastre/<state>.go`
2. Implement `cadastre.Provider` interface: `LookupParcel(ctx, coord) (*models.Parcel, error)`
3. Wire it up in `cmd/server/main.go` based on geocoded state detection

## Spatial Data

- GeoJSON parsing via `github.com/paulmach/orb/geojson`
- Polygons are `orb.Polygon` (outer ring at index 0)
- Bounding box via `polygon.Bound()`
- Distances in meters via Haversine formula (see `cadastre.haversineDistance`)
- Coordinate order in orb: `[longitude, latitude]` (GeoJSON standard)

## PDF Generation

- Use `gofpdf` methods: `Rect`, `Line`, `Polygon`, `ImageOptions`, `CellFormat`, `SetAlpha`
- Coordinate system: mm from top-left of page
- Convert lat/lng to PDF mm via `imagery.LatLngToPixel` → pixel → `mmPerPixel` scaling
