package physics

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// CheckAABBOverlap returns true if two 3D Axis-Aligned Bounding Boxes intersect.
func CheckAABBOverlap(a, b rl.BoundingBox) bool {
	return a.Min.X < b.Max.X && a.Max.X > b.Min.X &&
		a.Min.Y < b.Max.Y && a.Max.Y > b.Min.Y &&
		a.Min.Z < b.Max.Z && a.Max.Z > b.Min.Z
}

// ComputeAABBOverlap calculates the minimum penetration depth and separation normal
// pointing from box B to box A (direction to push A out of B).
func ComputeAABBOverlap(a, b rl.BoundingBox) (rl.Vector3, float32) {
	dx1 := b.Max.X - a.Min.X // push A in +X
	dx2 := a.Max.X - b.Min.X // push A in -X
	overlapX := dx1
	normX := float32(1.0)
	if dx2 < dx1 {
		overlapX = dx2
		normX = -1.0
	}

	dy1 := b.Max.Y - a.Min.Y // push A in +Y
	dy2 := a.Max.Y - b.Min.Y // push A in -Y
	overlapY := dy1
	normY := float32(1.0)
	if dy2 < dy1 {
		overlapY = dy2
		normY = -1.0
	}

	dz1 := b.Max.Z - a.Min.Z // push A in +Z
	dz2 := a.Max.Z - b.Min.Z // push A in -Z
	overlapZ := dz1
	normZ := float32(1.0)
	if dz2 < dz1 {
		overlapZ = dz2
		normZ = -1.0
	}

	// Find the axis with the minimum penetration depth
	if overlapX <= overlapY && overlapX <= overlapZ {
		return rl.NewVector3(normX, 0, 0), overlapX
	} else if overlapY <= overlapX && overlapY <= overlapZ {
		return rl.NewVector3(0, normY, 0), overlapY
	} else {
		return rl.NewVector3(0, 0, normZ), overlapZ
	}
}

// ResolveBodyVsBody calculates contact resolution and impulse exchange between two bodies.
func ResolveBodyVsBody(b1, b2 *Body) bool {
	if (b1.Type == BodyStatic && b2.Type == BodyStatic) || b1.IsHeld || b2.IsHeld {
		return false
	}

	box1 := b1.GetBoundingBox()
	box2 := b2.GetBoundingBox()

	if !CheckAABBOverlap(box1, box2) {
		return false
	}

	// Normal points from b2 to b1
	normal, depth := ComputeAABBOverlap(box1, box2)
	if depth <= 0.0001 {
		return false
	}

	totalInvMass := b1.InvMass + b2.InvMass
	if totalInvMass <= 0 {
		return false
	}

	// 1. Position resolution (prevent sinking / penetration)
	const percent = float32(0.85)
	const slop = float32(0.005)
	penCorr := float32(math.Max(float64(depth-slop), 0.0)) / totalInvMass * percent
	separation := rl.Vector3Scale(normal, penCorr)

	if b1.Type == BodyDynamic && !b1.IsHeld {
		b1.Position = rl.Vector3Add(b1.Position, rl.Vector3Scale(separation, b1.InvMass))
	}
	if b2.Type == BodyDynamic && !b2.IsHeld {
		b2.Position = rl.Vector3Subtract(b2.Position, rl.Vector3Scale(separation, b2.InvMass))
	}

	// 2. Velocity impulse resolution
	relVel := rl.Vector3Subtract(b1.Velocity, b2.Velocity)
	velAlongNormal := rl.Vector3DotProduct(relVel, normal)

	// If separating already, do not apply restitution impulse
	if velAlongNormal > 0 {
		return true
	}

	e := float32(math.Min(float64(b1.Restitution), float64(b2.Restitution)))
	j := -(1.0 + e) * velAlongNormal / totalInvMass
	impulse := rl.Vector3Scale(normal, j)

	if b1.Type == BodyDynamic && !b1.IsHeld {
		b1.Velocity = rl.Vector3Add(b1.Velocity, rl.Vector3Scale(impulse, b1.InvMass))
	}
	if b2.Type == BodyDynamic && !b2.IsHeld {
		b2.Velocity = rl.Vector3Subtract(b2.Velocity, rl.Vector3Scale(impulse, b2.InvMass))
	}

	// 3. Friction impulse along tangent
	tangent := rl.Vector3Subtract(relVel, rl.Vector3Scale(normal, velAlongNormal))
	tanLen := rl.Vector3Length(tangent)
	if tanLen > 0.001 {
		tangent = rl.Vector3Scale(tangent, 1.0/tanLen)
		jt := -rl.Vector3DotProduct(relVel, tangent) / totalInvMass
		mu := (b1.Friction + b2.Friction) * 0.5

		// Coulomb's friction cone limit
		maxJt := j * mu
		if jt > maxJt {
			jt = maxJt
		} else if jt < -maxJt {
			jt = -maxJt
		}

		frictionImpulse := rl.Vector3Scale(tangent, jt)
		if b1.Type == BodyDynamic && !b1.IsHeld {
			b1.Velocity = rl.Vector3Add(b1.Velocity, rl.Vector3Scale(frictionImpulse, b1.InvMass))
		}
		if b2.Type == BodyDynamic && !b2.IsHeld {
			b2.Velocity = rl.Vector3Subtract(b2.Velocity, rl.Vector3Scale(frictionImpulse, b2.InvMass))
		}
	}

	// Check if body 1 is resting on top of body 2
	if normal.Y > 0.7 && b1.Velocity.Y <= 0.2 {
		b1.IsGrounded = true
	} else if normal.Y < -0.7 && b2.Velocity.Y <= 0.2 {
		b2.IsGrounded = true
	}

	return true
}

// ResolveBodyVsAABB resolves collision between a dynamic body and a solid obstacle box.
func ResolveBodyVsAABB(b *Body, obstacle rl.BoundingBox) bool {
	if b.Type != BodyDynamic || b.IsHeld {
		return false
	}

	box := b.GetBoundingBox()
	if !CheckAABBOverlap(box, obstacle) {
		return false
	}

	normal, depth := ComputeAABBOverlap(box, obstacle)
	if depth <= 0.0001 {
		return false
	}

	// Push body out of obstacle
	b.Position = rl.Vector3Add(b.Position, rl.Vector3Scale(normal, depth))

	// Reflect velocity along collision normal
	vn := rl.Vector3DotProduct(b.Velocity, normal)
	if vn < 0 {
		e := b.Restitution
		if math.Abs(float64(vn)) < 0.4 {
			e = 0.0
		}
		b.Velocity = rl.Vector3Subtract(b.Velocity, rl.Vector3Scale(normal, (1.0+e)*vn))

		// Apply friction on tangential sliding
		if normal.Y > 0.7 { // Landed on top of solid obstacle
			b.IsGrounded = true
			b.Velocity.X *= (1.0 - b.Friction*0.5)
			b.Velocity.Z *= (1.0 - b.Friction*0.5)
		} else {
			// Wall collision friction
			b.Velocity.Y *= 0.85
		}
	}

	return true
}
