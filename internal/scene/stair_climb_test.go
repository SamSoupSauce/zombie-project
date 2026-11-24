package scene

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/camera"
	"zombie-project/internal/tiled"
)

// TestWalkUpStairsWithoutJumping verifies that a player can walk up the two-flight
// staircase in the Conestoga house map simply by moving forward, without pressing jump.
func TestWalkUpStairsWithoutJumping(t *testing.T) {
	mapPath := "maps/conestoga.tmx"
	tmxMap, err := tiled.LoadTMX(mapPath)
	if err != nil {
		t.Fatalf("failed to load conestoga.tmx: %v", err)
	}

	map3d := tiled.NewMap3D(tmxMap, 1.0, -2.2)
	solidBoxes := map3d.GetSolidBoxes()

	// Player starts on the wood floor in front of the lower flight (Col 8, Row 7)
	// Col 8 worldX = 0.5, Row 7 worldZ = -0.5
	// Entrance to lower flight is at Z = -1.0
	// Top of upper flight is at Z = -3.0
	// Loft platform is at Z = -3.5 (Row 4)
	startPos := rl.NewVector3(0.5, -2.12, -0.6)

	testCases := []struct {
		name      string
		speed     float32
		dt        float32
		maxFrames int
	}{
		{"WalkSpeed_60FPS", 3.5, 0.016, 60},
		{"SprintSpeed_60FPS", 7.0, 0.016, 40},
		{"WalkSpeed_30FPS", 3.5, 0.033, 35},
		{"SprintSpeed_30FPS", 7.0, 0.033, 25},
		{"SlowWalk_120FPS", 2.0, 0.008, 180},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fps := camera.NewFPSController(startPos, -2.2)
			fps.Position.Y = -2.12
			fps.WalkSpeed = tc.speed
			frameDeltaZ := -tc.speed * tc.dt // Moving North (-Z)

			reachedLoft := false
			for frame := 0; frame < tc.maxFrames; frame++ {
				oldZ := fps.Position.Z
				fps.MoveWithCollisions(rl.NewVector3(0, 0, frameDeltaZ), tc.dt, solidBoxes)

				// Verify player does not get stuck in place before reaching the loft
				if fps.Position.Z > -3.0 && math.Abs(float64(fps.Position.Z-oldZ)) < 0.0001 {
					t.Fatalf("frame %d: player got stuck at Z: %f, Y: %f without jumping", frame, fps.Position.Z, fps.Position.Y)
				}

				// Check if player has reached the top loft platform (elevation ~ +0.08m)
				if fps.Position.Z <= -3.1 && fps.Position.Y >= -0.05 {
					reachedLoft = true
					break
				}
			}

			if !reachedLoft {
				t.Fatalf("player failed to reach loft platform without jumping: ended at Pos: %v, grounded: %v",
					fps.Position, fps.IsGrounded)
			}
			if !fps.IsGrounded {
				t.Errorf("expected player to be grounded on the loft platform")
			}
		})
	}
}

// TestWalkUpStairsDiagonallyWithoutJumping verifies diagonal approach into the staircase.
func TestWalkUpStairsDiagonallyWithoutJumping(t *testing.T) {
	mapPath := "maps/conestoga.tmx"
	tmxMap, err := tiled.LoadTMX(mapPath)
	if err != nil {
		t.Fatalf("failed to load conestoga.tmx: %v", err)
	}

	map3d := tiled.NewMap3D(tmxMap, 1.0, -2.2)
	solidBoxes := map3d.GetSolidBoxes()

	// Player starts slightly West at (0.35, -2.12, -0.6) and moves forward-right (North-East)
	startPos := rl.NewVector3(0.35, -2.12, -0.6)
	fps := camera.NewFPSController(startPos, -2.2)
	fps.Position.Y = -2.12

	const dt = float32(0.016)
	// Move North (-Z) with slight East (+X) drift toward center of staircase
	delta := rl.NewVector3(0.005, 0, -3.5*dt)

	reachedLoft := false
	for frame := 0; frame < 60; frame++ {
		fps.MoveWithCollisions(delta, dt, solidBoxes)
		if fps.Position.Z <= -3.1 && fps.Position.Y >= -0.05 {
			reachedLoft = true
			break
		}
	}

	if !reachedLoft {
		t.Fatalf("player failed to reach loft on diagonal walk: Pos: %v", fps.Position)
	}
}
