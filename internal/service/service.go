// Package service holds the use cases of the back-office: every mutation runs
// in one store transaction, enforces the domain rules (internal/domain) and
// publishes a live-update event after commit.
package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"awt/internal/blob"
	"awt/internal/civl"
	"awt/internal/domain"
	"awt/internal/store"
	"awt/internal/store/gen"
)

// Errors returned by the service. Handlers map them to HTTP codes in one
// place; state machine refusals wrap domain.ErrInvalidTransition.
var (
	// ErrNotFound: the entity does not exist or is soft-deleted.
	ErrNotFound = errors.New("not found")
	// ErrFrozen: the competition was imported from legacy and its results
	// are official (decision 0003); it refuses every mutation.
	ErrFrozen = errors.New("competition is frozen (imported results)")
	// ErrConflict: a name, code or acronym is already used.
	ErrConflict = errors.New("already exists")
)

// ValidationError lists the invalid input fields.
type ValidationError struct {
	Fields []domain.FieldError
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		msgs[i] = f.Error()
	}
	return "invalid input: " + strings.Join(msgs, "; ")
}

// invalid returns a *ValidationError, or nil when there are no field errors.
func invalid(fields ...domain.FieldError) error {
	if len(fields) == 0 {
		return nil
	}
	return &ValidationError{Fields: fields}
}

// refused wraps a business precondition failure (e.g. a run still open) as
// a refused state transition, keeping its explanatory message.
func refused(err error) error {
	return fmt.Errorf("%w: %w", domain.ErrInvalidTransition, err)
}

// CIVLClient is the part of civl.Client the service uses.
type CIVLClient interface {
	FetchPilot(ctx context.Context, civlid int) (civl.PilotData, error)
	FetchRankings(ctx context.Context) (map[int]int, error)
}

// Service is the use case layer on top of the store.
type Service struct {
	db    *store.DB
	blobs blob.Store
	bus   *Bus
	civl  CIVLClient
}

// New returns a Service.
func New(db *store.DB, blobs blob.Store, bus *Bus, civl CIVLClient) *Service {
	return &Service{db: db, blobs: blobs, bus: bus, civl: civl}
}

// read returns queries on the read pool.
func (s *Service) read() *gen.Queries { return gen.New(s.db.Read) }

// newID returns a new 24-char lowercase hex id.
func newID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// now is the current time as stored: RFC 3339 UTC.
func now() string { return time.Now().UTC().Format(time.RFC3339) }

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// notFound turns sql.ErrNoRows into ErrNotFound.
func notFound(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", what, ErrNotFound)
	}
	return fmt.Errorf("get %s: %w", what, err)
}

// sqliteCoder is implemented by modernc.org/sqlite errors.
type sqliteCoder interface{ Code() int }

// Extended SQLite result codes of a uniqueness violation.
const (
	sqliteConstraintPrimaryKey = 1555
	sqliteConstraintUnique     = 2067
)

// conflict maps a uniqueness violation to ErrConflict.
func conflict(err error, what string) error {
	var c sqliteCoder
	if errors.As(err, &c) && (c.Code() == sqliteConstraintUnique || c.Code() == sqliteConstraintPrimaryKey) {
		return fmt.Errorf("%s: %w", what, ErrConflict)
	}
	return fmt.Errorf("save %s: %w", what, err)
}
