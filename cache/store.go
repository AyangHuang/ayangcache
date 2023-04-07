package cache

import (
	"time"
)

type store interface {
	Get(uint64, uint64) (interface{}, bool)
	Add(uint64, uint64, interface{}, time.Time) bool
	Del(uint64, uint64) (uint64, interface{})
}
