package camera

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestFPSControllerInit(t *testing.T) {
	spawn := rl.NewVector3(0, -2.2, 0)
	fps := NewFPSController(spawn, -2.2)

	if fps.Position != spawn {
		t.Errorf("expected spawn pos %v, got %v", spawn, fps.Position)
	}
	expectedEyeY := spawn.Y + fps.EyeHeight
	if math.Abs(float64(fps.Camera.Position.Y-expectedEyeY)) > 0.01 {
		t.Errorf("expected camera eye Y %f, got %f", expectedEyeY, fps.Camera.Position.Y)
	}
	if !fps.IsGrounded {
		t.Errorf("expected player to start grounded")
	}
}

func TestFPSControllerGravityAndGround(t *testing.T) {
	spawn := rl.NewVector3(0, 5.0, 0) // dropped in air
	fps := NewFPSController(spawn, -2.2)

	if fps.IsGrounded {
		t.Errorf("player in air should not be grounded")
	}

	// Simulate 1 second of falling
	for i := 0; i < 60; i++ {
		fps.Update(1.0/60.0, nil)
	}

	// Should have landed on ground (-2.2)
	if fps.Position.Y != -2.2 {
		t.Errorf("expected player to land at -2.2, got %f", fps.Position.Y)
	}
	if !fps.IsGrounded {
		t.Errorf("player on ground should be marked grounded")
	}
	if fps.Velocity.Y != 0 {
		t.Errorf("vertical velocity should be zeroed after landing, got %f", fps.Velocity.Y)
	}
}

func TestFPSControllerWallCollision(t *testing.T) {
	spawn := rl.NewVector3(0, -2.2, 0)
	fps := NewFPSController(spawn, -2.2)

	// Solid wall obstacle directly in front at X: 1.0 to 3.0, Z: -1.0 to 1.0
	wallBox := rl.NewBoundingBox(
		rl.NewVector3(1.0, -2.2, -1.0),
		rl.NewVector3(3.0, 0.0, 1.0),
	)

	// Attempt to move into the wall (newPos X = 1.2)
	attemptPos := rl.NewVector3(1.2, -2.2, 0.0)
	resolvedPos := fps.resolveWallCollisions(attemptPos, []rl.BoundingBox{wallBox})

	// X movement should be blocked back to fps.Position.X (0.0)
	if resolvedPos.X >= 1.0-fps.PlayerRadius {
		t.Errorf("expected wall collision to prevent moving into X:1.0, got X:%f", resolvedPos.X)
	}
}

func TestFPSControllerLookRay(t *testing.T) {
	spawn := rl.NewVector3(0, -2.2, 0)
	fps := NewFPSController(spawn, -2.2)
	fps.Yaw = 0.0
	fps.Pitch = 0.0

	ray := fps.GetLookRay()
	if math.Abs(float64(ray.Direction.Z-1.0)) > 0.01 {
		t.Errorf("expected forward look ray Z direction ~1.0, got %f", ray.Direction.Z)
	}

	// Turned 90 degrees right: should point along -X (screen right)
	fps.Yaw = float32(math.Pi / 2.0)
	ray90 := fps.GetLookRay()
	if math.Abs(float64(ray90.Direction.X+1.0)) > 0.01 {
		t.Errorf("expected right look ray X direction ~-1.0, got %f", ray90.Direction.X)
	}
}

func TestFPSControllerCursorHiding(t *testing.T) {
	spawn := rl.NewVector3(0, -2.2, 0)
	fps := NewFPSController(spawn, -2.2)

	// LockCursor should set IsLocked to true
	fps.LockCursor()
	if !fps.IsLocked {
		t.Errorf("expected IsLocked true after LockCursor")
	}
	if !fps.justLocked {
		t.Errorf("expected justLocked true after LockCursor")
	}
	if !fps.wasFocused {
		t.Errorf("expected wasFocused true after LockCursor")
	}

	// UnlockCursor should set IsLocked to false
	fps.UnlockCursor()
	if fps.IsLocked {
		t.Errorf("expected IsLocked false after UnlockCursor")
	}
	if fps.justLocked {
		t.Errorf("expected justLocked false after UnlockCursor")
	}
}

func TestFPSControllerHumanParent(t *testing.T) {
	spawn := rl.NewVector3(2.0, -2.2, 3.0)
	fps := NewFPSController(spawn, -2.2)

	if fps.Human == nil {
		t.Fatalf("FPSController must have Human model as parent object")
	}
	if fps.Human.Position != spawn {
		t.Errorf("expected Human.Position %v, got %v", spawn, fps.Human.Position)
	}

	// Update movement & sync
	fps.Yaw = 1.25
	fps.Pitch = 0.45
	fps.Update(0.016, nil)

	if fps.Human.Yaw != fps.Yaw {
		t.Errorf("expected Human.Yaw synced to %f, got %f", fps.Yaw, fps.Human.Yaw)
	}
	if fps.Human.Pitch != fps.Pitch {
		t.Errorf("expected Human.Pitch synced to %f, got %f", fps.Pitch, fps.Human.Pitch)
	}

	// Teleport sync
	teleportTarget := rl.NewVector3(15.0, 0.0, -10.0)
	fps.Teleport(teleportTarget)
	if fps.Human.Position != teleportTarget {
		t.Errorf("expected Human.Position synced after teleport to %v, got %v", teleportTarget, fps.Human.Position)
	}
}

