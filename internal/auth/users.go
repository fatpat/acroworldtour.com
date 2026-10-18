// Package auth implements accounts, sessions, CSRF protection and role checks
// for the back-office (decision 0009).
package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"awt/internal/config"
	"awt/internal/store"
	"awt/internal/store/gen"
)

const defaultBcryptCost = 12

// Roles.
const (
	RoleAdmin  = "admin"
	RoleScorer = "scorer"
)

// ErrInvalidCredentials is returned for an unknown, disabled or wrong-password
// account, without saying which.
var ErrInvalidCredentials = errors.New("invalid credentials")

// User is an account row.
type User = gen.User

// Auth owns the user and session logic.
type Auth struct {
	db      *store.DB
	cfg     config.Config
	limiter *LoginLimiter
	now     func() time.Time
	cost    int // bcrypt cost; lowered by tests only

	dummyOnce sync.Once
	dummyHash []byte
}

// New builds an Auth on db.
func New(db *store.DB, cfg config.Config) *Auth {
	return &Auth{db: db, cfg: cfg, limiter: NewLoginLimiter(5, 15*time.Minute), now: time.Now, cost: defaultBcryptCost}
}

func normEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func newUserID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateUser adds an account. The email is stored lower-cased.
func (a *Auth) CreateUser(ctx context.Context, email, name, role, password string) (User, error) {
	email = normEmail(email)
	if email == "" || password == "" {
		return User{}, errors.New("email and password are required")
	}
	if role != RoleAdmin && role != RoleScorer {
		return User{}, fmt.Errorf("role %q: must be admin or scorer", role)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), a.cost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	id, err := newUserID()
	if err != nil {
		return User{}, err
	}
	u := User{ID: id, Email: email, Name: name, PasswordHash: string(hash), Role: role}
	err = a.db.Tx(ctx, func(q *gen.Queries) error {
		if _, err := q.GetUserByEmail(ctx, email); err == nil {
			return fmt.Errorf("user %s already exists", email)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return q.UpsertUser(ctx, gen.UpsertUserParams{ID: u.ID, Email: u.Email, Name: u.Name, PasswordHash: u.PasswordHash, Role: u.Role})
	})
	return u, err
}

// SetPassword replaces a user's password and revokes their sessions.
func (a *Auth) SetPassword(ctx context.Context, email, password string) error {
	if password == "" {
		return errors.New("password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), a.cost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return a.db.Tx(ctx, func(q *gen.Queries) error {
		u, err := q.GetUserByEmail(ctx, normEmail(email))
		if err != nil {
			return fmt.Errorf("user %s: %w", email, err)
		}
		if err := q.UpdateUserPassword(ctx, gen.UpdateUserPasswordParams{PasswordHash: string(hash), ID: u.ID}); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, u.ID)
	})
}

// AssignCompetition lets a user work on a competition, given by id or code.
func (a *Auth) AssignCompetition(ctx context.Context, email, competition string) error {
	return a.db.Tx(ctx, func(q *gen.Queries) error {
		u, err := q.GetUserByEmail(ctx, normEmail(email))
		if err != nil {
			return fmt.Errorf("user %s: %w", email, err)
		}
		c, err := q.GetCompetition(ctx, competition)
		if errors.Is(err, sql.ErrNoRows) {
			c, err = q.GetCompetitionByCode(ctx, competition)
		}
		if err != nil {
			return fmt.Errorf("competition %s: %w", competition, err)
		}
		return q.AddUserCompetition(ctx, gen.AddUserCompetitionParams{UserID: u.ID, CompetitionID: c.ID})
	})
}
