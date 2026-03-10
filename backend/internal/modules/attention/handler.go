package attention

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc              *Service
	requireBoardAuth fiber.Handler
}

func NewHandler(svc *Service, requireBoardAuth fiber.Handler) *Handler {
	return &Handler{svc: svc, requireBoardAuth: requireBoardAuth}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/attention", h.List)
	app.Get("/api/boards/:id/attention", h.requireBoardAuth, h.ListBoard)
}

func (h *Handler) List(c *fiber.Ctx) error {
	userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
	limit := parseLimit(c.Query("limit"), 8)
	items, err := h.svc.ListByUser(c.Context(), userID, limit)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) ListBoard(c *fiber.Ctx) error {
	limit := parseLimit(c.Query("limit"), 8)
	items, err := h.svc.ListByBoard(c.Context(), c.Params("id"), limit)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}

func parseLimit(raw string, fallback int) int {
	if raw = strings.TrimSpace(raw); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			return n
		}
	}
	return fallback
}
