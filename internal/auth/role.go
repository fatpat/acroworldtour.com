package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"awt/internal/store/gen"
)

// ErrOwnRole refuses an admin changing their own role: another admin (or the
// CLI) must do it.
var ErrOwnRole = errors.New("you cannot change your own role")

// ErrLastAdmin refuses demoting the last enabled admin, which would leave no
// one able to manage accounts from the manager.
var ErrLastAdmin = errors.New("cannot demote the last enabled admin")

// SetRole changes a user's role and closes their sessions, so the new role
// applies from their next login. actor is the acting user's id, or "cli". It
// reports false, and changes nothing, when the user already has the role.
// Competition assignments are kept: admins ignore them, and they apply again
// after a demotion to scorer.
func (a *Auth) SetRole(ctx context.Context, actor, userID, role string) (bool, error) {
	if role != RoleAdmin && role != RoleScorer {
		return false, &InputError{Field: "role", Msg: fmt.Sprintf("role %q: must be admin or scorer", role)}
	}
	if actor == userID {
		return false, ErrOwnRole
	}
	var old string
	err := a.db.Tx(ctx, func(q *gen.Queries) error {
		u, err := q.GetUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("user %s: %w", userID, err)
		}
		old = u.Role
		if old == role {
			return nil
		}
		if err := q.UpdateUserRole(ctx, gen.UpdateUserRoleParams{Role: role, ID: userID}); err != nil {
			return err
		}
		// Counted after the update, inside the (BEGIN IMMEDIATE) transaction,
		// so two admins demoting each other cannot both succeed.
		n, err := q.CountEnabledAdmins(ctx)
		if err != nil {
			return fmt.Errorf("count admins: %w", err)
		}
		if n == 0 && old == RoleAdmin && !u.Disabled {
			return ErrLastAdmin
		}
		if err := q.DeleteUserSessions(ctx, userID); err != nil {
			return fmt.Errorf("close sessions: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("set role of %s: %w", userID, err)
	}
	if old == role {
		return false, nil
	}
	slog.Info("user role changed", "event", "user.role_changed", "actor", actor, "user_id", userID, "old_role", old, "new_role", role)
	return true, nil
}
