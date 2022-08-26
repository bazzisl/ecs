package ecs

import (
	"reflect"
	"sync"
)

var ComponentMeta = &componentMeta{
	seq:   0,
	types: map[reflect.Type]uint16{},
}

func GetComponentMeta[T ComponentObject]() ComponentMetaInfo {
	return ComponentMeta.GetComponentMetaInfo(TypeOf[T]())
}

type ComponentMetaInfo struct {
	it uint16
}

type componentMeta struct {
	mu    sync.RWMutex
	seq   uint16
	types map[reflect.Type]uint16
}

func (c *componentMeta) GetComponentMetaInfo(typ reflect.Type) ComponentMetaInfo {
	it := uint16(0)
	c.mu.RLock()
	if v, ok := c.types[typ]; ok {
		it = v
		c.mu.RUnlock()
	} else {
		c.mu.RUnlock()
		c.mu.Lock()
		if v, ok = c.types[typ]; ok {
			it = v
		} else {
			c.seq++
			it = c.seq
		}
		c.mu.Unlock()
	}

	return ComponentMetaInfo{
		it: it,
	}
}
