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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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
	// reauth limits wrong current passwords on account actions, per user.
	reauth *LoginLimiter
	// beforeSensitiveTx runs between the password check and the
	// transaction of a sensitive action; tests use it to race a change.
	beforeSensitiveTx func()
	now               func() time.Time
	cost              int // bcrypt cost; lowered by tests only
}

// New builds an Auth on db.
func New(db *store.DB, cfg config.Config) *Auth {
	return &Auth{db: db, cfg: cfg, limiter: NewLoginLimiter(5, 15*time.Minute), reauth: NewLoginLimiter(5, 15*time.Minute), now: time.Now, cost: defaultBcryptCost}
}

const minPasswordLen = 10

var emailShape = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// InputError is a rejected user input; Field names the form field.
type InputError struct{ Field, Msg string }

func (e *InputError) Error() string { return e.Msg }

// ExistsError is returned when creating a user whose email is taken.
type ExistsError struct{ Email string }

func (e *ExistsError) Error() string { return fmt.Sprintf("user %s already exists", e.Email) }

func checkPassword(p string) error {
	if utf8.RuneCountInString(p) < minPasswordLen {
		return &InputError{Field: "password", Msg: fmt.Sprintf("password must be at least %d characters", minPasswordLen)}
	}
	return nil
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
	if email == "" {
		return User{}, &InputError{Field: "email", Msg: "email is required"}
	}
	if !emailShape.MatchString(email) {
		return User{}, &InputError{Field: "email", Msg: "email must look like name@example.org"}
	}
	if strings.TrimSpace(name) == "" {
		return User{}, &InputError{Field: "name", Msg: "name is required"}
	}
	if err := checkPassword(password); err != nil {
		return User{}, err
	}
	if role != RoleAdmin && role != RoleScorer {
		return User{}, &InputError{Field: "role", Msg: fmt.Sprintf("role %q: must be admin or scorer", role)}
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
			return &ExistsError{Email: email}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return q.UpsertUser(ctx, gen.UpsertUserParams{ID: u.ID, Email: u.Email, Name: u.Name, PasswordHash: u.PasswordHash, Role: u.Role})
	})
	return u, err
}

// SetPassword replaces a user's password and revokes their sessions.
func (a *Auth) SetPassword(ctx context.Context, email, password string) error {
	if err := checkPassword(password); err != nil {
		return err
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
		id, err := resolveCompetition(ctx, q, competition)
		if err == nil && id == "" {
			err = sql.ErrNoRows
		}
		if err != nil {
			return fmt.Errorf("competition %s: %w", competition, err)
		}
		return q.AddUserCompetition(ctx, gen.AddUserCompetitionParams{UserID: u.ID, CompetitionID: id})
	})
}

// ListUsers returns every account by email.
func (a *Auth) ListUsers(ctx context.Context) ([]User, error) {
	return gen.New(a.db.Read).ListUsers(ctx)
}

// GetUser returns an account by id; sql.ErrNoRows when unknown.
func (a *Auth) GetUser(ctx context.Context, id string) (User, error) {
	return gen.New(a.db.Read).GetUser(ctx, id)
}

// UserCompetitions returns the ids of the competitions a user is assigned to.
func (a *Auth) UserCompetitions(ctx context.Context, userID string) ([]string, error) {
	return gen.New(a.db.Read).ListUserCompetitions(ctx, userID)
}

// SetDisabled disables or re-enables an account. Disabling revokes the
// user's sessions.
func (a *Auth) SetDisabled(ctx context.Context, id string, disabled bool) error {
	return a.db.Tx(ctx, func(q *gen.Queries) error {
		u, err := q.GetUser(ctx, id)
		if err != nil {
			return fmt.Errorf("user %s: %w", id, err)
		}
		u.Disabled = disabled
		if err := q.UpsertUser(ctx, gen.UpsertUserParams{ID: u.ID, Email: u.Email, Name: u.Name, PasswordHash: u.PasswordHash, Role: u.Role, Disabled: u.Disabled}); err != nil {
			return err
		}
		if disabled {
			return q.DeleteUserSessions(ctx, id)
		}
		return nil
	})
}

// SetUserCompetitions replaces the competitions (ids) a user is assigned to.
func (a *Auth) SetUserCompetitions(ctx context.Context, userID string, competitions []string) error {
	return a.db.Tx(ctx, func(q *gen.Queries) error {
		if _, err := q.GetUser(ctx, userID); err != nil {
			return fmt.Errorf("user %s: %w", userID, err)
		}
		if err := q.DeleteUserCompetitions(ctx, userID); err != nil {
			return err
		}
		for _, c := range competitions {
			if err := q.AddUserCompetition(ctx, gen.AddUserCompetitionParams{UserID: userID, CompetitionID: c}); err != nil {
				return fmt.Errorf("assign competition %s: %w", c, err)
			}
		}
		return nil
	})
}

// SetPasswordByID is SetPassword for an account given by id.
func (a *Auth) SetPasswordByID(ctx context.Context, id, password string) error {
	u, err := a.GetUser(ctx, id)
	if err != nil {
		return fmt.Errorf("user %s: %w", id, err)
	}
	return a.SetPassword(ctx, u.Email, password)
}
