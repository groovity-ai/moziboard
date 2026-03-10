package home

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/home/overview", h.GetOverview)
}

func (h *Handler) GetOverview(c *fiber.Ctx) error {
	userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
	overview, err := h.svc.GetOverview(c.Context(), userID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(overview)
}
