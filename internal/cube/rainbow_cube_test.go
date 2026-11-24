package cube

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestRainbowCubeDefaults(t *testing.T) {
	c := NewRainbowCube(1.0)
	if c.Size != 1.0 {
		t.Errorf("expected size 1.0, got %f", c.Size)
	}
	if !c.AutoRotate {
		t.Errorf("expected auto rotate to be true by default")
	}
	if c.Mode != ModeFaceColors {
		t.Errorf("expected default mode ModeFaceColors, got %v", c.Mode)
	}
}

func TestRainbowCubeUpdate(t *testing.T) {
	c := NewRainbowCube(1.0)
	initialYaw := c.Rotation.Y
	dt := float32(0.1)

	c.Update(dt)

	expectedYaw := initialYaw + c.RotationSpeed.Y*dt
	if c.Rotation.Y != expectedYaw {
		t.Errorf("expected yaw %f, got %f", expectedYaw, c.Rotation.Y)
	}

	// Test pause
	c.ToggleAutoRotate()
	currentYaw := c.Rotation.Y
	c.Update(dt)
	if c.Rotation.Y != currentYaw {
		t.Errorf("expected yaw to remain %f when paused, got %f", currentYaw, c.Rotation.Y)
	}
}

func TestRainbowCubeModeCycle(t *testing.T) {
	c := NewRainbowCube(1.0)

	if c.Mode != ModeFaceColors {
		t.Errorf("expected initial mode ModeFaceColors")
	}

	c.CycleMode()
	if c.Mode != ModeVertexGradient {
		t.Errorf("expected mode ModeVertexGradient after 1 cycle, got %v", c.Mode)
	}

	c.CycleMode()
	if c.Mode != ModeChromaCycle {
		t.Errorf("expected mode ModeChromaCycle after 2 cycles, got %v", c.Mode)
	}

	c.CycleMode()
	if c.Mode != ModeFaceColors {
		t.Errorf("expected wrap around to ModeFaceColors after 3 cycles, got %v", c.Mode)
	}
}

func TestRainbowCubeSpeedAdjustment(t *testing.T) {
	c := NewRainbowCube(1.0)
	initialSpeedY := c.RotationSpeed.Y

	c.AdjustSpeed(2.0)
	if c.RotationSpeed.Y != initialSpeedY*2.0 {
		t.Errorf("expected speed %f, got %f", initialSpeedY*2.0, c.RotationSpeed.Y)
	}
}

func TestRainbowCubeRayIntersection(t *testing.T) {
	c := NewRainbowCubeWithConfig("cube_1", "TestCube", 1.0, rl.NewVector3(0, 0, 0), ModeFaceColors)
	c.Rotation = rl.NewVector3(0, 0, 0) // aligned with world axes

	// 1. Ray pointing straight at cube center from +Z
	hitRay := rl.Ray{
		Position:  rl.NewVector3(0, 0, 5),
		Direction: rl.NewVector3(0, 0, -1),
	}
	hit, dist := c.RayIntersects(hitRay)
	if !hit {
		t.Errorf("expected ray to hit cube")
	}
	if dist < 3.9 || dist > 4.1 {
		t.Errorf("expected hit distance ~4.0, got %f", dist)
	}

	// 2. Ray pointing away from cube
	missRay := rl.Ray{
		Position:  rl.NewVector3(0, 5, 5),
		Direction: rl.NewVector3(0, 1, 0),
	}
	hitMiss, _ := c.RayIntersects(missRay)
	if hitMiss {
		t.Errorf("expected ray to miss cube")
	}

	// 3. Test translated cube
	cTrans := NewRainbowCubeWithConfig("cube_2", "RightCube", 1.0, rl.NewVector3(3.0, 0, 0), ModeFaceColors)
	cTrans.Rotation = rl.NewVector3(0, 0, 0)
	rayRight := rl.Ray{
		Position:  rl.NewVector3(3.0, 0, 5),
		Direction: rl.NewVector3(0, 0, -1),
	}
	hitRight, distRight := cTrans.RayIntersects(rayRight)
	if !hitRight {
		t.Errorf("expected ray to hit translated cube")
	}
	if distRight < 3.9 || distRight > 4.1 {
		t.Errorf("expected hit distance ~4.0 on translated cube, got %f", distRight)
	}

	// 4. Test rotated cube (rotated 45 degrees around Y)
	cRot := NewRainbowCubeWithConfig("cube_rot", "RotCube", 1.0, rl.NewVector3(0, 0, 0), ModeFaceColors)
	cRot.Rotation = rl.NewVector3(0, 45, 0)
	hitRot, _ := cRot.RayIntersects(hitRay)
	if !hitRot {
		t.Errorf("expected ray to hit 45° rotated cube")
	}
}

func TestRainbowCubePhysicsIntegration(t *testing.T) {
	c := NewRainbowCubeWithConfig("cube_phys", "PhysicsCube", 0.75, rl.NewVector3(2.0, 1.0, -3.0), ModeFaceColors)

	if c.Body == nil {
		t.Fatalf("expected cube to have an initialized physics body")
	}
	if c.Body.Mass != 35.0 {
		t.Errorf("expected default mass 35.0, got %f", c.Body.Mass)
	}

	// Bounding box test
	box := c.GetBoundingBox()
	if box.Min.X != 1.25 || box.Max.X != 2.75 {
		t.Errorf("expected X bounds [1.25, 2.75], got [%f, %f]", box.Min.X, box.Max.X)
	}
	if box.Min.Y != 0.25 || box.Max.Y != 1.75 {
		t.Errorf("expected Y bounds [0.25, 1.75], got [%f, %f]", box.Min.Y, box.Max.Y)
	}

	// Velocity and impulse test
	c.SetVelocity(rl.NewVector3(1.0, 2.0, 3.0))
	vel := c.GetVelocity()
	if vel.X != 1.0 || vel.Y != 2.0 || vel.Z != 3.0 {
		t.Errorf("expected velocity (1,2,3), got (%f,%f,%f)", vel.X, vel.Y, vel.Z)
	}

	c.ApplyImpulse(rl.NewVector3(35.0, 0, 0)) // impulse 35 / mass 35 => +1.0 m/s
	if c.GetVelocity().X != 2.0 {
		t.Errorf("expected velocity.X 2.0 after impulse, got %f", c.GetVelocity().X)
	}

	// Physics properties
	c.SetRestitution(0.65)
	if c.GetRestitution() != 0.65 {
		t.Errorf("expected restitution 0.65, got %f", c.GetRestitution())
	}
	c.SetFriction(0.4)
	if c.GetFriction() != 0.4 {
		t.Errorf("expected friction 0.4, got %f", c.GetFriction())
	}

	// Holding behavior
	c.SetHeld(true)
	if !c.IsHeld() {
		t.Errorf("expected cube to be marked held")
	}
	if c.GetVelocity().X != 0 || c.GetVelocity().Y != 0 || c.GetVelocity().Z != 0 {
		t.Errorf("expected velocity zeroed when held")
	}

	// Position sync on update
	c.SetHeld(false)
	c.Body.Position = rl.NewVector3(5.0, 6.0, 7.0)
	c.Update(0.016)
	if c.Position.X != 5.0 || c.Position.Y != 6.0 || c.Position.Z != 7.0 {
		t.Errorf("expected cube position synced to body position (5,6,7), got (%f,%f,%f)", c.Position.X, c.Position.Y, c.Position.Z)
	}
}
