package service

import (
	"context"
	"errors"
)

var errTokenVersionUpdateUnsupported = errors.New("user repository does not support token version updates")

type passwordTokenVersionUpdater interface {
	UpdatePasswordAndIncrementTokenVersion(ctx context.Context, userID int64, passwordHash string) (int64, error)
}

type tokenVersionIncrementer interface {
	IncrementTokenVersion(ctx context.Context, userID int64) (int64, error)
}

func updatePasswordAndIncrementTokenVersion(ctx context.Context, repo UserRepository, userID int64, passwordHash string) (int64, error) {
	updater, ok := repo.(passwordTokenVersionUpdater)
	if !ok {
		return 0, errTokenVersionUpdateUnsupported
	}
	return updater.UpdatePasswordAndIncrementTokenVersion(ctx, userID, passwordHash)
}

func incrementTokenVersion(ctx context.Context, repo UserRepository, userID int64) (int64, error) {
	updater, ok := repo.(tokenVersionIncrementer)
	if !ok {
		return 0, errTokenVersionUpdateUnsupported
	}
	return updater.IncrementTokenVersion(ctx, userID)
}
