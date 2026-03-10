package boardhealth

import (
	"fmt"
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
	app.Get("/api/board-health", h.List)
	app.Get("/api/boards/:id/health", h.requireBoardAuth, h.GetBoardHealth)
}

func (h *Handler) List(c *fiber.Ctx) error {
	userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
	items, summary, err := h.svc.ListByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{
		"summary": summary,
		"boards":  items,
	})
}

func (h *Handler) GetBoardHealth(c *fiber.Ctx) error {
	items, summary, err := h.svc.ListByBoard(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{
		"summary": summary,
		"boards":  items,
	})
}
