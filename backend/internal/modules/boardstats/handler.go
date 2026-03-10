package boardstats

import "github.com/gofiber/fiber/v2"

type Handler struct {
	svc              *Service
	requireBoardAuth fiber.Handler
}

func NewHandler(svc *Service, requireBoardAuth fiber.Handler) *Handler {
	return &Handler{svc: svc, requireBoardAuth: requireBoardAuth}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Post("/api/board-stats/recompute", h.RecomputeAll)
	app.Post("/api/boards/:id/stats/recompute", h.requireBoardAuth, h.RecomputeBoard)
	app.Get("/api/boards/:id/stats", h.requireBoardAuth, h.GetBoardStats)
}

func (h *Handler) RecomputeAll(c *fiber.Ctx) error {
	if err := h.svc.RecomputeAll(c.Context()); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) RecomputeBoard(c *fiber.Ctx) error {
	if err := h.svc.RecomputeBoard(c.Context(), c.Params("id")); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) GetBoardStats(c *fiber.Ctx) error {
	item, err := h.svc.GetByBoard(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(item)
}
