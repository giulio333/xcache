package memory

import "time"

type item struct {
	value     any
	expiresAt time.Time // zero value means no expiration
}

func (i item) isExpired() bool {
	return !i.expiresAt.IsZero() && time.Now().After(i.expiresAt)
}
