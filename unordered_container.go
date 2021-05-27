package ecs

import (
	"reflect"
	"unsafe"
)

type UnorderedContainer struct {
	buf  []byte
	len  int
	unit uintptr
	head uintptr
}

func NewContainer(size uintptr) *UnorderedContainer {
	return &UnorderedContainer{
		buf:  make([]byte, 0, size),
		len:  0,
		unit: size,
	}
}

func (p *UnorderedContainer) Add(pointer unsafe.Pointer) (int, unsafe.Pointer) {
	data := reflect.SliceHeader{
		Data: uintptr(pointer),
		Len:  int(p.unit),
		Cap:  int(p.unit),
	}
	p.buf = append(p.buf, *(*[]byte)(unsafe.Pointer(&data))...)
	p.head = (*reflect.SliceHeader)(unsafe.Pointer(&p.buf)).Data
	p.len += 1
	return p.len - 1, unsafe.Pointer(p.head + uintptr(p.len-1)*p.unit)
}

func (p *UnorderedContainer) Remove(idx int) {
	if idx < 0 || idx >= p.len {
		return
	}
	offsetDelete := p.head + uintptr(idx)*p.unit
	offsetEnd := p.head + uintptr(p.len)*p.unit
	copy(p.buf[offsetDelete:offsetDelete+uintptr(p.unit)], p.buf[offsetEnd:])
	p.buf = p.buf[:offsetEnd]
	p.len -= 1
}

func (p *UnorderedContainer) Get(idx int) unsafe.Pointer {
	if idx < 0 || idx >= p.len {
		return nil
	}
	return unsafe.Pointer(p.head + uintptr(idx)*p.unit)
}

func (p UnorderedContainer) GetIterator() *iterator {
	return &iterator{
		memberSize: p.unit,
		size:       p.len,
		index:      -1,
		head:       p.head,
	}
}
