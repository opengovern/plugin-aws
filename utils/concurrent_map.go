package util

import "sync"

type ConcurrentMap[K comparable, V any] struct {
	data  map[K]V
	mutex sync.RWMutex
}

func NewMap[K comparable, V any]() ConcurrentMap[K, V] {
	return ConcurrentMap[K, V]{
		data:  map[K]V{},
		mutex: sync.RWMutex{},
	}
}

func (cm *ConcurrentMap[K, V]) Set(key K, value V) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.data[key] = value
}

func (cm *ConcurrentMap[K, V]) Delete(key K) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.data, key)
}

func (cm *ConcurrentMap[K, V]) Get(key K) (V, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	v, ok := cm.data[key]
	return v, ok
}
