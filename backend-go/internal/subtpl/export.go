package subtpl

import "time"

// NextTrafficResetAt ports getNextTrafficResetAt at Now().
func NextTrafficResetAt(strategy string, createdAt time.Time) *time.Time {
	return nextTrafficResetAt(strategy, createdAt, Now())
}

// Shuffle returns a shuffled copy like lodash's _.shuffle.
func Shuffle[T any](list []T) []T {
	out := append([]T(nil), list...)
	shuffle(out)
	return out
}
