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
}
