package boards

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/boards", h.GetBoards)
	app.Post("/api/boards", h.CreateBoard)
	app.Get("/api/members", h.GetMembers)
	app.Get("/api/boards/:id/members", h.GetBoardMembers)
	app.Post("/api/boards/:id/members", h.AddBoardMember)
	app.Delete("/api/boards/:id/members/:mid", h.RemoveBoardMember)
}

func (h *Handler) GetBoards(c *fiber.Ctx) error {
	boards, err := h.svc.ListBoards(c.Context())
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(boards)
}

func (h *Handler) CreateBoard(c *fiber.Ctx) error {
	var b Board
	if err := c.BodyParser(&b); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.CreateBoard(c.Context(), &b); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	return c.JSON(b)
}

func (h *Handler) GetMembers(c *fiber.Ctx) error {
	members, err := h.svc.ListMembers(c.Context())
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(members)
}

func (h *Handler) GetBoardMembers(c *fiber.Ctx) error {
	members, err := h.svc.ListBoardMembers(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(members)
}

func (h *Handler) AddBoardMember(c *fiber.Ctx) error {
	var req BoardMemberReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.AddBoardMember(c.Context(), c.Params("id"), req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	return c.SendStatus(204)
}

func (h *Handler) RemoveBoardMember(c *fiber.Ctx) error {
	if err := h.svc.RemoveBoardMember(c.Context(), c.Params("id"), c.Params("mid")); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	return c.SendStatus(204)
}
