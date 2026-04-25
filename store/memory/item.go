package memory

import "time"

// item is the internal record stored by MemoryStore for each key. value
// holds the user-supplied value as any; the typed view is reconstructed by
// the generic Cache[T] layer that wraps the store. expiresAt is the
// absolute expiration deadline; the zero value means the item never
// expires. tags are the labels associated with the value at Set time and
// drive the tag index used by DeleteByTag.
type item struct {
	value     any
	expiresAt time.Time
	tags      []string
}

// isExpired reports whether the item has a deadline that is already in the
// past. Items with a zero expiresAt are never considered expired.
func (i item) isExpired() bool {
	return !i.expiresAt.IsZero() && time.Now().After(i.expiresAt)
}
