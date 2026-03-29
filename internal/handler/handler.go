package handler

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/whatnick/site-planner/internal/cadastre"
	"github.com/whatnick/site-planner/internal/detect"
	"github.com/whatnick/site-planner/internal/geocode"
	"github.com/whatnick/site-planner/internal/imagery"
	"github.com/whatnick/site-planner/internal/models"
	"github.com/whatnick/site-planner/internal/planner"
)

type Handler struct {
	geocoder   geocode.Geocoder
	cadastrePr cadastre.Provider
	imager     imagery.Imager
	detector   detect.Detector
	templates  *template.Template
	pdfDir     string
	mu         sync.Mutex
	pdfFiles   map[string]pdfRecord
}

type pdfRecord struct {
	path      string
	createdAt time.Time
}

func New(geocoder geocode.Geocoder, cadastrePr cadastre.Provider, imager imagery.Imager, detector detect.Detector, templateDir string) (*Handler, error) {
	tmpl, err := template.ParseGlob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	pdfDir := filepath.Join(os.TempDir(), "site-planner-pdfs")
	if err := os.MkdirAll(pdfDir, 0750); err != nil {
		return nil, fmt.Errorf("creating PDF directory: %w", err)
	}

	h := &Handler{
		geocoder:   geocoder,
		cadastrePr: cadastrePr,
		imager:     imager,
		detector:   detector,
		templates:  tmpl,
		pdfDir:     pdfDir,
		pdfFiles:   make(map[string]pdfRecord),
	}

	go h.cleanupLoop()

	return h, nil
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.handleHome)
	mux.HandleFunc("POST /generate", h.handleGenerate)
	mux.HandleFunc("GET /download/{id}", h.handleDownload)
}

