package ecs

import (
	"reflect"
	"sync"
	"unsafe"
)

type CollectionOperate uint8

const (
	CollectionOperateNone   CollectionOperate = iota
	CollectionOperateAdd                      //add component operation
	CollectionOperateDelete                   //delete component operation
)

type IComponentCollection interface {
	operate(op CollectionOperate, entity Entity, component IComponent)
	getTempTasks() []func()
	clearDisposable()
	getCollection(typ reflect.Type) IComponentSet
	getCollections() *SparseComponentCollection
}

type SparseComponentCollection struct {
	m       map[reflect.Type]int
	indices []int
	sets    []IComponentSet
}

func NewSparseComponentCollection() *SparseComponentCollection {
	return &SparseComponentCollection{
		m:       make(map[reflect.Type]int),
		indices: make([]int, 0),
		sets:    make([]IComponentSet, 0),
	}
}

func (s *SparseComponentCollection) Add(intType uint16, componentSet IComponentSet) {
	if intType >= uint16(len(s.indices)) {
		newIndices := make([]int, intType+1)
		copy(newIndices, s.indices)
		s.indices = newIndices
	}
	index := len(s.sets)
	s.m[componentSet.ElementType()] = index
	s.indices[intType] = index
	s.sets = append(s.sets, componentSet)
}

func (s *SparseComponentCollection) Get(intType uint16) IComponentSet {
	if intType >= uint16(len(s.indices)) {
		return nil
	}
	index := s.indices[intType]
	if index == -1 {
		return nil
	}
	return s.sets[index]
}

func (s *SparseComponentCollection) GetByType(typ reflect.Type) IComponentSet {
	index, ok := s.m[typ]
	if !ok {
		return nil
	}
	return s.sets[index]
}

type ComponentCollection struct {
	collections *SparseComponentCollection
	world       *ecsWorld
	bucket      int64
	locks       []sync.RWMutex
	opLog       []map[reflect.Type]*opTaskList
	once        []map[reflect.Type]struct{}
}

func NewComponentCollection(world *ecsWorld, k int) *ComponentCollection {
	cc := &ComponentCollection{
		world:       world,
		collections: NewSparseComponentCollection(),
	}

	for i := 1; ; i++ {
		if c := int64(1 << i); int64(k) < c {
			cc.bucket = c - 1
			break
		}
	}

	cc.locks = make([]sync.RWMutex, cc.bucket+1)
	for i := int64(0); i < cc.bucket+1; i++ {
		cc.locks[i] = sync.RWMutex{}
	}
	cc.opLog = make([]map[reflect.Type]*opTaskList, cc.bucket+1)
	cc.initOptTemp()
	cc.once = make([]map[reflect.Type]struct{}, cc.bucket+1)
	cc.initOnce()

	return cc
}

func (c *ComponentCollection) initOptTemp() {
	for index := range c.opLog {
		c.locks[index].Lock()
		c.opLog[index] = make(map[reflect.Type]*opTaskList)
		c.locks[index].Unlock()
	}
}

func (c *ComponentCollection) initOnce() {
	for index := range c.once {
		c.locks[index].Lock()
		c.once[index] = make(map[reflect.Type]struct{})
		c.locks[index].Unlock()
	}
}

func (c *ComponentCollection) operate(op CollectionOperate, entity Entity, component IComponent) {
	var hash int64
	switch component.getComponentType() {
	case ComponentTypeFree, ComponentTypeFreeDisposable:
		hash = int64((uintptr)(unsafe.Pointer(&hash))) & c.bucket
	case ComponentTypeNormal, ComponentTypeDisposable:
		hash = int64(entity) & c.bucket
	}

	typ := component.Type()
	newOpt := opTaskPool.Get()
	newOpt.target = entity
	newOpt.com = component
	newOpt.op = op

	b := c.opLog[hash]

	c.locks[hash].Lock()
	defer c.locks[hash].Unlock()

	tl, ok := b[typ]
	if !ok {
		tl = &opTaskList{}
		b[typ] = tl
	}

	tl.Append(newOpt)
}

