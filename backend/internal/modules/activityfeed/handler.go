package activityfeed

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
	app.Get("/api/activityfeed", h.ListRecent)
	app.Get("/api/boards/:id/activityfeed", h.requireBoardAuth, h.ListBoardRecent)
}

func (h *Handler) ListRecent(c *fiber.Ctx) error {
	userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
	limit := parseLimit(c.Query("limit"), 12)
	items, err := h.svc.ListRecentByUser(c.Context(), userID, limit)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) ListBoardRecent(c *fiber.Ctx) error {
	limit := parseLimit(c.Query("limit"), 12)
	items, err := h.svc.ListRecentByBoard(c.Context(), c.Params("id"), limit)
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
