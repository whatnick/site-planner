package imagery

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/paulmach/orb"
)

const (
	staticMapsURL = "https://maps.googleapis.com/maps/api/staticmap"
	imageSize     = 640 // max free tier
	imageScale    = 2   // gives 1280x1280 actual pixels
)

// Imager is the interface for satellite image fetching.
type Imager interface {
	FetchSatelliteImage(ctx context.Context, bbox orb.Bound) ([]byte, int, float64, error)
}

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchSatelliteImage fetches a satellite image centered on the parcel bounding box.
// Returns the PNG image bytes, the zoom level used, and meters-per-pixel.
func (c *Client) FetchSatelliteImage(ctx context.Context, bbox orb.Bound) ([]byte, int, float64, error) {
	center := bbox.Center()
	zoom := calculateZoom(bbox, imageSize*imageScale)

	params := url.Values{
		"center":  {fmt.Sprintf("%f,%f", center[1], center[0])},
		"zoom":    {fmt.Sprintf("%d", zoom)},
		"size":    {fmt.Sprintf("%dx%d", imageSize, imageSize)},
		"scale":   {fmt.Sprintf("%d", imageScale)},
		"maptype": {"satellite"},
		"key":     {c.apiKey},
	}

	reqURL := staticMapsURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("fetching satellite image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, 0, fmt.Errorf("static maps returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("reading image: %w", err)
	}

	mpp := metersPerPixel(center[1], zoom)

	return data, zoom, mpp, nil
}

// calculateZoom determines the best zoom level to fit the bounding box
// within the given pixel dimensions with ~20% padding.
func calculateZoom(bbox orb.Bound, pixelSize int) int {
	latSpan := bbox.Max[1] - bbox.Min[1]
	lngSpan := bbox.Max[0] - bbox.Min[0]

	// Add 40% padding (20% each side)
	latSpan *= 1.4
	lngSpan *= 1.4

	for zoom := 21; zoom >= 1; zoom-- {
		// At this zoom, how many degrees fit in the image?
		// 256 pixels at zoom 0 = 360 degrees of longitude
		worldSize := 256.0 * math.Pow(2, float64(zoom))
		degreesPerPixel := 360.0 / worldSize
		lngFit := degreesPerPixel * float64(pixelSize)

		// Latitude is affected by Mercator projection
		centerLat := (bbox.Max[1] + bbox.Min[1]) / 2
		latFit := lngFit * math.Cos(centerLat*math.Pi/180)

		if lngFit >= lngSpan && latFit >= latSpan {
			return zoom
		}
	}
	return 1
}

// metersPerPixel returns the ground resolution at a given latitude and zoom level.
func metersPerPixel(lat float64, zoom int) float64 {
	return 156543.03392 * math.Cos(lat*math.Pi/180) / math.Pow(2, float64(zoom))
}

// LatLngToPixel converts a lat/lng to pixel coordinates relative to the map image center.
// Returns (x, y) in pixels where (0, 0) is the top-left of the image.
func LatLngToPixel(lat, lng, centerLat, centerLng float64, zoom int, imgWidth, imgHeight int) (float64, float64) {
	worldSize := 256.0 * math.Pow(2, float64(zoom))

	// Convert to world pixel coordinates
	centerX := (centerLng + 180) / 360 * worldSize
	centerY := (1 - math.Log(math.Tan(centerLat*math.Pi/180)+1/math.Cos(centerLat*math.Pi/180))/(math.Pi)) / 2 * worldSize

	pointX := (lng + 180) / 360 * worldSize
	pointY := (1 - math.Log(math.Tan(lat*math.Pi/180)+1/math.Cos(lat*math.Pi/180))/(math.Pi)) / 2 * worldSize

	// Offset from center of image
	px := float64(imgWidth)/2 + (pointX - centerX)
	py := float64(imgHeight)/2 + (pointY - centerY)

	return px, py
}
