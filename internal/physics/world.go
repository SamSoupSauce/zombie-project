package physics

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// World manages the 3D physics simulation environment, including gravity,
// collision detection, contact resolution, and rigid-body integration.
type World struct {
	Gravity    rl.Vector3
	GroundY    float32
	Bodies     []*Body
	SolidBoxes []rl.BoundingBox
	ShowDebug  bool
}

// NewWorld initializes a new physics simulation world with ground plane and gravity.
func NewWorld(groundY float32) *World {
	return &World{
		Gravity:    rl.NewVector3(0, -18.0, 0), // Realistic game gravity (m/s^2)
		GroundY:    groundY,
		Bodies:     make([]*Body, 0, 32),
		SolidBoxes: make([]rl.BoundingBox, 0, 64),
		ShowDebug:  false,
	}
}

// AddBody registers a physics body into the simulation world.
func (w *World) AddBody(b *Body) {
	for _, existing := range w.Bodies {
		if existing.ID == b.ID {
			return
		}
	}
	w.Bodies = append(w.Bodies, b)
}

// RemoveBody removes a body by unique identifier.
func (w *World) RemoveBody(id string) {
	for i, b := range w.Bodies {
		if b.ID == id {
			w.Bodies = append(w.Bodies[:i], w.Bodies[i+1:]...)
			return
		}
	}
}

// GetBody looks up a registered physics body by ID.
func (w *World) GetBody(id string) *Body {
	for _, b := range w.Bodies {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// SetSolidBoxes updates the solid static environment collision boxes (e.g. from TMX map).
func (w *World) SetSolidBoxes(boxes []rl.BoundingBox) {
	w.SolidBoxes = boxes
}

// Step advances the physics world by time delta dt using sub-stepped integration.
func (w *World) Step(dt float32) {
	if dt <= 0 {
		return
	}
	// Clamp maximum frame delta to prevent physics explosion on lag spikes
	if dt > 0.05 {
		dt = 0.05
	}

	// 2 Sub-steps for high numerical stability and accurate collision resolution
	const subSteps = 2
	subDt := dt / float32(subSteps)

	for step := 0; step < subSteps; step++ {
		// 1. Integrate dynamic bodies
		for _, b := range w.Bodies {
			if b.Type != BodyDynamic || b.IsHeld {
				continue
			}

			// Apply gravity acceleration (always when airborne or ascending)
			if b.UseGravity && (!b.IsGrounded || b.Velocity.Y > 0.001) {
				b.Velocity = rl.Vector3Add(b.Velocity, rl.Vector3Scale(w.Gravity, subDt))
			}

			// Apply linear air drag
			dampingFactor := float32(math.Pow(float64(1.0-b.LinearDamping), float64(subDt*60.0)))
			b.Velocity.X *= dampingFactor
			b.Velocity.Z *= dampingFactor

			// Apply angular air drag
			angDampFactor := float32(math.Pow(float64(1.0-b.AngularDamping), float64(subDt*60.0)))
			b.AngularVelocity = rl.Vector3Scale(b.AngularVelocity, angDampFactor)

			// Integrate position
			b.Position = rl.Vector3Add(b.Position, rl.Vector3Scale(b.Velocity, subDt))

			// Ground plane collision (Y = GroundY)
			minGroundY := w.GroundY + b.HalfExtents.Y
			if b.Position.Y <= minGroundY {
				b.Position.Y = minGroundY

				// Vertical impact response
				if b.Velocity.Y < -0.4 {
					b.Velocity.Y = -b.Velocity.Y * b.Restitution
					// Angular roll perturbation on bounce
					b.AngularVelocity.X += b.Velocity.Z * 30.0
					b.AngularVelocity.Z -= b.Velocity.X * 30.0
				} else if b.Velocity.Y <= 0 {
					b.Velocity.Y = 0
					b.IsGrounded = true
				} else {
					// Moving upward (e.g. ascending from throw or leap)
					b.IsGrounded = false
				}

				// Ground surface friction
				groundFric := float32(math.Pow(float64(1.0-b.Friction*0.8), float64(subDt*60.0)))
				if groundFric < 0 {
					groundFric = 0
				}
				b.Velocity.X *= groundFric
				b.Velocity.Z *= groundFric

				// Low-speed resting threshold (anti-jitter)
				if math.Abs(float64(b.Velocity.X)) < 0.04 {
					b.Velocity.X = 0
				}
				if math.Abs(float64(b.Velocity.Z)) < 0.04 {
					b.Velocity.Z = 0
				}
			} else {
				b.IsGrounded = false
			}

			// Environment solid obstacles collision (TMX walls, pillars, barricades)
			for _, solidBox := range w.SolidBoxes {
				ResolveBodyVsAABB(b, solidBox)
			}
		}

		// 2. Inter-body pairwise collisions (Cube vs Cube)
		for i := 0; i < len(w.Bodies); i++ {
			for j := i + 1; j < len(w.Bodies); j++ {
				ResolveBodyVsBody(w.Bodies[i], w.Bodies[j])
			}
		}
	}
}

// DrawDebug renders 3D collision boxes and velocity direction vectors for all active bodies.
func (w *World) DrawDebug() {
	if !w.ShowDebug {
		return
	}

	// Draw static environment solid collision boxes (walls, floors, stairs, corners)
	for _, box := range w.SolidBoxes {
		rl.DrawBoundingBox(box, rl.NewColor(80, 180, 240, 160))
	}

	for _, b := range w.Bodies {
		box := b.GetBoundingBox()
		col := rl.NewColor(0, 220, 255, 230) // Dynamic in-air: bright cyan

		if b.IsHeld {
			col = rl.NewColor(255, 230, 40, 230) // Held: bright yellow
		} else if b.Type == BodyStatic {
			col = rl.NewColor(255, 120, 40, 230) // Static: orange
		} else if b.IsGrounded {
			col = rl.NewColor(80, 255, 140, 230) // Grounded: vibrant green
		}

		rl.DrawBoundingBox(box, col)

		// Draw velocity motion vector indicator if moving
		speed := b.GetSpeed()
		if speed > 0.15 {
			velEnd := rl.Vector3Add(b.Position, rl.Vector3Scale(b.Velocity, 0.25))
			rl.DrawLine3D(b.Position, velEnd, rl.NewColor(255, 80, 80, 240))
		}
	}
}
