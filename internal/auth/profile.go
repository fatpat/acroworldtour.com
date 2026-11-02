package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"awt/internal/store/gen"
)

// ErrOwnProfile refuses an admin editing their own email or name from the
// users page: the account page does it and re-checks the password for an
// email change.
var ErrOwnProfile = errors.New("use your account page to edit your own email and name")

// checkProfile normalises and validates an email and a name.
func checkProfile(email, name string) (string, string, error) {
	email, name = normEmail(email), strings.TrimSpace(name)
	if email == "" {
		return "", "", &InputError{Field: "email", Msg: "email is required"}
	}
	if !emailShape.MatchString(email) {
		return "", "", &InputError{Field: "email", Msg: "email must look like name@example.org"}
	}
	if name == "" {
		return "", "", &InputError{Field: "name", Msg: "name is required"}
	}
	return email, name, nil
}

// applyProfile stores email and name (already checked) on u and returns the
// fields that changed. A new email must not belong to another account.
func applyProfile(ctx context.Context, q *gen.Queries, u User, email, name string) ([]string, error) {
	var changed []string
	if email != u.Email {
		if _, err := q.GetUserByEmail(ctx, email); err == nil {
			return nil, &ExistsError{Email: email}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		changed = append(changed, "email")
	}
	if name != u.Name {
		changed = append(changed, "name")
	}
	if len(changed) == 0 {
		return nil, nil
	}
	if err := q.UpdateUserProfile(ctx, gen.UpdateUserProfileParams{Email: email, Name: name, ID: u.ID}); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return changed, nil
}

func logProfileChange(actor, userID string, changed []string) {
	slog.Info("user profile changed", "event", "user.profile_changed", "actor", actor, "user_id", userID, "fields", strings.Join(changed, ","))
}

// UpdateProfile lets an admin (actor) change another user's email and name.
// The user's sessions are kept: the email only identifies the account at
// login. It returns the fields that changed (none when nothing did).
func (a *Auth) UpdateProfile(ctx context.Context, actor, userID, email, name string) ([]string, error) {
	email, name, err := checkProfile(email, name)
	if err != nil {
		return nil, err
	}
	if actor == userID {
		return nil, ErrOwnProfile
	}
	var changed []string
	err = a.db.Tx(ctx, func(q *gen.Queries) error {
		u, err := q.GetUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("user %s: %w", userID, err)
		}
		changed, err = applyProfile(ctx, q, u, email, name)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update profile of %s: %w", userID, err)
	}
	if len(changed) > 0 {
		logProfileChange(actor, userID, changed)
	}
	return changed, nil
}

// UpdateOwnProfile changes the email and name of the session's account. A
// new email is a sensitive change, like a new password: it needs the current
// password and closes the user's other sessions (keepSession is kept). A
// name-only change needs neither.
func (a *Auth) UpdateOwnProfile(ctx context.Context, userID, email, name, currentPassword, keepSession string) ([]string, error) {
	email, name, err := checkProfile(email, name)
	if err != nil {
		return nil, err
	}
	u, err := a.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user %s: %w", userID, err)
	}
	var changed []string
	if email == u.Email {
		// Name only. The stored email is written back, so a change an admin
		// made meanwhile is kept.
		err = a.db.Tx(ctx, func(q *gen.Queries) error {
			u, err := q.GetUser(ctx, userID)
			if err != nil {
				return fmt.Errorf("user %s: %w", userID, err)
			}
			changed, err = applyProfile(ctx, q, u, u.Email, name)
			return err
		})
	} else {
		err = a.sensitive(ctx, userID, currentPassword, keepSession, func(q *gen.Queries, u User, _ time.Time) (bool, error) {
			changed, err = applyProfile(ctx, q, u, email, name)
			return len(changed) > 0, err
		})
	}
	if err != nil {
		return nil, fmt.Errorf("update profile of %s: %w", userID, err)
	}
	if len(changed) > 0 {
		logProfileChange(userID, userID, changed)
	}
	return changed, nil
}