func TestFPSControllerLedgeThreshold(t *testing.T) {
	spawn := rl.NewVector3(0, -2.2, 0)
	fps := NewFPSController(spawn, -2.2)

	if fps.StepHeight < 0.6 {
		t.Errorf("expected default StepHeight to be at least 0.6, got %f", fps.StepHeight)
	}

	// Concrete curb / ledge of height 0.45 (same as game's Concrete Curb)
	curbBox := rl.NewBoundingBox(
		rl.NewVector3(1.0, -2.2, -1.0),
		rl.NewVector3(3.0, -1.75, 1.0),
	)

	// Walk forward into the curb
	fps.MoveWithCollisions(rl.NewVector3(1.2, 0, 0), 0.016, []rl.BoundingBox{curbBox})

	if fps.Position.X < 1.0 {
		t.Errorf("expected player to walk across X:1.0 curb boundary, got X:%f", fps.Position.X)
	}
	if math.Abs(float64(fps.Position.Y-(-1.75))) > 0.01 {
		t.Errorf("expected player to step up onto curb at Y:-1.75, got Y:%f", fps.Position.Y)
	}
	if !fps.IsGrounded {
		t.Errorf("expected player to be grounded on top of curb")
	}
}

func TestFPSControllerStaircaseWalking(t *testing.T) {
	spawn := rl.NewVector3(0.5, -2.2, 0.0)
	fps := NewFPSController(spawn, -2.2)

	// 3-step staircase with 0.40m rise per step
	staircase := []rl.BoundingBox{
		rl.NewBoundingBox(rl.NewVector3(1.0, -2.2, -1.0), rl.NewVector3(2.0, -1.8, 1.0)),
		rl.NewBoundingBox(rl.NewVector3(2.0, -2.2, -1.0), rl.NewVector3(3.0, -1.4, 1.0)),
		rl.NewBoundingBox(rl.NewVector3(3.0, -2.2, -1.0), rl.NewVector3(4.0, -1.0, 1.0)),
	}

	// Step 1: walk onto step 1
	fps.MoveWithCollisions(rl.NewVector3(0.9, 0, 0), 0.016, staircase)
	if math.Abs(float64(fps.Position.Y-(-1.8))) > 0.01 {
		t.Fatalf("expected step 1 Y:-1.8, got %f", fps.Position.Y)
	}

	// Step 2: walk onto step 2
	fps.MoveWithCollisions(rl.NewVector3(1.0, 0, 0), 0.016, staircase)
	if math.Abs(float64(fps.Position.Y-(-1.4))) > 0.01 {
		t.Fatalf("expected step 2 Y:-1.4, got %f", fps.Position.Y)
	}

	// Step 3: walk onto step 3
	fps.MoveWithCollisions(rl.NewVector3(1.0, 0, 0), 0.016, staircase)
	if math.Abs(float64(fps.Position.Y-(-1.0))) > 0.01 {
		t.Fatalf("expected step 3 Y:-1.0, got %f", fps.Position.Y)
	}

	if !fps.IsGrounded {
		t.Errorf("expected player grounded at top of staircase")
	}
}

func TestFPSControllerStepHeightAdjustment(t *testing.T) {
	spawn := rl.NewVector3(0, -2.2, 0)
	fps := NewFPSController(spawn, -2.2)

	// Ledge of height 0.80 (higher than default 0.65 threshold)
	highLedge := rl.NewBoundingBox(
		rl.NewVector3(1.0, -2.2, -1.0),
		rl.NewVector3(3.0, -1.4, 1.0),
	)

	// Attempt to walk into high ledge with default StepHeight
	fps.MoveWithCollisions(rl.NewVector3(1.2, 0, 0), 0.016, []rl.BoundingBox{highLedge})
	if fps.Position.X >= 1.0-fps.PlayerRadius {
		t.Errorf("high ledge (0.80) should block player with default StepHeight 0.65, got X:%f", fps.Position.X)
	}

	// Now increase threshold to 0.85
	fps.SetStepHeight(0.85)
	if fps.GetStepHeight() != 0.85 {
		t.Errorf("expected GetStepHeight 0.85, got %f", fps.GetStepHeight())
	}

	// Move into the ledge again: should step up to -1.40!
	fps.MoveWithCollisions(rl.NewVector3(1.2, 0, 0), 0.016, []rl.BoundingBox{highLedge})
	if fps.Position.X < 1.0 {
		t.Errorf("expected player to step onto ledge after increasing threshold, got X:%f", fps.Position.X)
	}
	if math.Abs(float64(fps.Position.Y-(-1.4))) > 0.01 {
		t.Errorf("expected player at Y:-1.4, got %f", fps.Position.Y)
	}
}

func TestCoordinateCheck(t *testing.T) {
	cam := rl.Camera3D{
		Position:   rl.NewVector3(0, 0, 0),
		Target:     rl.NewVector3(0, 0, 1),
		Up:         rl.NewVector3(0, 1, 0),
		Fovy:       60.0,
		Projection: rl.CameraPerspective,
	}
	mat := rl.GetCameraMatrix(cam)
	// In OpenGL view matrix:
	// Row 0 is the camera's Right vector! (m0, m4, m8) or (m0, m1, m2)
	t.Logf("mat: m0=%.2f, m1=%.2f, m2=%.2f, m3=%.2f", mat.M0, mat.M1, mat.M2, mat.M3)
	t.Logf("mat: m4=%.2f, m5=%.2f, m6=%.2f, m7=%.2f", mat.M4, mat.M5, mat.M6, mat.M7)
	t.Logf("mat: m8=%.2f, m9=%.2f, m10=%.2f, m11=%.2f", mat.M8, mat.M9, mat.M10, mat.M11)
	t.Logf("mat: m12=%.2f, m13=%.2f, m14=%.2f, m15=%.2f", mat.M12, mat.M13, mat.M14, mat.M15)
}
