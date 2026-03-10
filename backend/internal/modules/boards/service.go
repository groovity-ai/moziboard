package boards

import (
	"context"
	"errors"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListBoards(ctx context.Context) ([]Board, error) {
	return s.repo.ListBoards(ctx)
}

func (s *Service) CreateBoard(ctx context.Context, b *Board) error {
	b.Title = strings.TrimSpace(b.Title)
	if b.Title == "" {
		return errors.New("title is required")
	}
	return s.repo.CreateBoard(ctx, b)
}

func (s *Service) ListMembers(ctx context.Context) ([]Member, error) {
	return s.repo.ListMembers(ctx)
}

func (s *Service) ListBoardMembers(ctx context.Context, boardID string) ([]Member, error) {
	return s.repo.ListBoardMembers(ctx, boardID)
}

func (s *Service) AddBoardMember(ctx context.Context, boardID string, req BoardMemberReq) error {
	if strings.TrimSpace(req.MemberID) == "" {
		return errors.New("member_id is required")
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "editor"
	}
	return s.repo.AddBoardMember(ctx, boardID, req.MemberID, role)
}

func (s *Service) RemoveBoardMember(ctx context.Context, boardID, memberID string) error {
	if strings.TrimSpace(memberID) == "" {
		return errors.New("member id is required")
	}
	return s.repo.RemoveBoardMember(ctx, boardID, memberID)
}
