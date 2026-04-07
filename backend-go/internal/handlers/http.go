package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"landrisk/backend-go/internal/domain"
)

type AnalyzerService interface {
	Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.Report, bool, error)
	GetReport(ctx context.Context, id string) (domain.StoredReport, error)
	GetReportPDF(ctx context.Context, id string) ([]byte, error)
	GetTrace(ctx context.Context, id string) (domain.TraceResponse, error)
}

type Handler struct {
	analyzer AnalyzerService
	logger   zerolog.Logger
}

func New(analyzer AnalyzerService, logger zerolog.Logger) *Handler {
	return &Handler{
		analyzer: analyzer,
		logger:   logger.With().Str("component", "http-handler").Logger(),
	}
}

func (h *Handler) Register(app *fiber.App) {
	app.Post("/analyze", h.Analyze)
	app.Get("/report/:id", h.GetReport)
	app.Get("/trace/:id", h.GetTrace)
	app.Get("/health", h.Health)
}

func (h *Handler) Analyze(ctx *fiber.Ctx) error {
	var req domain.AnalyzeRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	report, cacheHit, err := h.analyzer.Analyze(ctx.UserContext(), req)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "failed") {
			status = http.StatusServiceUnavailable
		}
		return ctx.Status(status).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"report": report, "cacheHit": cacheHit})
}

func (h *Handler) GetReport(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	format := strings.ToLower(ctx.Query("format", "json"))
	if format == "pdf" {
		pdfBytes, err := h.analyzer.GetReportPDF(ctx.UserContext(), id)
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				status = http.StatusNotFound
			}
			return ctx.Status(status).JSON(fiber.Map{"error": err.Error()})
		}
		ctx.Set("Content-Type", "application/pdf")
		ctx.Set("Content-Disposition", "attachment; filename=land-risk-report-"+id+".pdf")
		return ctx.Send(pdfBytes)
	}

	stored, err := h.analyzer.GetReport(ctx.UserContext(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			status = http.StatusNotFound
		}
		return ctx.Status(status).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(stored)
}

func (h *Handler) GetTrace(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	trace, err := h.analyzer.GetTrace(ctx.UserContext(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			status = http.StatusNotFound
		}
		return ctx.Status(status).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(trace)
}

func (h *Handler) Health(ctx *fiber.Ctx) error {
	return ctx.JSON(fiber.Map{"status": "ok"})
}
