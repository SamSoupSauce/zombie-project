package physics

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestBodyCreationAndDefaults(t *testing.T) {
	pos := rl.NewVector3(1.0, 2.0, 3.0)
	extents := rl.NewVector3(0.5, 0.5, 0.5)
	b := NewBody("b1", "TestBody", pos, extents, 20.0)

	if b.Position != pos {
		t.Errorf("expected position %v, got %v", pos, b.Position)
	}
	if b.Mass != 20.0 {
		t.Errorf("expected mass 20.0, got %f", b.Mass)
	}
	if math.Abs(float64(b.InvMass-0.05)) > 0.0001 {
		t.Errorf("expected invMass 0.05, got %f", b.InvMass)
	}
	if b.Type != BodyDynamic {
		t.Errorf("expected BodyDynamic type")
	}

	box := b.GetBoundingBox()
	if box.Min.X != 0.5 || box.Max.X != 1.5 {
		t.Errorf("expected AABB X bounds [0.5, 1.5], got [%f, %f]", box.Min.X, box.Max.X)
	}
}

func TestSetMassAndStatic(t *testing.T) {
	b := NewBody("b1", "TestBody", rl.NewVector3(0, 0, 0), rl.NewVector3(1, 1, 1), 10.0)

	b.SetStatic(true)
	if b.Type != BodyStatic {
		t.Errorf("expected BodyStatic after SetStatic(true)")
	}
	if b.InvMass != 0 {
		t.Errorf("expected InvMass 0 for static body")
	}

	b.SetStatic(false)
	if b.Type != BodyDynamic {
		t.Errorf("expected BodyDynamic after SetStatic(false)")
	}
	if b.InvMass == 0 {
		t.Errorf("expected positive InvMass for dynamic body")
	}

	b.SetMass(0.0)
	if b.Type != BodyStatic {
		t.Errorf("setting mass 0 should make body static")
	}
}

func TestApplyForceAndImpulse(t *testing.T) {
	b := NewBody("b1", "TestBody", rl.NewVector3(0, 0, 0), rl.NewVector3(1, 1, 1), 10.0)

	// Apply force of 100N in X for 0.1s -> acc = 10 m/s^2 -> deltaV = 1 m/s
	b.ApplyForce(rl.NewVector3(100.0, 0, 0), 0.1)
	if math.Abs(float64(b.Velocity.X-1.0)) > 0.001 {
		t.Errorf("expected Velocity.X ~1.0, got %f", b.Velocity.X)
	}

	// Apply impulse of 50 N*s in Z -> deltaV = 5 m/s
	b.ApplyImpulse(rl.NewVector3(0, 0, 50.0))
	if math.Abs(float64(b.Velocity.Z-5.0)) > 0.001 {
		t.Errorf("expected Velocity.Z ~5.0, got %f", b.Velocity.Z)
	}
}

func TestGravityAndGroundCollision(t *testing.T) {
	world := NewWorld(-2.0)
	world.Gravity = rl.NewVector3(0, -10.0, 0)

	// Spawn cube of half-size 0.5 at Y = 1.0 (bottom is at Y = 0.5)
	b := NewBody("cube_drop", "FallingCube", rl.NewVector3(0, 1.0, 0), rl.NewVector3(0.5, 0.5, 0.5), 15.0)
	b.Restitution = 0.3
	world.AddBody(b)

	// Simulate 3 seconds of falling, bouncing, and settling
	for i := 0; i < 180; i++ {
		world.Step(1.0 / 60.0)
	}

	// Target resting position: GroundY + HalfExtents.Y = -2.0 + 0.5 = -1.5
	expectedY := float32(-1.5)
	if math.Abs(float64(b.Position.Y-expectedY)) > 0.05 {
		t.Errorf("expected cube resting at Y=%f, got %f", expectedY, b.Position.Y)
	}
	if !b.IsGrounded {
		t.Errorf("expected cube to be marked IsGrounded after landing")
	}
}

func TestGroundFrictionDeceleration(t *testing.T) {
	world := NewWorld(-2.0)
	// Place cube resting on the ground with initial horizontal slide velocity
	b := NewBody("slider", "SlidingCube", rl.NewVector3(0, -1.5, 0), rl.NewVector3(0.5, 0.5, 0.5), 10.0)
	b.Velocity = rl.NewVector3(8.0, 0, 0)
	b.Friction = 0.8
	world.AddBody(b)

	// Step world for 1.5 seconds
	for i := 0; i < 90; i++ {
		world.Step(1.0 / 60.0)
	}

	if b.Velocity.X != 0 {
		t.Errorf("expected sliding velocity to decelerate to 0, got %f", b.Velocity.X)
	}
}

