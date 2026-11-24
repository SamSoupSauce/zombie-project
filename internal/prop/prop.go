package prop

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Prop represents an interactive 3D physics object that has mass and can be picked up.
type Prop interface {
	GetID() string
	GetName() string
	GetPosition() rl.Vector3
	SetPosition(pos rl.Vector3)
	GetRotation() rl.Vector3
	SetRotation(rot rl.Vector3)
	GetSize() float32
	GetMass() float32     // Mass in kilograms (kg)
	SetMass(mass float32) // Mass in kilograms (kg)
	IsHeld() bool
	SetHeld(held bool)
	RayIntersects(ray rl.Ray) (bool, float32)
	Draw()
	Update(dt float32)
}
