package planner

import (
	"bytes"
	"fmt"
	"math"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/whatnick/site-planner/internal/imagery"
	"github.com/whatnick/site-planner/internal/models"
)

const (
	// A3 landscape dimensions in mm
	pageWidth  = 420.0
	pageHeight = 297.0

	// Map area margins
	marginLeft   = 15.0
	marginRight  = 15.0
	marginTop    = 40.0
	marginBottom = 25.0

	// Actual pixel size of the satellite image (640 * scale 2)
	imgPixelSize = 1280
)

// GeneratePDF creates an A3 landscape site plan PDF and returns the bytes.
func GeneratePDF(plan *models.SitePlan) ([]byte, error) {
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "L",
		UnitStr:        "mm",
		SizeStr:        "A3",
	})
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	// Title block
	drawTitleBlock(pdf, plan)

	// Map area
	mapX := marginLeft
	mapY := marginTop
	mapW := pageWidth - marginLeft - marginRight
	mapH := pageHeight - marginTop - marginBottom

	// Draw satellite image
	drawSatelliteImage(pdf, plan, mapX, mapY, mapW, mapH)

	// Draw parcel boundary overlay
	drawParcelBoundary(pdf, plan, mapX, mapY, mapW, mapH)

	// Draw dimension annotations
	drawDimensions(pdf, plan, mapX, mapY, mapW, mapH)

	// North arrow
	drawNorthArrow(pdf, mapX+mapW-20, mapY+10)

	// Scale bar
	drawScaleBar(pdf, plan, mapX+10, mapY+mapH-15)

	// Footer / disclaimer
	drawFooter(pdf, plan)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("generating PDF: %w", err)
	}

	return buf.Bytes(), nil
}

func drawTitleBlock(pdf *gofpdf.Fpdf, plan *models.SitePlan) {
	// Background bar
	pdf.SetFillColor(33, 37, 41)
	pdf.Rect(0, 0, pageWidth, 35, "F")

	// Title
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetXY(marginLeft, 8)
	pdf.CellFormat(200, 10, "SITE PLAN", "", 0, "L", false, 0, "")

	// Address
	pdf.SetFont("Helvetica", "", 12)
	pdf.SetXY(marginLeft, 20)
	pdf.CellFormat(300, 8, plan.FormattedAddress, "", 0, "L", false, 0, "")

	// Parcel info (right side)
	pdf.SetFont("Helvetica", "", 10)
	if plan.Parcel != nil {
		pdf.SetXY(pageWidth-marginRight-150, 8)
		pdf.CellFormat(150, 6, fmt.Sprintf("Parcel: %s", plan.Parcel.PlanParcel), "", 0, "R", false, 0, "")

		pdf.SetXY(pageWidth-marginRight-150, 15)
		pdf.CellFormat(150, 6, fmt.Sprintf("Area: %.0f m\u00B2", plan.Parcel.Area), "", 0, "R", false, 0, "")
	}

	// Date
	pdf.SetXY(pageWidth-marginRight-150, 22)
	pdf.CellFormat(150, 6, fmt.Sprintf("Generated: %s", time.Now().Format("02 Jan 2006")), "", 0, "R", false, 0, "")
}

