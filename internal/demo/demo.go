package demo

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/paulmach/orb"
	"github.com/whatnick/site-planner/internal/detect"
	"github.com/whatnick/site-planner/internal/models"
)

// Demo data for "12 King William St, Kent Town SA 5067"
const (
	demoLat           = -34.9211
	demoLng           = 138.6243
	demoFormattedAddr = "12 King William St, Kent Town SA 5067, Australia"
	demoPlanParcel    = "D12345/A100"
	demoParcelID      = "54321"
	demoZoom          = 20
	demoImgSize       = 1280
)

// -----------------------------------------------------------------------
// Geocoder stub
// -----------------------------------------------------------------------

// Geocoder returns hardcoded coordinates for any address (demo mode).
type Geocoder struct{}

func (g *Geocoder) Lookup(_ context.Context, _ string) (*models.Coord, string, error) {
	return &models.Coord{Lat: demoLat, Lng: demoLng}, demoFormattedAddr, nil
}

// -----------------------------------------------------------------------
// Cadastre stub
// -----------------------------------------------------------------------

// Cadastre returns a hardcoded parcel polygon for Kent Town.
type Cadastre struct{}

func (c *Cadastre) LookupParcel(_ context.Context, _ models.Coord) (*models.Parcel, error) {
	// Realistic residential lot ~27 m × 56 m in Kent Town
	polygon := orb.Polygon{
		orb.Ring{
			orb.Point{138.62415, -34.92135}, // SW
			orb.Point{138.62445, -34.92135}, // SE
			orb.Point{138.62445, -34.92085}, // NE
			orb.Point{138.62415, -34.92085}, // NW
			orb.Point{138.62415, -34.92135}, // close
		},
	}

	ring := polygon[0]
	edges := make([]models.ParcelEdge, 0, len(ring)-1)
	for i := 0; i < len(ring)-1; i++ {
		start := models.Coord{Lat: ring[i][1], Lng: ring[i][0]}
		end := models.Coord{Lat: ring[i+1][1], Lng: ring[i+1][0]}
		edges = append(edges, models.ParcelEdge{
			Start:  start,
			End:    end,
			Length: haversine(start, end),
		})
	}

	return &models.Parcel{
		ID:          demoParcelID,
		PlanParcel:  demoPlanParcel,
		Address:     demoFormattedAddr,
		Polygon:     polygon,
		BoundingBox: polygon.Bound(),
		Edges:       edges,
		Area:        1526.0, // ~27 × 56 m
	}, nil
}

// -----------------------------------------------------------------------
// Imager stub
// -----------------------------------------------------------------------

// Imager returns a generated placeholder satellite PNG.
type Imager struct{}

func (im *Imager) FetchSatelliteImage(_ context.Context, _ orb.Bound) ([]byte, int, float64, error) {
	imgData := generatePlaceholderPNG()
	// metersPerPixel at zoom 20, lat ≈ -34.92
	mpp := 156543.03392 * math.Cos(demoLat*math.Pi/180) / math.Pow(2, float64(demoZoom))
	return imgData, demoZoom, mpp, nil
}

// -----------------------------------------------------------------------
// Detector stub
// -----------------------------------------------------------------------

// Detector returns hardcoded structure detections.
type Detector struct{}

func (d *Detector) Available() bool { return true }

func (d *Detector) DetectStructures(_ context.Context, _ []byte, _, _ int) ([]detect.Detection, error) {
	return []detect.Detection{
		{Label: "house", Confidence: 0.95, BBoxPixels: [4]int{480, 420, 280, 180}},
		{Label: "shed", Confidence: 0.82, BBoxPixels: [4]int{820, 280, 120, 100}},
		{Label: "driveway", Confidence: 0.88, BBoxPixels: [4]int{560, 620, 80, 280}},
		{Label: "fence", Confidence: 0.75, BBoxPixels: [4]int{350, 250, 580, 10}},
		{Label: "garden_bed", Confidence: 0.70, BBoxPixels: [4]int{380, 800, 200, 140}},
	}, nil
}

// -----------------------------------------------------------------------
// Placeholder image generation
// -----------------------------------------------------------------------

func generatePlaceholderPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, demoImgSize, demoImgSize))

	// Base grass colour with subtle variation
	for y := 0; y < demoImgSize; y++ {
		for x := 0; x < demoImgSize; x++ {
			g := uint8(90 + (x+y)%30) // 90-119 green variation
			img.SetRGBA(x, y, color.RGBA{R: 72, G: g, B: 45, A: 255})
		}
	}

	// House (brown roof)
	fillRect(img, 480, 420, 280, 180, color.RGBA{R: 139, G: 90, B: 43, A: 255})

	// Shed (lighter brown)
	fillRect(img, 820, 280, 120, 100, color.RGBA{R: 170, G: 130, B: 70, A: 255})

	// Driveway (gray)
	fillRect(img, 560, 620, 80, 280, color.RGBA{R: 140, G: 140, B: 140, A: 255})

	// Fence line (dark)
	fillRect(img, 350, 250, 580, 6, color.RGBA{R: 80, G: 60, B: 40, A: 255})

	// Garden bed (dark green)
	fillRect(img, 380, 800, 200, 140, color.RGBA{R: 40, G: 100, B: 30, A: 255})

	// Footpath along bottom edge
	fillRect(img, 200, 1150, 900, 30, color.RGBA{R: 160, G: 160, B: 155, A: 255})

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func fillRect(img *image.RGBA, x0, y0, w, h int, c color.RGBA) {
	for y := y0; y < y0+h && y < demoImgSize; y++ {
		for x := x0; x < x0+w && x < demoImgSize; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func haversine(a, b models.Coord) float64 {
	const R = 6371000
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return R * c
}
