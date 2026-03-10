package docs

import (
	"context"
	"errors"
	"strings"
)

type EmbeddingUpdater func(id int, text string)
type Embedder func(text string) (string, error)

type Service struct {
	repo            *Repository
	embeddingUpdate EmbeddingUpdater
	embedder        Embedder
}

func NewService(repo *Repository, embeddingUpdate EmbeddingUpdater, embedder Embedder) *Service {
	return &Service{repo: repo, embeddingUpdate: embeddingUpdate, embedder: embedder}
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
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int) (bool, error) {
	return s.repo.Delete(ctx, id)
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
