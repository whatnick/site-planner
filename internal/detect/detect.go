package detect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Detection represents a structure identified in a satellite image.
type Detection struct {
	Label      string   `json:"label"`      // e.g. "shed", "driveway", "carport", "pool"
	Confidence float64  `json:"confidence"` // 0.0–1.0
	BBoxPixels [4]int   `json:"bbox"`       // [x, y, width, height] in image pixels
	Polygon    [][2]int `json:"polygon"`    // polygon points in image pixels (from SAM)
}

// Client calls an LLM vision API to detect structures and optionally
// a SAM segmentation endpoint for precise polygon outlines.
type Client struct {
	openAIKey  string
	samURL     string // optional SAM API endpoint
	httpClient *http.Client
}

// NewClient creates a detection client. openAIKey is required for LLM vision.
// samURL is optional — if empty, only bounding boxes are returned (no polygons).
func NewClient(openAIKey, samURL string) *Client {
	return &Client{
		openAIKey:  openAIKey,
		samURL:     samURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Available reports whether the detection client has the necessary config.
func (c *Client) Available() bool {
	return c.openAIKey != ""
}

// DetectStructures sends the satellite image to GPT-4o vision for structure
// detection, then optionally refines with SAM segmentation.
func (c *Client) DetectStructures(ctx context.Context, imgPNG []byte, imgWidth, imgHeight int) ([]Detection, error) {
	if c.openAIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not configured")
	}

	detections, err := c.llmDetect(ctx, imgPNG, imgWidth, imgHeight)
	if err != nil {
		return nil, fmt.Errorf("LLM detection: %w", err)
	}

	// If SAM endpoint is available, refine each detection with precise polygons
	if c.samURL != "" && len(detections) > 0 {
		refined, err := c.samRefine(ctx, imgPNG, imgWidth, imgHeight, detections)
		if err != nil {
			// SAM failure is non-fatal — fall back to bounding boxes
			return detections, nil
		}
		return refined, nil
	}

	return detections, nil
}

// llmDetect calls OpenAI GPT-4o vision to identify structures in the image.
func (c *Client) llmDetect(ctx context.Context, imgPNG []byte, imgWidth, imgHeight int) ([]Detection, error) {
	b64Img := base64.StdEncoding.EncodeToString(imgPNG)

	prompt := fmt.Sprintf(`Analyze this satellite/aerial image of a property (%dx%d pixels).
Identify all visible structures and features. For each, return a JSON object with:
- "label": one of "house", "shed", "garage", "carport", "driveway", "pool", "deck", "pergola", "outbuilding", "retaining_wall", "fence", "garden_bed", "paved_area", "other"
- "confidence": a float 0.0-1.0
- "bbox": [x, y, width, height] in image pixel coordinates (top-left origin)

Return ONLY a JSON array of objects. No markdown, no explanation. Example:
[{"label":"house","confidence":0.95,"bbox":[400,350,280,200]}]

If no structures are visible, return an empty array: []`, imgWidth, imgHeight)

	reqBody := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": prompt,
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url":    "data:image/png;base64," + b64Img,
							"detail": "high",
						},
					},
				},
			},
		},
		"max_tokens":  2048,
		"temperature": 0.1,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.openAIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling OpenAI: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI returned status %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	// Parse the OpenAI response
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("parsing OpenAI response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	content := openAIResp.Choices[0].Message.Content

	// Strip markdown code fences if present
	content = stripCodeFences(content)

	var detections []Detection
	if err := json.Unmarshal([]byte(content), &detections); err != nil {
		return nil, fmt.Errorf("parsing detections JSON: %w (content: %s)", err, truncate(content, 200))
	}

	// Clamp bounding boxes to image dimensions
	for i := range detections {
		d := &detections[i]
		if d.BBoxPixels[0] < 0 {
			d.BBoxPixels[0] = 0
		}
		if d.BBoxPixels[1] < 0 {
			d.BBoxPixels[1] = 0
		}
		if d.BBoxPixels[0]+d.BBoxPixels[2] > imgWidth {
			d.BBoxPixels[2] = imgWidth - d.BBoxPixels[0]
		}
		if d.BBoxPixels[1]+d.BBoxPixels[3] > imgHeight {
			d.BBoxPixels[3] = imgHeight - d.BBoxPixels[1]
		}
	}

	return detections, nil
}

// stripCodeFences removes markdown ``` fences from LLM output.
func stripCodeFences(s string) string {
	// Remove leading ```json or ``` and trailing ```
	out := s
	for len(out) > 0 && out[0] == '`' {
		out = out[1:]
	}
	if len(out) > 4 && out[:4] == "json" {
		out = out[4:]
	}
	for len(out) > 0 && out[len(out)-1] == '`' {
		out = out[:len(out)-1]
	}
	return out
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