func TestAABBOverlapAndPenetration(t *testing.T) {
	boxA := rl.NewBoundingBox(rl.NewVector3(0, 0, 0), rl.NewVector3(2, 2, 2))
	boxB := rl.NewBoundingBox(rl.NewVector3(1.5, 0, 0), rl.NewVector3(3.5, 2, 2))
	boxC := rl.NewBoundingBox(rl.NewVector3(5, 5, 5), rl.NewVector3(7, 7, 7))

	if !CheckAABBOverlap(boxA, boxB) {
		t.Errorf("expected boxA and boxB to overlap")
	}
	if CheckAABBOverlap(boxA, boxC) {
		t.Errorf("expected boxA and boxC to NOT overlap")
	}

	// Overlap between A and B along X should be 0.5
	normal, depth := ComputeAABBOverlap(boxA, boxB)
	if math.Abs(float64(depth-0.5)) > 0.01 {
		t.Errorf("expected penetration depth 0.5, got %f", depth)
	}
	// Normal to push A out of B along X should be -1
	if normal.X != -1.0 {
		t.Errorf("expected normal.X == -1.0, got %f", normal.X)
	}
}

func TestBodyVsBodyCollision(t *testing.T) {
	world := NewWorld(-2.0)

	// Two cubes close together moving directly towards each other
	b1 := NewBody("c1", "Cube1", rl.NewVector3(-0.8, -1.5, 0), rl.NewVector3(0.5, 0.5, 0.5), 10.0)
	b1.Velocity = rl.NewVector3(5.0, 0, 0)
	b1.Restitution = 0.5
	b1.Friction = 0.0 // Isolate linear impulse test
	world.AddBody(b1)

	b2 := NewBody("c2", "Cube2", rl.NewVector3(0.8, -1.5, 0), rl.NewVector3(0.5, 0.5, 0.5), 10.0)
	b2.Velocity = rl.NewVector3(-5.0, 0, 0)
	b2.Restitution = 0.5
	b2.Friction = 0.0
	world.AddBody(b2)

	// Step until they collide
	collided := false
	for i := 0; i < 30; i++ {
		world.Step(1.0 / 60.0)
		if b1.Velocity.X < 0 && b2.Velocity.X > 0 {
			collided = true
			break
		}
	}

	if !collided {
		t.Errorf("expected bodies to collide and rebound in opposite directions")
	}
}

func TestSolidObstacleCollision(t *testing.T) {
	world := NewWorld(-2.0)

	// Solid wall obstacle at X: 2.0 to 4.0
	wall := rl.NewBoundingBox(rl.NewVector3(2.0, -2.0, -2.0), rl.NewVector3(4.0, 2.0, 2.0))
	world.SetSolidBoxes([]rl.BoundingBox{wall})

	// Cube moving towards the wall
	b := NewBody("mover", "MovingCube", rl.NewVector3(0.0, -1.5, 0), rl.NewVector3(0.5, 0.5, 0.5), 10.0)
	b.Velocity = rl.NewVector3(6.0, 0, 0)
	b.Restitution = 0.4
	world.AddBody(b)

	for i := 0; i < 60; i++ {
		world.Step(1.0 / 60.0)
	}

	// Cube should have bounced off the wall and be moving left (negative X)
	if b.Position.X > 1.6 {
		t.Errorf("cube should not have penetrated past wall at X=2.0 (got %f)", b.Position.X)
	}
	if b.Velocity.X > 0 {
		t.Errorf("cube should have reflected velocity (got %f)", b.Velocity.X)
	}
}

func TestThrowCubeTrajectoryAndGravity(t *testing.T) {
	world := NewWorld(-2.2)
	world.Gravity = rl.NewVector3(0, -18.0, 0)

	// Throw cube from Y = -0.7 (eye height) with 15 m/s forward (+Z) and 4 m/s upward (+Y)
	b := NewBody("thrown_cube", "ThrownRainbowCube", rl.NewVector3(0, -0.7, 0), rl.NewVector3(0.5, 0.5, 0.5), 25.0)
	b.Velocity = rl.NewVector3(0, 4.0, 15.0)
	b.AngularVelocity = rl.NewVector3(120.0, 45.0, 60.0)
	b.Restitution = 0.35
	world.AddBody(b)

	// In the first 10 steps (ascending under upward velocity and gravity)
	for i := 0; i < 10; i++ {
		world.Step(1.0 / 60.0)
	}

	// Y velocity should have decreased due to gravity (-18 m/s^2)
	if b.Velocity.Y >= 4.0 {
		t.Errorf("expected gravity to decelerate upward velocity, got %f", b.Velocity.Y)
	}
	if b.Position.Z <= 0 {
		t.Errorf("expected cube to travel forward in Z, got %f", b.Position.Z)
	}

	// Step another 2 seconds to ensure it lands, bounces, and rests on ground
	for i := 0; i < 120; i++ {
		world.Step(1.0 / 60.0)
	}

	// Resting on ground: ground (-2.2) + halfExtents.Y (0.5) = -1.7
	expectedY := float32(-1.7)
	if math.Abs(float64(b.Position.Y-expectedY)) > 0.05 {
		t.Errorf("expected thrown cube to land and rest at Y=%f, got %f", expectedY, b.Position.Y)
	}
	if !b.IsGrounded {
		t.Errorf("expected thrown cube to be grounded after settling")
	}
}