func (h *Handler) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := map[string]interface{}{
		"DetectionAvailable": h.detector.Available(),
	}
	if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("ERROR rendering index: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.FormValue("address"))
	if address == "" {
		h.renderResult(w, nil, fmt.Errorf("please enter an address"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Step 1: Geocode address
	coord, formattedAddr, err := h.geocoder.Lookup(ctx, address)
	if err != nil {
		h.renderResult(w, nil, fmt.Errorf("geocoding failed: %w", err))
		return
	}
	log.Printf("Geocoded: %s → (%.6f, %.6f)", formattedAddr, coord.Lat, coord.Lng)

	// Step 2: Lookup cadastre parcel
	parcel, err := h.cadastrePr.LookupParcel(ctx, *coord)
	if err != nil {
		h.renderResult(w, nil, fmt.Errorf("cadastre lookup failed: %w", err))
		return
	}
	parcel.Address = formattedAddr
	log.Printf("Found parcel: %s (%.0f m²)", parcel.PlanParcel, parcel.Area)

	// Step 3: Fetch satellite image
	imgData, zoom, mpp, err := h.imager.FetchSatelliteImage(ctx, parcel.BoundingBox)
	if err != nil {
		h.renderResult(w, nil, fmt.Errorf("imagery fetch failed: %w", err))
		return
	}
	log.Printf("Satellite image: zoom=%d, meters/pixel=%.3f", zoom, mpp)

	// Step 4: Optionally detect structures via AI
	enableDetection := r.FormValue("detect") == "on"
	var detections []models.Detection
	if enableDetection && h.detector.Available() {
		dets, err := h.detector.DetectStructures(ctx, imgData, 1280, 1280)
		if err != nil {
			log.Printf("WARNING: AI detection failed: %v", err)
			// Non-fatal — continue without detections
		} else {
			for _, d := range dets {
				detections = append(detections, models.Detection{
					Label:      d.Label,
					Confidence: d.Confidence,
					BBoxPixels: d.BBoxPixels,
					Polygon:    d.Polygon,
				})
			}
			log.Printf("Detected %d structures via AI", len(detections))
		}
	}

	// Step 5: Parse proposed buildings from form
	proposedBuildings := parseProposedBuildings(r)

	// Step 6: Build site plan model
	plan := &models.SitePlan{
		Address:           address,
		FormattedAddress:  formattedAddr,
		Parcel:            parcel,
		SatelliteImage:    imgData,
		MapCenterLat:      parcel.BoundingBox.Center()[1],
		MapCenterLng:      parcel.BoundingBox.Center()[0],
		ZoomLevel:         zoom,
		MetersPerPixel:    mpp,
		Detections:        detections,
		ProposedBuildings: proposedBuildings,
	}

	// Step 7: Generate PDF
	pdfData, err := planner.GeneratePDF(plan)
	if err != nil {
		h.renderResult(w, nil, fmt.Errorf("PDF generation failed: %w", err))
		return
	}

	// Step 8: Save PDF and return download link
	id := uuid.New().String()
	pdfPath := filepath.Join(h.pdfDir, id+".pdf")
	if err := os.WriteFile(pdfPath, pdfData, 0600); err != nil {
		h.renderResult(w, nil, fmt.Errorf("saving PDF failed: %w", err))
		return
	}

	h.mu.Lock()
	h.pdfFiles[id] = pdfRecord{path: pdfPath, createdAt: time.Now()}
	h.mu.Unlock()

	log.Printf("Generated PDF: %s (%d bytes)", id, len(pdfData))

	data := map[string]interface{}{
		"Address":        formattedAddr,
		"ParcelID":       parcel.PlanParcel,
		"Area":           fmt.Sprintf("%.0f", parcel.Area),
		"EdgeCount":      len(parcel.Edges),
		"DownloadID":     id,
		"DetectionCount": len(detections),
		"ProposedCount":  len(proposedBuildings),
	}
	h.renderResult(w, data, nil)
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	h.mu.Lock()
	rec, ok := h.pdfFiles[id]
	h.mu.Unlock()

	if !ok {
		http.Error(w, "PDF not found or expired", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="site-plan-%s.pdf"`, id[:8]))
	http.ServeFile(w, r, rec.path)
}

func (h *Handler) renderResult(w http.ResponseWriter, data map[string]interface{}, err error) {
	tplData := map[string]interface{}{}
	if data != nil {
		tplData = data
	}
	if err != nil {
		tplData["Error"] = err.Error()
	}

	if execErr := h.templates.ExecuteTemplate(w, "result.html", tplData); execErr != nil {
		log.Printf("ERROR rendering result: %v", execErr)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// cleanupLoop removes PDFs older than 1 hour.
func (h *Handler) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.Lock()
		for id, rec := range h.pdfFiles {
			if time.Since(rec.createdAt) > time.Hour {
				os.Remove(rec.path)
				delete(h.pdfFiles, id)
				log.Printf("Cleaned up PDF: %s", id)
			}
		}
		h.mu.Unlock()
	}
}

// parseProposedBuildings extracts proposed building entries from form fields.
// Expected form fields: bldg_label[], bldg_width[], bldg_height[], bldg_x[], bldg_y[]
func parseProposedBuildings(r *http.Request) []models.ProposedBuilding {
	labels := r.Form["bldg_label[]"]
	widths := r.Form["bldg_width[]"]
	heights := r.Form["bldg_height[]"]
	xs := r.Form["bldg_x[]"]
	ys := r.Form["bldg_y[]"]

	n := len(labels)
	if len(widths) < n {
		n = len(widths)
	}
	if len(heights) < n {
		n = len(heights)
	}

	var buildings []models.ProposedBuilding
	for i := 0; i < n; i++ {
		label := strings.TrimSpace(labels[i])
		if label == "" {
			continue
		}

		w, err := strconv.ParseFloat(strings.TrimSpace(widths[i]), 64)
		if err != nil || w <= 0 || w > 200 {
			continue
		}
		h, err := strconv.ParseFloat(strings.TrimSpace(heights[i]), 64)
		if err != nil || h <= 0 || h > 200 {
			continue
		}

		// Default position: center of image
		posX := 0.5
		posY := 0.5
		if i < len(xs) {
			if v, err := strconv.ParseFloat(strings.TrimSpace(xs[i]), 64); err == nil && v >= 0 && v <= 1 {
				posX = v
			}
		}
		if i < len(ys) {
			if v, err := strconv.ParseFloat(strings.TrimSpace(ys[i]), 64); err == nil && v >= 0 && v <= 1 {
				posY = v
			}
		}

		buildings = append(buildings, models.ProposedBuilding{
			Label:  label,
			Width:  w,
			Height: h,
			X:      posX,
			Y:      posY,
		})
	}

	return buildings
}
