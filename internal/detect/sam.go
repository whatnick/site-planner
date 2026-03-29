package detect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// samRequest is the payload sent to a SAM API endpoint.
type samRequest struct {
	Image      string   `json:"image"`       // base64 PNG
	Width      int      `json:"width"`       // image width
	Height     int      `json:"height"`      // image height
	Boxes      [][4]int `json:"boxes"`       // bounding boxes [x, y, w, h]
	Labels     []string `json:"labels"`      // corresponding labels
}

// samResponse is the response from a SAM API endpoint.
type samResponse struct {
	Segments []struct {
		Label   string   `json:"label"`
		Polygon [][2]int `json:"polygon"` // polygon points in image pixel coords
	} `json:"segments"`
}

// samRefine calls the SAM segmentation endpoint to get precise polygon outlines
// for each detection's bounding box.
//
// The SAM API is expected to accept a POST with JSON body containing:
//   - "image": base64-encoded PNG
//   - "width", "height": image dimensions
//   - "boxes": array of [x, y, w, h] bounding boxes
//   - "labels": array of label strings
//
// And return JSON with a "segments" array containing polygon outlines.
func (c *Client) samRefine(ctx context.Context, imgPNG []byte, imgWidth, imgHeight int, detections []Detection) ([]Detection, error) {
	boxes := make([][4]int, len(detections))
	labels := make([]string, len(detections))
	for i, d := range detections {
		boxes[i] = d.BBoxPixels
		labels[i] = d.Label
	}

	reqBody := samRequest{
		Image:  base64.StdEncoding.EncodeToString(imgPNG),
		Width:  imgWidth,
		Height: imgHeight,
		Boxes:  boxes,
		Labels: labels,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling SAM request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.samURL+"/segment", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("creating SAM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling SAM: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading SAM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SAM returned status %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var samResp samResponse
	if err := json.Unmarshal(respBody, &samResp); err != nil {
		return nil, fmt.Errorf("parsing SAM response: %w", err)
	}

	// Merge SAM polygons back into detections
	result := make([]Detection, len(detections))
	copy(result, detections)
	for i := range result {
		if i < len(samResp.Segments) {
			result[i].Polygon = samResp.Segments[i].Polygon
		}
	}

	return result, nil
}
