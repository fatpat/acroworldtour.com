package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"awt/internal/store/gen"
)

// CookieName is the session cookie.
const CookieName = "awt_session"

const sessionTTL = 14 * 24 * time.Hour

// Session is a logged-in session. Token is only known at login time; the
// database keeps its SHA-256 hash.
type Session struct {
	Token     string
	CSRF      string
	ExpiresAt time.Time
	User      User
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// dummyHash is a bcrypt cost 12 hash of a throwaway string, compared against
// when the user is unknown so timing does not reveal which emails exist.
var dummyHash = []byte("$2a$12$n0NefR8ArDHYJjtIsCy5JOtczxTeFEo6SivcaNkyJDh0.OZokgFqS")

// Login checks the credentials and opens a new session (a fresh token on every
// login).
func (a *Auth) Login(ctx context.Context, email, password string) (Session, error) {
	u, err := gen.New(a.db.Read).GetUserByEmail(ctx, normEmail(email))
	hash := []byte(u.PasswordHash)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return Session{}, err
		}
		hash = dummyHash // keep the timing of unknown users close to real ones
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil || err != nil || u.Disabled {
		return Session{}, ErrInvalidCredentials
	}
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	now := a.now().UTC()
	s := Session{Token: token, CSRF: csrf, ExpiresAt: now.Add(sessionTTL), User: u}
	err = a.db.Tx(ctx, func(q *gen.Queries) error {
		if err := q.DeleteExpiredSessions(ctx, now.Format(time.RFC3339)); err != nil {
			return err
		}
		return q.InsertSession(ctx, gen.InsertSessionParams{
			TokenHash: hashToken(token), UserID: u.ID, CsrfToken: csrf,
			ExpiresAt: s.ExpiresAt.Format(time.RFC3339), CreatedAt: now.Format(time.RFC3339),
		})
	})
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return s, nil
}

// SetCookie writes the session cookie.
func (a *Auth) SetCookie(w http.ResponseWriter, s Session) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: s.Token, Path: "/",
		MaxAge: int(sessionTTL / time.Second), Expires: s.ExpiresAt,
		HttpOnly: true, Secure: a.cfg.Env != "dev", SameSite: http.SameSiteLaxMode,
	})
}

// Logout deletes the session behind the request cookie and clears the cookie.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.cfg.Env != "dev", SameSite: http.SameSiteLaxMode,
	})
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil
	}
	return a.db.Tx(r.Context(), func(q *gen.Queries) error {
		return q.DeleteSession(r.Context(), hashToken(c.Value))
	})
}

var errNoSession = errors.New("no valid session")

// UserFromRequest resolves the session cookie to a live session and an
// enabled user.
func (a *Auth) UserFromRequest(r *http.Request) (Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return Session{}, errNoSession
	}
	q := gen.New(a.db.Read)
	row, err := q.GetSession(r.Context(), gen.GetSessionParams{TokenHash: hashToken(c.Value), ExpiresAt: a.now().UTC().Format(time.RFC3339)})
	if err != nil {
		return Session{}, errNoSession
	}
	u, err := q.GetUser(r.Context(), row.UserID)
	if err != nil || u.Disabled {
		return Session{}, errNoSession
	}
	exp, err := time.Parse(time.RFC3339, row.ExpiresAt)
	if err != nil {
		return Session{}, errNoSession
	}
	return Session{Token: c.Value, CSRF: row.CsrfToken, ExpiresAt: exp, User: u}, nil
}