func (c *ComponentCollection) clearDisposable() {
	for i := 0; i < len(c.once); i++ {
		c.locks[i].Lock()
		m := c.once[i]
		if len(m) > 0 {
			for typ, _ := range m {
				c.removeAllByType(typ)
				delete(m, typ)
				//Log.Info("collection Remove (Disposable):", typ.String())
			}
		}
		c.locks[i].Unlock()
	}
}

func (c *ComponentCollection) disposableTemp(com IComponent, typ reflect.Type) {
	var hash int64
	switch com.getComponentType() {
	case ComponentTypeFree, ComponentTypeFreeDisposable:
		hash = int64((uintptr)(unsafe.Pointer(&hash))) & c.bucket
	case ComponentTypeNormal, ComponentTypeDisposable:
		e := com.Owner()
		hash = int64(e) & c.bucket
	}
	c.locks[hash].Lock()
	defer c.locks[hash].Unlock()

	if _, ok := c.once[hash][typ]; !ok {
		c.once[hash][typ] = struct{}{}
	}
}

func (c *ComponentCollection) getTempTasks() []func() {
	combination := make(map[reflect.Type]*opTaskList)

	for i := 0; i < len(c.opLog); i++ {
		c.locks[i].RLock()
		for typ, list := range c.opLog[i] {
			if list.Len() == 0 {
				continue
			}
			if _, ok := combination[typ]; ok {
				combination[typ].Combine(list)
			} else {
				combination[typ] = list
			}
		}
		c.opLog[i] = make(map[reflect.Type]*opTaskList)
		c.locks[i].RUnlock()
	}

	var tasks []func()
	for typ, list := range combination {
		taskList := list
		set := c.collections.GetByType(typ)
		if set == nil {
			newSet := taskList.head.com.newCollection()
			c.collections.Add(newSet.GetElementMeta().it, newSet)
			set = c.collections.Get(newSet.GetElementMeta().it)
		}

		// 是否可以避免func封装？
		fn := func() {
			c.opExecute(taskList, set)
		}
		tasks = append(tasks, fn)
	}
	return tasks
}

func (c *ComponentCollection) opExecute(taskList *opTaskList, collection IComponentSet) {
	var t reflect.Type
	meta := collection.GetElementMeta()
	world := c.world
	for task := taskList.head; task != nil; task = task.next {
		task.com.setIntType(meta.it)
		task.com.setOwner(task.target)
		t = task.com.Type()
		switch task.op {
		case CollectionOperateAdd:
			task.com.addToCollection(collection.pointer())
			switch task.com.getComponentType() {
			case ComponentTypeNormal:
				//task.target.componentAdded(t, ret)
				//TODO channel并行block时间长
				//world.siblingCache.ch <- cacheOpPool.Get().(*CacheOp).Set(task.target, meta.it, uint8(1))
			case ComponentTypeDisposable:
				c.disposableTemp(task.com, t)
				//task.target.componentAdded(t, ret)
				//world.siblingCache.ch <- cacheOpPool.Get().(*CacheOp).Set(task.target, meta.it, uint8(1))
			case ComponentTypeFreeDisposable:
				c.disposableTemp(task.com, t)
			}
		case CollectionOperateDelete:
			task.com.deleteFromCollection(collection)
			switch task.com.getComponentType() {
			case ComponentTypeNormal, ComponentTypeDisposable:
				//task.target.componentDeleted(t, task.com.getComponentType())
				world.siblingCache.ch <- cacheOpPool.Get().(*CacheOp).Set(task.target, meta.it, uint8(2))
			}
		}
	}
	next := taskList.head
	for next != nil {
		task := next
		next = next.next
		opTaskPool.Put(task)
	}
	taskList.Reset()
}

func (c *ComponentCollection) getCollection(typ reflect.Type) IComponentSet {
	return c.collections.GetByType(typ)
}

func (c *ComponentCollection) getCollections() *SparseComponentCollection {
	return c.collections
}

func (c *ComponentCollection) removeAllByType(typ reflect.Type) {
	set := c.collections.GetByType(typ)
	if set == nil {
		return
	}
	set.Clear()
}
