package physics

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// BodyType defines the mobility and response type of a rigid physics body.
type BodyType int

const (
	BodyDynamic BodyType = iota // Fully simulated with gravity, forces, and impulses
	BodyStatic                  // Immovable obstacle with infinite mass
	BodyKinematic               // Manually animated, moves without external forces
)

// Body represents a 3D rigid physics body with mass, linear velocity, angular velocity,
// bounding box extents, restitution, and surface friction.
type Body struct {
	ID              string
	Name            string
	Type            BodyType
	Position        rl.Vector3
	Velocity        rl.Vector3
	AngularVelocity rl.Vector3 // Rotational velocity in deg/s
	Mass            float32    // Mass in kg (> 0 for dynamic)
	InvMass         float32    // 1/Mass (0 for static)
	HalfExtents     rl.Vector3 // Half width (X), half height (Y), half depth (Z)
	Restitution     float32    // Bounciness coefficient [0.0 = no bounce, 1.0 = full bounce]
	Friction        float32    // Surface friction [0.0 = frictionless, 1.0 = rough]
	LinearDamping   float32    // Air resistance deceleration factor
	AngularDamping  float32    // Rotational deceleration factor
	UseGravity      bool       // Whether gravity accelerates this body
	IsGrounded      bool       // True when resting on the floor or an obstacle below
	IsHeld          bool       // True when carried by the player (skips physics step)
}

// NewBody creates a new dynamic physics body with default physical attributes.
func NewBody(id, name string, pos, halfExtents rl.Vector3, mass float32) *Body {
	b := &Body{
		ID:             id,
		Name:           name,
		Type:           BodyDynamic,
		Position:       pos,
		Velocity:       rl.NewVector3(0, 0, 0),
		AngularVelocity: rl.NewVector3(0, 0, 0),
		HalfExtents:    halfExtents,
		Restitution:    0.35, // Natural rubber/wood bounce
		Friction:       0.75, // Standard surface friction
		LinearDamping:  0.02, // Subtle air drag
		AngularDamping: 0.15, // Rolling resistance
		UseGravity:     true,
		IsGrounded:     false,
		IsHeld:         false,
	}
	b.SetMass(mass)
	return b
}

// SetMass updates the body's mass and calculates inverse mass.
// If mass <= 0, the body automatically becomes static.
func (b *Body) SetMass(mass float32) {
	if mass <= 0.0001 {
		b.Mass = 0
		b.InvMass = 0
		b.Type = BodyStatic
	} else {
		b.Mass = mass
		b.InvMass = 1.0 / mass
		if b.Type == BodyStatic {
			b.Type = BodyDynamic
		}
	}
}

// SetStatic sets whether this body is completely static (immovable).
func (b *Body) SetStatic(isStatic bool) {
	if isStatic {
		b.Type = BodyStatic
		b.InvMass = 0
		b.Velocity = rl.NewVector3(0, 0, 0)
		b.AngularVelocity = rl.NewVector3(0, 0, 0)
	} else {
		b.Type = BodyDynamic
		if b.Mass <= 0 {
			b.Mass = 20.0
		}
		b.InvMass = 1.0 / b.Mass
	}
}

// GetBoundingBox returns the 3D Axis-Aligned Bounding Box (AABB) in world space.
func (b *Body) GetBoundingBox() rl.BoundingBox {
	return rl.NewBoundingBox(
		rl.NewVector3(b.Position.X-b.HalfExtents.X, b.Position.Y-b.HalfExtents.Y, b.Position.Z-b.HalfExtents.Z),
		rl.NewVector3(b.Position.X+b.HalfExtents.X, b.Position.Y+b.HalfExtents.Y, b.Position.Z+b.HalfExtents.Z),
	)
}

// ApplyForce applies a continuous force (Newtons) over a time step dt.
func (b *Body) ApplyForce(force rl.Vector3, dt float32) {
	if b.Type != BodyDynamic || b.IsHeld || b.InvMass == 0 {
		return
	}
	acc := rl.Vector3Scale(force, b.InvMass*dt)
	b.Velocity = rl.Vector3Add(b.Velocity, acc)
	if force.Y > 0.01 {
		b.IsGrounded = false
	}
}

// ApplyImpulse imparts an instantaneous velocity change (J = F * dt).
func (b *Body) ApplyImpulse(impulse rl.Vector3) {
	if b.Type != BodyDynamic || b.IsHeld || b.InvMass == 0 {
		return
	}
	deltaV := rl.Vector3Scale(impulse, b.InvMass)
	b.Velocity = rl.Vector3Add(b.Velocity, deltaV)
	if impulse.Y > 0.01 {
		b.IsGrounded = false
	}
}

// GetSpeed returns the current scalar linear speed in m/s.
func (b *Body) GetSpeed() float32 {
	return float32(math.Sqrt(float64(b.Velocity.X*b.Velocity.X + b.Velocity.Y*b.Velocity.Y + b.Velocity.Z*b.Velocity.Z)))
}

// GetHorizontalSpeed returns the horizontal scalar speed in m/s.
func (b *Body) GetHorizontalSpeed() float32 {
	return float32(math.Sqrt(float64(b.Velocity.X*b.Velocity.X + b.Velocity.Z*b.Velocity.Z)))
}
