package authz

import (
	"context"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BoardAuthorizer struct{ db *pgxpool.Pool }

func NewBoardAuthorizer(db *pgxpool.Pool) *BoardAuthorizer { return &BoardAuthorizer{db: db} }

func (a *BoardAuthorizer) RequireBoardAccess(paramName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardID := strings.TrimSpace(c.Params(paramName))
		if boardID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "board id required"})
		}
		userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
		if userID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		ok, err := a.HasBoardAccess(c.Context(), boardID, userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to verify board access"})
		}
		if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
}

func (a *BoardAuthorizer) RequireTaskAccess(taskParam string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
		if userID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		taskID := strings.TrimSpace(c.Params(taskParam))
		if taskID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "task id required"})
		}
		ok, err := a.HasTaskAccess(c.Context(), taskID, userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to verify task access"})
		}
		if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
}

func (a *BoardAuthorizer) RequireDocumentAccess(docParam string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
		if userID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		docID := strings.TrimSpace(c.Params(docParam))
		if docID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "document id required"})
		}
		ok, err := a.HasDocumentAccess(c.Context(), docID, userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to verify document access"})
		}
		if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
}

func (a *BoardAuthorizer) HasBoardAccess(ctx context.Context, boardID, userID string) (bool, error) {
	var ok bool
	err := a.db.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM boards b
		LEFT JOIN board_members bm ON bm.board_id = b.id AND bm.member_id = $2
		WHERE b.id::text = $1 AND (bm.member_id IS NOT NULL OR COALESCE(b.user_id, '') = $2)
	)`, boardID, userID).Scan(&ok)
	return ok, err
}

func (a *BoardAuthorizer) HasTaskAccess(ctx context.Context, taskID, userID string) (bool, error) {
	var ok bool
	err := a.db.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1
		FROM tasks t
		JOIN boards b ON b.id = t.board_id
		LEFT JOIN board_members bm ON bm.board_id = b.id AND bm.member_id = $2
		WHERE t.id::text = $1 AND (bm.member_id IS NOT NULL OR COALESCE(b.user_id, '') = $2)
	)`, taskID, userID).Scan(&ok)
	return ok, err
}

func (a *BoardAuthorizer) HasDocumentAccess(ctx context.Context, docID, userID string) (bool, error) {
	var ok bool
	err := a.db.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1
		FROM documents d
		JOIN boards b ON b.id = d.board_id
		LEFT JOIN board_members bm ON bm.board_id = b.id AND bm.member_id = $2
		WHERE d.id::text = $1 AND (bm.member_id IS NOT NULL OR COALESCE(b.user_id, '') = $2)
	)`, docID, userID).Scan(&ok)
	return ok, err
}

func (a *BoardAuthorizer) RequireBoardAccessFromQuery(field string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
		if userID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		boardID := strings.TrimSpace(c.Query(field))
		if boardID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "board id required"})
		}
		ok, err := a.HasBoardAccess(c.Context(), boardID, userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to verify board access"})
		}
		if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
}

func (a *BoardAuthorizer) RequireBoardAccessFromBody(field string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := strings.TrimSpace(fmt.Sprintf("%v", c.Locals("auth.user_id")))
		if userID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var body map[string]interface{}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		boardID := strings.TrimSpace(fmt.Sprintf("%v", body[field]))
		if boardID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "board id required"})
		}
		ok, err := a.HasBoardAccess(c.Context(), boardID, userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to verify board access"})
		}
		if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
}
