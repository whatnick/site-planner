package models

import "github.com/paulmach/orb"

// Coord represents a WGS84 coordinate.
type Coord struct {
	Lat float64
	Lng float64
}

// ParcelEdge represents one edge of a parcel boundary with its length.
type ParcelEdge struct {
	Start  Coord
	End    Coord
	Length float64 // meters
}

// Parcel represents a cadastral land parcel.
type Parcel struct {
	ID          string
	PlanParcel  string
	Address     string
	Polygon     orb.Polygon
	BoundingBox orb.Bound
	Edges       []ParcelEdge
	Area        float64 // square meters
}

// ProposedBuilding represents a building the user wants to add to the site plan.
type ProposedBuilding struct {
	Label  string  // e.g. "Proposed Dwelling", "New Garage"
	Width  float64 // meters
	Height float64 // meters (depth)
	X      float64 // position as fraction of image width (0.0–1.0)
	Y      float64 // position as fraction of image height (0.0–1.0)
}

// SitePlan holds all data needed to render a PDF site plan.
type SitePlan struct {
	Address          string
	FormattedAddress string
	Parcel           *Parcel
	SatelliteImage   []byte // PNG data
	MapCenterLat     float64
	MapCenterLng     float64
	ZoomLevel        int
	ScaleRatio       string // e.g. "1:500"
	MetersPerPixel   float64

	// AI-detected structures (optional)
	Detections []Detection

	// User-specified proposed buildings (optional)
	ProposedBuildings []ProposedBuilding
}

// Detection represents a structure identified in the satellite image by AI.
type Detection struct {
	Label      string
	Confidence float64
	BBoxPixels [4]int   // [x, y, width, height] in image pixels
	Polygon    [][2]int // precise outline in image pixels (from SAM, may be nil)
}
