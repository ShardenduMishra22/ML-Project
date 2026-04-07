package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"

	"landrisk/backend-go/internal/domain"
)

func BuildReport(
	reportID string,
	request domain.AnalyzeRequest,
	preprocessed map[string]any,
	satellite domain.SatelliteFeatures,
	prediction domain.MLPrediction,
	validationResult domain.ValidationSummary,
	riskClass string,
	confidence float64,
	explanation []string,
	apiTrace []domain.APICallTrace,
	latency domain.LatencyMetrics,
) domain.Report {
	return domain.Report{
		ID:                reportID,
		CreatedAt:         time.Now().UTC(),
		Input:             request,
		AdminHierarchy:    mapOrEmpty(preprocessed["adminHierarchy"]),
		SurveyDetails:     mapOrEmpty(preprocessed["surveyDetails"]),
		Geometry:          mapOrEmpty(preprocessed["geometry"]),
		Jurisdiction:      mapOrEmpty(preprocessed["jurisdiction"]),
		Zonation:          mapOrEmpty(preprocessed["zonation"]),
		SatelliteFeatures: satellite,
		MLPrediction:      prediction,
		RiskClass:         riskClass,
		Confidence:        confidence,
		Explanation:       explanation,
		ValidationSummary: validationResult,
		Evidence:          mapOrEmpty(preprocessed["evidence"]),
		APITrace:          apiTrace,
		LatencyMetrics:    latency,
	}
}

func GeneratePDF(report domain.Report) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "Land Risk Assessment Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 8, fmt.Sprintf("Report ID: %s", report.ID), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 8, fmt.Sprintf("Created At: %s", report.CreatedAt.Format(time.RFC3339)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 8, fmt.Sprintf("Risk Class: %s", report.RiskClass), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 8, fmt.Sprintf("Confidence: %.2f", report.Confidence), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 8, "Explanation", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	for _, item := range report.Explanation {
		pdf.MultiCell(0, 6, "- "+item, "", "L", false)
	}

	pdf.Ln(1)
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 8, "Validation Summary", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	validationBytes, _ := json.MarshalIndent(report.ValidationSummary, "", "  ")
	pdf.MultiCell(0, 5, string(validationBytes), "", "L", false)

	pdf.Ln(1)
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 8, "Evidence", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	evidenceBytes, _ := json.MarshalIndent(report.Evidence, "", "  ")
	pdf.MultiCell(0, 5, string(evidenceBytes), "", "L", false)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func mapOrEmpty(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return map[string]any{}
}
