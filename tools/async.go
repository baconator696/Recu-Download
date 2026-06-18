package tools

import "sync"

type RWLock[T ~string | ~bool | ~int] struct {
	data   T
	rwLock sync.RWMutex
}

func (self *RWLock[T]) ReadClone() T {
	var v T
	self.rwLock.RLock()
	v = self.data
	self.rwLock.RUnlock()
	return v
}
func (self *RWLock[T]) Write(data T) {
	self.rwLock.Lock()
	self.data = data
	self.rwLock.Unlock()
}

