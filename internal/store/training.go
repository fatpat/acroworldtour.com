package store

import (
	"context"
	"fmt"

	"awt/internal/store/gen"
)

// TrainingAuthReset counts what ResetTrainingAuth removed.
type TrainingAuthReset struct {
	Sessions        int64
	LoginChallenges int64
	TOTP            int64
	SecurityKeys    int64
	RecoveryCodes   int64
	MFALocks        int64 // users whose MFA failure count or lock was cleared
}

// ResetTrainingAuth prepares a copy of the prod database for training
// (decision 0018): in one transaction it deletes every session and pending
// login and every second factor (authenticator apps, security keys,
// recovery codes) and clears the MFA failure counters and locks. Accounts
// and passwords stay: managers log in to training with their prod password.
func ResetTrainingAuth(ctx context.Context, db *DB) (TrainingAuthReset, error) {
	var r TrainingAuthReset
	err := db.Tx(ctx, func(q *gen.Queries) error {
		steps := []struct {
			what string
			do   func(context.Context) (int64, error)
			n    *int64
		}{
			{"sessions", q.DeleteAllSessions, &r.Sessions},
			{"pending logins", q.DeleteAllLoginChallenges, &r.LoginChallenges},
			{"authenticator apps", q.DeleteAllUserTOTP, &r.TOTP},
			{"security keys", q.DeleteAllUserWebAuthnCredentials, &r.SecurityKeys},
			{"recovery codes", q.DeleteAllUserRecoveryCodes, &r.RecoveryCodes},
			{"mfa lockouts", q.ResetAllUserMFAFailures, &r.MFALocks},
		}
		for _, s := range steps {
			n, err := s.do(ctx)
			if err != nil {
				return fmt.Errorf("delete %s: %w", s.what, err)
			}
			*s.n = n
		}
		return nil
	})
	if err != nil {
		return TrainingAuthReset{}, fmt.Errorf("reset training auth: %w", err)
	}
	return r, nil
}

// CloneSummary counts the users, competitions, news and files of a
// database, for the training refresh's report.
func CloneSummary(ctx context.Context, db *DB) (gen.CloneSummaryRow, error) {
	s, err := gen.New(db.Read).CloneSummary(ctx)
	if err != nil {
		return s, fmt.Errorf("clone summary: %w", err)
	}
	return s, nil
}
