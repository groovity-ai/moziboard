package docs

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc                 *Service
	requireBoardAuth    fiber.Handler
	requireBoardQuery   fiber.Handler
	requireDocumentAuth fiber.Handler
}

func NewHandler(svc *Service, requireBoardAuth fiber.Handler, requireBoardQuery fiber.Handler, requireDocumentAuth fiber.Handler) *Handler {
	return &Handler{svc: svc, requireBoardAuth: requireBoardAuth, requireBoardQuery: requireBoardQuery, requireDocumentAuth: requireDocumentAuth}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/boards/:id/docs", h.requireBoardAuth, h.GetBoardDocs)
	app.Post("/api/boards/:id/docs", h.requireBoardAuth, h.CreateDoc)
	app.Put("/api/docs/:id", h.requireDocumentAuth, h.UpdateDoc)
	app.Delete("/api/docs/:id", h.requireDocumentAuth, h.DeleteDoc)
	app.Get("/api/docs/search", h.requireBoardQuery, h.SearchDocs)
}

func (h *Handler) GetBoardDocs(c *fiber.Ctx) error {
	docs, err := h.svc.ListByBoard(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(docs)
}

func (h *Handler) CreateDoc(c *fiber.Ctx) error {
	var d Document
	if err := c.BodyParser(&d); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.Create(c.Context(), c.Params("id"), &d); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	return c.JSON(d)
}

func (h *Handler) UpdateDoc(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var patch Document
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	updated, err := h.svc.Update(c.Context(), id, &patch)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return c.Status(404).SendString("Document not found")
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(updated)
}

func (h *Handler) DeleteDoc(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	deleted, err := h.svc.Delete(c.Context(), id)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	if !deleted {
		return c.Status(404).SendString("Document not found")
	}
	return c.SendStatus(200)
}

func (h *Handler) SearchDocs(c *fiber.Ctx) error {
	docs, err := h.svc.Search(c.Context(), c.Query("board_id"), c.Query("q"))
	if err != nil {
		if err.Error() == "query required" {
			return c.Status(400).SendString(err.Error())
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(docs)
}
