package ecs

type Iterator[T ComponentObject, TP ComponentPointer[T]] interface {
	Begin() *T
	Val() *T
	Next() *T
	End() bool
	Empty() bool
}

type Iter[T ComponentObject, TP ComponentPointer[T]] struct {
	c      *Collection[T, TP]
	len    int
	offset int
	cur    *T
}

func EmptyIter[T ComponentObject, TP ComponentPointer[T]]() Iterator[T, TP] {
	return &Iter[T, TP]{}
}

func NewIterator[T ComponentObject, TP ComponentPointer[T]](collection *Collection[T, TP]) Iterator[T, TP] {
	iter := &Iter[T, TP]{
		c:      collection,
		len:    collection.Len(),
		offset: 0,
	}
	if iter.len != 0 {
		iter.cur = &(collection.data[0])
	}
	return iter
}

func (i *Iter[T, TP]) Empty() bool {
	if i.len == 0 {
		return true
	}
	return false
}

func (i *Iter[T, TP]) End() bool {
	if i.offset > i.len-1 || i.len == 0 {
		return true
	}
	return false
}

func (i *Iter[T, TP]) Begin() *T {
	if i.len != 0 {
		i.offset = 0
		i.cur = &(i.c.data[0])
	}
	return i.cur
}

func (i *Iter[T, TP]) Val() *T {
	return i.cur
}

func (i *Iter[T, TP]) Next() *T {
	i.offset++
	if !i.End() {
		i.cur = &(i.c.data[i.offset])
	}
	return i.cur
}
