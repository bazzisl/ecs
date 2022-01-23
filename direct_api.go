package ecs

import (
	"errors"
	"fmt"
	"unsafe"
)

// runtime api

func RuntimeConfigure(config *RuntimeConfig) {
	Runtime.Configure(config)
}

func Run() {
	Runtime.run()
}

func Stop() {
	Runtime.stop()
}

func CreateWorld(config *WorldConfig) IWorld {
	return Runtime.newWorld(config)
}

func DestroyWorld(world IWorld) {
	Runtime.destroyWorld(world.(*ecsWorld))
}

func AddJob(job func(), hashKey ...uint32) {
	Runtime.addJob(job, hashKey...)
}

// world api

func WorldRun(world IWorld) {
	world.Run()
}

func GetWorldID(world IWorld) int64 {
	return world.GetID()
}

func GetWorldStatus(world IWorld) WorldStatus {
	return world.GetStatus()
}

func RegisterSystem[T SystemObject](world IWorld, order ...Order) {
	world.registerForT(new(T), order...)
}

func GetSystem[T SystemObject](w IWorld) (ISystem, bool) {
	return w.getSystem(TypeOf[T]())
}

func GetEntityInfo(world IWorld, entity Entity) *EntityInfo {
	return world.getEntityInfo(entity)
}

func AddFreeComponent[T FreeComponentObject, TP FreeComponentPointer[T]](world IWorld, component *T) {
	world.addFreeComponent(TP(component))
}

// entity api

func NewEntity(world IWorld) *EntityInfo {
	return newEntityInfo(world.(*ecsWorld))
}

func EntityDestroyByInfo(info *EntityInfo) {
	info.Destroy()
}

func EntityDestroy(world IWorld, entity Entity) {
	info := world.getEntityInfo(entity)
	if info != nil {
		info.Destroy()
	}
}

// system api

func AddRequireComponent[T ComponentObject](sys ISystem) {
	sys.setRequirementsByType(TypeOf[T]())
}

func NewPeripheralSystem[T PeripheralSystemObject, TP PeripheralSystemPointer[T]]() *T {
	var ins T
	psys := IPeripheralSystem(TP(&ins))
	psys.init()
	if i, ok := psys.(InitReceiver); ok {
		i.Init()
	}
	return &ins
}

func GetInterestedComponents[T ComponentObject, TP ComponentPointer[T]](sys ISystem, outError ...*error) Iterator[T, TP] {
	setError := func(format string, args ...interface{}) Iterator[T, TP] {
		if len(outError) > 0 {
			*(outError[0]) = errors.New(fmt.Sprintf(format, args...))
		}
		return EmptyIter[T, TP]()
	}
	if sys.getState() == SystemStateInvalid {
		return setError("must init system first")
	}
	typ := GetType[T]()
	if _, ok := sys.Requirements()[typ]; !ok {
		return setError("not require, typ:", typ)
	}
	if sys.World() == nil {
		return setError("world is nil")
	}
	c := sys.World().getComponents(typ)
	if c == nil {
		return EmptyIter[T, TP]()
	}
	return NewIterator(c.(*Collection[T, TP]))
}

func GetRelatedComponent[T ComponentObject](sys ISystem, entity *EntityInfo) *T {
	if entity == nil {
		return nil
	}
	typ := TypeOf[T]()
	isRequire := sys.isRequire(typ)
	if !isRequire {
		return nil
	}
	c := entity.getComponentByType(typ)
	return (*T)(unsafe.Pointer((*iface)(unsafe.Pointer(&c)).data))
}
