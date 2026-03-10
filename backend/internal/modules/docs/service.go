package docs

import (
	"context"
	"errors"
	"strings"
)

type EmbeddingUpdater func(id int, text string)
type Embedder func(text string) (string, error)

type BoardStatsUpdater interface {
	RecomputeBoard(ctx context.Context, boardID string) error
}

type Service struct {
	repo            *Repository
	embeddingUpdate EmbeddingUpdater
	embedder        Embedder
	boardStats      BoardStatsUpdater
}

func NewService(repo *Repository, embeddingUpdate EmbeddingUpdater, embedder Embedder, boardStats BoardStatsUpdater) *Service {
	return &Service{repo: repo, embeddingUpdate: embeddingUpdate, embedder: embedder, boardStats: boardStats}
}

func (s *Service) ListByBoard(ctx context.Context, boardID string) ([]Document, error) {
	return s.repo.ListByBoard(ctx, boardID)
}

func (s *Service) Create(ctx context.Context, boardID string, d *Document) error {
	d.Title = strings.TrimSpace(d.Title)
	if d.Title == "" {
		return errors.New("title is required")
	}
	if err := s.repo.Create(ctx, boardID, d); err != nil {
		return err
	}
	d.BoardID = boardID
	if s.embeddingUpdate != nil {
		go s.embeddingUpdate(d.ID, d.Title+" "+d.Content)
	}
	if s.boardStats != nil {
		go s.boardStats.RecomputeBoard(context.Background(), boardID)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, id int, patch *Document) (Document, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Document{}, err
	}
	if strings.TrimSpace(patch.Title) != "" {
		existing.Title = patch.Title
	}
	if patch.Content != "" {
		existing.Content = patch.Content
	}
	if err := s.repo.Update(ctx, id, existing.Title, existing.Content); err != nil {
		return Document{}, err
	}
	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Document{}, err
	}
	if s.embeddingUpdate != nil {
		go s.embeddingUpdate(updated.ID, updated.Title+" "+updated.Content)
	}
	if s.boardStats != nil {
		go s.boardStats.RecomputeBoard(context.Background(), updated.BoardID)
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int) (bool, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	deleted, err := s.repo.Delete(ctx, id)
	if err != nil {
		return false, err
	}
	if deleted && s.boardStats != nil {
		go s.boardStats.RecomputeBoard(context.Background(), existing.BoardID)
	}
	return deleted, nil
}

func (s *Service) Search(ctx context.Context, boardID, query string) ([]Document, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("query required")
	}
	if s.embedder == nil {
		return nil, errors.New("embedder not configured")
	}
	embedding, err := s.embedder(query)
	if err != nil {
		return nil, err
	}
	return s.repo.Search(ctx, boardID, embedding)
}
