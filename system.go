package ecs

import (
	"reflect"
	"sync"
	"unsafe"
)

type SystemState uint8

const (
	SystemStateInvalid SystemState = iota
	SystemStateInit
	SystemStateStart
	SystemStatePause
	SystemStateUpdate
	SystemStateDestroy
	SystemStateDestroyed
)

const (
	SystemCustomEventInvalid CustomEventName = ""
	SystemCustomEventPause                   = "__internal__Pause"
	SystemCustomEventResume                  = "__internal__Resume"
	SystemCustomEventStop                    = "__internal__Stop"
)

type ISystem interface {
	Type() reflect.Type
	Order() Order
	World() IWorld
	Requirements() map[reflect.Type]IRequirement
	IsRequire(component IComponent) (IRequirement, bool)
	ID() int64
	Pause()
	Resume()
	Stop()

	GetUtility() IUtility
	pause()
	resume()
	stop()
	doSync(func(api *SystemApi))
	doAsync(func(api *SystemApi))
	eventsSyncExecute()
	eventsAsyncExecute()
	getPointer() unsafe.Pointer
	isRequire(componentType reflect.Type) (IRequirement, bool)
	setOrder(order Order)
	setRequirements(rqs ...IRequirement)
	getState() SystemState
	setState(state SystemState)
	setSecurity(isSafe bool)
	isThreadSafe() bool
	setExecuting(isExecuting bool)
	isExecuting() bool
	baseInit(world *ecsWorld, ins ISystem)
	getOptimizer() *OptimizerReporter
	getGetterCache() *GetterCache
}

type SystemObject interface {
	__SystemIdentification()
}

type SystemPointer[T SystemObject] interface {
	ISystem
	*T
}

type System[T SystemObject] struct {
	lock              sync.Mutex
	requirements      map[reflect.Type]IRequirement
	eventsSync        []func(api *SystemApi)
	eventsAsync       []func(api *SystemApi)
	getterCache       *GetterCache
	order             Order
	optimizerReporter *OptimizerReporter
	world             *ecsWorld
	utility           IUtility
	realType          reflect.Type
	state             SystemState
	isSafe            bool
	executing         bool
	id                int64
}

func (s System[T]) __SystemIdentification() {}

func (s *System[T]) instance() (sys ISystem) {
	(*iface)(unsafe.Pointer(&sys)).data = unsafe.Pointer(s)
	return
}

func (s *System[T]) rawInstance() *T {
	return (*T)(unsafe.Pointer(s))
}

func (s *System[T]) ID() int64 {
	if s.id == 0 {
		s.id = LocalUniqueID()
	}
	return s.id
}

func (s *System[T]) doSync(fn func(api *SystemApi)) {
	s.eventsSync = append(s.eventsSync, fn)
}

func (s *System[T]) doAsync(fn func(api *SystemApi)) {
	s.eventsAsync = append(s.eventsAsync, fn)
}

func (s *System[T]) eventsSyncExecute() {
	api := &SystemApi{sys: s}
	events := s.eventsSync
	s.eventsSync = make([]func(api *SystemApi), 0)
	for _, f := range events {
		f(api)
	}
	api.sys = nil
}

func (s *System[T]) eventsAsyncExecute() {
	api := &SystemApi{sys: s}
	events := s.eventsAsync
	s.eventsAsync = make([]func(api *SystemApi), 0)
	var err error
	for _, f := range events {
		err = TryAndReport(func() {
			f(api)
		})
		if err != nil {
			Log.Error(err)
		}
	}
	api.sys = nil
}

func (s *System[T]) SetRequirements(rqs ...IRequirement) {
	s.setRequirements(rqs...)
}

func (s *System[T]) isInitialized() bool {
	return s.state >= SystemStateInit
}

func (s *System[T]) setRequirements(rqs ...IRequirement) {
	if s.isInitialized() {
		return
	}
	if s.requirements == nil {
		s.requirements = map[reflect.Type]IRequirement{}
	}
	var typ reflect.Type
	for _, value := range rqs {
		typ = value.Type()
		s.requirements[typ] = value
		ComponentMeta.GetComponentMetaInfo(typ)
	}
}

func (s *System[T]) setSecurity(isSafe bool) {
	s.isSafe = isSafe
}
func (s *System[T]) isThreadSafe() bool {
	return s.isSafe
}

func (s *System[T]) SetUtility(utility IUtility) {
	s.utility = utility
	s.utility.setSystem(s)
	s.utility.setWorld(s.world)
}

func (s *System[T]) GetUtility() IUtility {
	return s.utility
}

func (s *System[T]) Pause() {
	s.doAsync(func(api *SystemApi) {
		api.Pause()
	})
}

func (s *System[T]) Resume() {
	s.doAsync(func(api *SystemApi) {
		api.Resume()
	})
}

func (s *System[T]) Stop() {
	s.doAsync(func(api *SystemApi) {
		api.Stop()
	})
}

func (s *System[T]) pause() {
	if s.getState() == SystemStateUpdate {
		s.setState(SystemStatePause)
	}
}

func (s *System[T]) resume() {
	if s.getState() == SystemStatePause {
		s.setState(SystemStateUpdate)
	}
}

func (s *System[T]) stop() {
	if s.getState() < SystemStateDestroy {
		s.setState(SystemStateDestroy)
	}
}

func (s *System[T]) getState() SystemState {
	return s.state
}

func (s *System[T]) setState(state SystemState) {
	s.state = state
}

func (s *System[T]) setExecuting(isExecuting bool) {
	s.executing = isExecuting
}

func (s *System[T]) isExecuting() bool {
	return s.executing
}

func (s *System[T]) Requirements() map[reflect.Type]IRequirement {
	return s.requirements
}

func (s *System[T]) IsRequire(com IComponent) (IRequirement, bool) {
	return s.isRequire(com.Type())
}

func (s *System[T]) isRequire(typ reflect.Type) (IRequirement, bool) {
	r, ok := s.requirements[typ]
	return r, ok
}

func (s *System[T]) baseInit(world *ecsWorld, ins ISystem) {
	s.requirements = map[reflect.Type]IRequirement{}
	s.eventsSync = make([]func(api *SystemApi), 0)
	s.eventsAsync = make([]func(api *SystemApi), 0)
	s.getterCache = NewGetterCache(len(s.requirements))

	if ins.Order() == OrderInvalid {
		s.setOrder(OrderDefault)
	}
	s.world = world

	if i, ok := ins.(InitReceiver); ok {
		err := TryAndReport(func() {
			i.Init()
		})
		if err != nil {
			Log.Error(err)
		}
	}

	s.state = SystemStateStart
}

func (s *System[T]) getPointer() unsafe.Pointer {
	return unsafe.Pointer(s)
}

func (s *System[T]) Type() reflect.Type {
	if s.realType == nil {
		s.realType = TypeOf[T]()
	}
	return s.realType
}

func (s *System[T]) setOrder(order Order) {
	if s.isInitialized() {
		return
	}

	s.order = order
}

func (s *System[T]) Order() Order {
	return s.order
}

func (s *System[T]) World() IWorld {
	return s.world
}

func (s *System[T]) GetEntityInfo(entity Entity) EntityInfo {
	return s.world.GetEntityInfo(entity)
}

// get optimizer
func (s *System[T]) getOptimizer() *OptimizerReporter {
	if s.optimizerReporter == nil {
		s.optimizerReporter = &OptimizerReporter{}
		s.optimizerReporter.init()
	}
	return s.optimizerReporter
}

func (s *System[T]) getGetterCache() *GetterCache {
	return s.getterCache
}