func drawSatelliteImage(pdf *gofpdf.Fpdf, plan *models.SitePlan, x, y, w, h float64) {
	if len(plan.SatelliteImage) == 0 {
		return
	}

	// Register the PNG image from memory
	reader := bytes.NewReader(plan.SatelliteImage)
	pdf.RegisterImageOptionsReader("satellite", gofpdf.ImageOptions{ImageType: "PNG"}, reader)

	// Draw image scaled to fit the map area while maintaining aspect ratio
	// The satellite image is square (1280x1280), so fit to the smaller dimension
	size := w
	imgX := x
	imgY := y
	if h < w {
		size = h
		imgX = x + (w-h)/2
	} else {
		imgY = y + (h-w)/2
	}

	pdf.ImageOptions("satellite", imgX, imgY, size, size, false,
		gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

func drawParcelBoundary(pdf *gofpdf.Fpdf, plan *models.SitePlan, mapX, mapY, mapW, mapH float64) {
	if plan.Parcel == nil || len(plan.Parcel.Polygon) == 0 {
		return
	}

	ring := plan.Parcel.Polygon[0]
	if len(ring) < 3 {
		return
	}

	// Determine the image drawing area (same logic as drawSatelliteImage)
	size := mapW
	imgX := mapX
	imgY := mapY
	if mapH < mapW {
		size = mapH
		imgX = mapX + (mapW-mapH)/2
	} else {
		imgY = mapY + (mapW-mapH)/2 // corrected for tall maps
	}

	center := plan.Parcel.BoundingBox.Center()

	// Build polygon points in PDF coordinates
	points := make([]gofpdf.PointType, 0, len(ring))
	for _, pt := range ring {
		px, py := imagery.LatLngToPixel(pt[1], pt[0], center[1], center[0], plan.ZoomLevel, imgPixelSize, imgPixelSize)
		// Convert pixel coordinates to mm on the PDF
		mmPerPixel := size / float64(imgPixelSize)
		pdfX := imgX + px*mmPerPixel
		pdfY := imgY + py*mmPerPixel
		points = append(points, gofpdf.PointType{X: pdfX, Y: pdfY})
	}

	// Draw filled boundary with transparency
	pdf.SetAlpha(0.2, "Normal")
	pdf.SetFillColor(255, 0, 0)
	pdf.Polygon(points, "F")
	pdf.SetAlpha(1.0, "Normal")

	// Draw boundary outline
	pdf.SetDrawColor(255, 0, 0)
	pdf.SetLineWidth(0.6)
	pdf.Polygon(points, "D")
}

func drawDimensions(pdf *gofpdf.Fpdf, plan *models.SitePlan, mapX, mapY, mapW, mapH float64) {
	if plan.Parcel == nil || len(plan.Parcel.Edges) == 0 {
		return
	}

	size := mapW
	imgX := mapX
	imgY := mapY
	if mapH < mapW {
		size = mapH
		imgX = mapX + (mapW-mapH)/2
	} else {
		imgY = mapY + (mapW-mapH)/2
	}

	center := plan.Parcel.BoundingBox.Center()
	mmPerPixel := size / float64(imgPixelSize)

	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(255, 255, 0)

	for _, edge := range plan.Parcel.Edges {
		if edge.Length < 0.5 {
			continue // skip tiny edges
		}

		// Midpoint of the edge in PDF coordinates
		midLat := (edge.Start.Lat + edge.End.Lat) / 2
		midLng := (edge.Start.Lng + edge.End.Lng) / 2
		px, py := imagery.LatLngToPixel(midLat, midLng, center[1], center[0], plan.ZoomLevel, imgPixelSize, imgPixelSize)
		pdfX := imgX + px*mmPerPixel
		pdfY := imgY + py*mmPerPixel

		label := fmt.Sprintf("%.1fm", edge.Length)

		// Draw a small background rect for readability
		tw := pdf.GetStringWidth(label) + 2
		pdf.SetFillColor(0, 0, 0)
		pdf.SetAlpha(0.7, "Normal")
		pdf.Rect(pdfX-tw/2, pdfY-3, tw, 5, "F")
		pdf.SetAlpha(1.0, "Normal")

		// Draw text
		pdf.SetXY(pdfX-tw/2+1, pdfY-2.5)
		pdf.CellFormat(tw-2, 4, label, "", 0, "C", false, 0, "")
	}
}

func drawNorthArrow(pdf *gofpdf.Fpdf, x, y float64) {
	// Arrow body
	arrowLen := 15.0

	pdf.SetDrawColor(255, 255, 255)
	pdf.SetLineWidth(0.5)

	// Arrow shaft
	pdf.Line(x, y+arrowLen, x, y)

	// Arrowhead (filled triangle)
	pdf.SetFillColor(255, 255, 255)
	points := []gofpdf.PointType{
		{X: x, Y: y},
		{X: x - 3, Y: y + 5},
		{X: x + 3, Y: y + 5},
	}
	pdf.Polygon(points, "F")

	// "N" label
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(x-3, y-7)
	pdf.CellFormat(6, 6, "N", "", 0, "C", false, 0, "")
}

func drawScaleBar(pdf *gofpdf.Fpdf, plan *models.SitePlan, x, y float64) {
	if plan.MetersPerPixel == 0 {
		return
	}

	// Determine the map area size for mm-per-pixel conversion
	mapW := pageWidth - marginLeft - marginRight
	mapH := pageHeight - marginTop - marginBottom
	size := mapW
	if mapH < mapW {
		size = mapH
	}
	mmPerPixel := size / float64(imgPixelSize)

	// Calculate a nice round distance for the scale bar
	// Target ~60mm bar length
	targetBarMM := 60.0
	targetBarPixels := targetBarMM / mmPerPixel
	targetMeters := targetBarPixels * plan.MetersPerPixel

	// Round to a nice number
	niceMeters := niceRound(targetMeters)
	barPixels := niceMeters / plan.MetersPerPixel
	barMM := barPixels * mmPerPixel

	// Background
	pdf.SetFillColor(0, 0, 0)
	pdf.SetAlpha(0.7, "Normal")
	pdf.Rect(x-2, y-2, barMM+20, 14, "F")
	pdf.SetAlpha(1.0, "Normal")

	// Scale bar
	pdf.SetDrawColor(255, 255, 255)
	pdf.SetFillColor(255, 255, 255)
	pdf.SetLineWidth(0.4)

	barY := y + 4
	pdf.Line(x, barY, x+barMM, barY)
	// End ticks
	pdf.Line(x, barY-2, x, barY+2)
	pdf.Line(x+barMM, barY-2, x+barMM, barY+2)
	// Half tick
	pdf.Line(x+barMM/2, barY-1, x+barMM/2, barY+1)

	// Labels
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(x, barY+2)
	pdf.CellFormat(10, 4, "0", "", 0, "L", false, 0, "")

	var distLabel string
	if niceMeters >= 1000 {
		distLabel = fmt.Sprintf("%.0f km", niceMeters/1000)
	} else {
		distLabel = fmt.Sprintf("%.0f m", niceMeters)
	}
	pdf.SetXY(x+barMM-10, barY+2)
	pdf.CellFormat(20, 4, distLabel, "", 0, "C", false, 0, "")

	// Scale ratio
	scaleRatio := niceMeters / (barMM / 1000) // meters per real-world meter at printed scale
	pdf.SetXY(x, y-1)
	pdf.CellFormat(barMM, 4, fmt.Sprintf("Scale approx 1:%.0f (at A3)", scaleRatio), "", 0, "L", false, 0, "")
}

// niceRound rounds to a "nice" number for scale bars (1, 2, 5, 10, 20, 50, 100, ...).
func niceRound(v float64) float64 {
	if v <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(v))
	base := math.Pow(10, exp)
	fraction := v / base

	switch {
	case fraction < 1.5:
		return base
	case fraction < 3.5:
		return 2 * base
	case fraction < 7.5:
		return 5 * base
	default:
		return 10 * base
	}
}

func drawFooter(pdf *gofpdf.Fpdf, plan *models.SitePlan) {
	pdf.SetFillColor(33, 37, 41)
	pdf.Rect(0, pageHeight-18, pageWidth, 18, "F")

	pdf.SetFont("Helvetica", "I", 7)
	pdf.SetTextColor(180, 180, 180)
	pdf.SetXY(marginLeft, pageHeight-14)
	pdf.CellFormat(pageWidth-marginLeft-marginRight, 4,
		"DISCLAIMER: This site plan is generated from publicly available SA government spatial data and Google Maps imagery.",
		"", 1, "L", false, 0, "")
	pdf.SetXY(marginLeft, pageHeight-10)
	pdf.CellFormat(pageWidth-marginLeft-marginRight, 4,
		"It is indicative only, not a licensed survey, and should not be relied upon for legal or construction purposes. Verify all dimensions independently.",
		"", 0, "L", false, 0, "")

	// Source attribution
	pdf.SetXY(pageWidth-marginRight-120, pageHeight-14)
	pdf.CellFormat(120, 4, "Cadastre: Location SA (Government of South Australia)", "", 1, "R", false, 0, "")
	pdf.SetXY(pageWidth-marginRight-120, pageHeight-10)
	pdf.CellFormat(120, 4, "Imagery: Google Maps Static API", "", 0, "R", false, 0, "")
}
