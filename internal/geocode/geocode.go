package geocode

import (
	"context"
	"fmt"

	"github.com/whatnick/site-planner/internal/models"
	"googlemaps.github.io/maps"
)

// Geocoder is the interface for address geocoding.
type Geocoder interface {
	Lookup(ctx context.Context, address string) (*models.Coord, string, error)
}

type Client struct {
	mapsClient *maps.Client
	apiKey     string
}

func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return &Client{apiKey: apiKey}, nil
	}
	c, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("creating maps client: %w", err)
	}
	return &Client{mapsClient: c, apiKey: apiKey}, nil
}

// Lookup geocodes an Australian address and returns the coordinate and formatted address.
func (c *Client) Lookup(ctx context.Context, address string) (*models.Coord, string, error) {
	if c.mapsClient == nil {
		return nil, "", fmt.Errorf("GOOGLE_API_KEY not configured — set it and restart the server")
	}
	req := &maps.GeocodingRequest{
		Address: address,
		Components: map[maps.Component]string{
			maps.ComponentCountry: "AU",
		},
	}

	results, err := c.mapsClient.Geocode(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("geocoding request: %w", err)
	}
	if len(results) == 0 {
		return nil, "", fmt.Errorf("no results found for address: %s", address)
	}

	result := results[0]
	loc := result.Geometry.Location

	coord := &models.Coord{
		Lat: loc.Lat,
		Lng: loc.Lng,
	}

	return coord, result.FormattedAddress, nil
}
