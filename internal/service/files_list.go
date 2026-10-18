package service

import (
	"context"
	"fmt"
)

// ListFiles returns the active uploaded files, newest first.
func (s *Service) ListFiles(ctx context.Context) ([]File, error) {
	rows, err := s.read().ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	out := make([]File, 0, len(rows))
	for _, r := range rows {
		out = append(out, fileFromRow(r))
	}
	return out, nil
}
