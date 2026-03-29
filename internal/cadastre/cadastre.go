package cadastre

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/whatnick/site-planner/internal/models"
)

// Provider defines the interface for cadastre data lookup.
// Implement this for each Australian state.
type Provider interface {
	LookupParcel(ctx context.Context, coord models.Coord) (*models.Parcel, error)
}

// SAProvider queries the SA government ArcGIS REST endpoint for cadastral parcels.
type SAProvider struct {
	httpClient *http.Client
	baseURL    string
}

func NewSAProvider() *SAProvider {
	return &SAProvider{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://lsa4.geohub.sa.gov.au/server/rest/services/LSA/LocationSAViewerV34/MapServer/124/query",
	}
}

func (p *SAProvider) LookupParcel(ctx context.Context, coord models.Coord) (*models.Parcel, error) {
	params := url.Values{
		"geometry":       {fmt.Sprintf("%f,%f", coord.Lng, coord.Lat)},
		"geometryType":   {"esriGeometryPoint"},
		"spatialRel":     {"esriSpatialRelIntersects"},
		"outFields":      {"parcel_id,plan,parcel,planparcel"},
		"returnGeometry": {"true"},
		"outSR":          {"4326"},
		"f":              {"geojson"},
	}

	reqURL := p.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying SA cadastre: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SA cadastre returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	fc, err := geojson.UnmarshalFeatureCollection(body)
	if err != nil {
		return nil, fmt.Errorf("parsing GeoJSON: %w", err)
	}

	if len(fc.Features) == 0 {
		return nil, fmt.Errorf("no parcel found at coordinates (%f, %f)", coord.Lat, coord.Lng)
	}

	feature := fc.Features[0]
	polygon, ok := feature.Geometry.(orb.Polygon)
	if !ok {
		// Try MultiPolygon — take the first one
		if mp, ok := feature.Geometry.(orb.MultiPolygon); ok && len(mp) > 0 {
			polygon = mp[0]
		} else {
			return nil, fmt.Errorf("unexpected geometry type: %T", feature.Geometry)
		}
	}

	parcelID := safeString(feature.Properties, "parcel_id")
	planParcel := safeString(feature.Properties, "planparcel")

	parcel := &models.Parcel{
		ID:          parcelID,
		PlanParcel:  planParcel,
		Polygon:     polygon,
		BoundingBox: polygon.Bound(),
	}

	// Compute edges with distances
	if len(polygon) > 0 {
		ring := polygon[0] // outer ring
		for i := 0; i < len(ring)-1; i++ {
			start := models.Coord{Lat: ring[i][1], Lng: ring[i][0]}
			end := models.Coord{Lat: ring[i+1][1], Lng: ring[i+1][0]}
			length := haversineDistance(start, end)
			parcel.Edges = append(parcel.Edges, models.ParcelEdge{
				Start:  start,
				End:    end,
				Length: length,
			})
		}
		parcel.Area = computeArea(ring)
	}

	return parcel, nil
}

func safeString(props map[string]interface{}, key string) string {
	if v, ok := props[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// haversineDistance computes distance in meters between two WGS84 coordinates.
func haversineDistance(a, b models.Coord) float64 {
	const R = 6371000 // Earth radius in meters
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return R * c
}

// computeArea calculates the approximate area in sq meters using the Shoelace formula
// on projected coordinates (simple equirectangular approximation).
func computeArea(ring orb.Ring) float64 {
	if len(ring) < 3 {
		return 0
	}
	// Use mid-latitude for equirectangular projection
	var sumLat float64
	for _, p := range ring {
		sumLat += p[1]
	}
	midLat := sumLat / float64(len(ring))
	cosLat := math.Cos(midLat * math.Pi / 180)

	// Convert to approximate meters
	const mPerDeg = 111319.9 // meters per degree at equator

	var area float64
	n := len(ring)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		xi := ring[i][0] * cosLat * mPerDeg
		yi := ring[i][1] * mPerDeg
		xj := ring[j][0] * cosLat * mPerDeg
		yj := ring[j][1] * mPerDeg
		area += xi*yj - xj*yi
	}
	return math.Abs(area) / 2
}
