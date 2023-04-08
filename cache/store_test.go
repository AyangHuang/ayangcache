package cache

import (
	"testing"
	"time"
)

func TestShareStore_Add_Get_Del(t *testing.T) {
	s := newShareStore()
	hashKey, conflict := KeyToHash("ayang")

	if ok := s.Add(hashKey, conflict, "ayangcache", time.Time{}); !ok {
		t.Fatalf("Add failed")
	}

	if ok := s.Add(hashKey, conflict, "ayangcache", time.Time{}); ok {
		t.Fatalf("Add failed")
	}

	if v, ok := s.Get(hashKey, conflict); !ok || "ayangcache" != v.(string) {
		t.Fatalf("Get failed")
	}

	if v, ok := s.Del(hashKey, conflict); !ok || "ayangcache" != v.(string) {
		t.Fatalf("Del failed")
	}

	if _, ok := s.Get(hashKey, conflict); ok {
		t.Fatalf("Del failed")
	}

	if _, ok := s.Get(hashKey, conflict); ok {
		t.Fatalf("expiration failed")
	}
}

func TestExpiration(t *testing.T) {
	s := newShareStore()
	hashKey, conflict := KeyToHash("ayang")

	if ok := s.Add(hashKey, conflict, "ayangcache", time.Now().Add(time.Second)); !ok {
		t.Fatalf("Add expiration failed")
	}

	if _, ok := s.Get(hashKey, conflict); !ok {
		t.Fatalf("Get expiration failed")
	}

	time.Sleep(time.Second)
	if _, ok := s.Get(hashKey, conflict); ok {
		t.Fatalf("Get expitation falied")
	}

}
