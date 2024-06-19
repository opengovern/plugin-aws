package util

import (
	"sync"
)

type ConcurrentMap[K comparable, V any] struct {
	data sync.Map
}

func NewMap[K comparable, V any]() ConcurrentMap[K, V] {
	return ConcurrentMap[K, V]{
		data: sync.Map{},
	}
}

func (cm *ConcurrentMap[K, V]) Set(key K, value V) {
	cm.data.Store(key, value)
}

func (cm *ConcurrentMap[K, V]) Delete(key K) {
	cm.data.Delete(key)
}

func (cm *ConcurrentMap[K, V]) Get(key K) (V, bool) {
	v, ok := cm.data.Load(key)
	if !ok {
		return *new(V), false
	}
	return v.(V), true
}

func (cm *ConcurrentMap[K, V]) Range(f func(key K, value V) bool) {
	cm.data.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}
