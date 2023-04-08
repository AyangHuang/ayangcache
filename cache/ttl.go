package cache

import "time"

type expiration interface {
	Add(uint64, uint64, time.Time)
	Clean(store, policy)
}
