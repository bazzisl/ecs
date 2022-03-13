package game

import "github.com/zllangct/ecs"

type MoveChange struct {
	ecs.FreeDisposableComponent[MoveChange, *MoveChange]
	Entity ecs.Entity
	V      int
	Dir    []int
}
