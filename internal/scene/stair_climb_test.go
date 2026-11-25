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

// TestDemoSceneNoFloatingRainbowBoxes verifies that the 4 floating rainbow boxes
// previously hardcoded at origin and in the air have been eliminated from NewDemoScene.
func TestDemoSceneNoFloatingRainbowBoxes(t *testing.T) {
	scene := NewDemoScene()
	if scene == nil {
		t.Fatalf("failed to initialize NewDemoScene")
	}

	for _, c := range scene.Cubes {
		// None of the spawned props should be floating in the air at Y >= 0.0
		if c.Position.Y >= 0.0 {
			t.Errorf("found floating cube '%s' (ID: %s) at Y=%f", c.Name, c.ID, c.Position.Y)
		}
		// None of the old demo cube IDs should exist
		if c.ID == "cube_main" || c.ID == "cube_companion" || c.ID == "cube_mini" || c.ID == "cube_heavy" {
			t.Errorf("found old demo floating cube '%s' (ID: %s)", c.Name, c.ID)
		}
	}
}

// TestDemoSceneSecondFloorExploration verifies that the player can navigate the Conestoga house
// in NewDemoScene, climb the stairs, walk onto the 2nd floor, and explore it while grounded at Y ~ +0.08m.
func TestDemoSceneSecondFloorExploration(t *testing.T) {
	scene := NewDemoScene()
	if scene == nil {
		t.Fatalf("failed to initialize NewDemoScene")
	}
	if scene.Map3D == nil {
		t.Fatalf("NewDemoScene missing Map3D")
	}

	solidBoxes := scene.Map3D.GetSolidBoxes()
	fps := scene.FPS

	// Move player to the base of the stairs in the Conestoga house (Col 8, Row 7)
	fps.Position = rl.NewVector3(0.5, -2.12, -0.6)
	fps.Velocity = rl.NewVector3(0, 0, 0)
	fps.IsGrounded = true

	// 1. Walk North up stairs to 2nd floor landing (Z = -3.5)
	for frame := 0; frame < 40; frame++ {
		fps.MoveWithCollisions(rl.NewVector3(0, 0, -0.09), 0.016, solidBoxes)
	}

	if fps.Position.Y < -0.05 {
		t.Fatalf("player did not reach 2nd floor height: Y=%f (Z=%f)", fps.Position.Y, fps.Position.Z)
	}
	if !fps.IsGrounded {
		t.Fatalf("player not grounded on 2nd floor landing")
	}

	// 2. Walk West into 2nd floor room (Col 8 -> Col 6)
	for frame := 0; frame < 30; frame++ {
		fps.MoveWithCollisions(rl.NewVector3(-0.08, 0, 0), 0.016, solidBoxes)
	}

	if fps.Position.Y < -0.05 {
		t.Fatalf("player fell below 2nd floor walking West: Y=%f (X=%f)", fps.Position.Y, fps.Position.X)
	}
	if !fps.IsGrounded {
		t.Fatalf("player lost grounding on 2nd floor floor planks")
	}

	// 3. Walk South within 2nd floor room (Row 4 -> Row 6)
	for frame := 0; frame < 25; frame++ {
		fps.MoveWithCollisions(rl.NewVector3(0, 0, 0.08), 0.016, solidBoxes)
	}

	if fps.Position.Y < -0.05 {
		t.Fatalf("player fell below 2nd floor walking South: Y=%f (Z=%f)", fps.Position.Y, fps.Position.Z)
	}
	if !fps.IsGrounded {
		t.Fatalf("player should remain grounded on 2nd floor")
	}
}

// TestSecondFloorPerimeterWalkNoFallThrough verifies that a player walking along the entire perimeter
// of the 2nd floor (stepping onto wall-edge floor tiles and wall-corner floor tiles) remains fully supported
// at Y ~ +0.08m and never falls through the floor or ceiling to the ground floor.
func TestSecondFloorPerimeterWalkNoFallThrough(t *testing.T) {
	scene := NewDemoScene()
	if scene == nil || scene.Map3D == nil {
		t.Fatalf("failed to initialize demo scene")
	}

	solidBoxes := scene.Map3D.GetSolidBoxes()
	fps := scene.FPS

	// Start at 2nd floor landing (Col 8, Row 4)
	fps.Position = rl.NewVector3(0.5, 0.08, -3.5)
	fps.Velocity = rl.NewVector3(0, 0, 0)
	fps.IsGrounded = true

	// Step 1: Walk North into North wall tile (Col 8, Row 3)
	for i := 0; i < 20; i++ {
		fps.MoveWithCollisions(rl.NewVector3(0, 0, -0.06), 0.016, solidBoxes)
		if fps.Position.Y < -0.05 {
			t.Fatalf("player fell below 2nd floor walking North into wall tile: Y=%f (Z=%f)", fps.Position.Y, fps.Position.Z)
		}
	}

	// Step 2: Walk West along North wall edge tiles to NW Corner (Col 3, Row 3)
	for i := 0; i < 80; i++ {
		fps.MoveWithCollisions(rl.NewVector3(-0.08, 0, 0), 0.016, solidBoxes)
		if fps.Position.Y < -0.05 {
			t.Fatalf("player fell below 2nd floor walking West along North wall: Y=%f (X=%f, Z=%f)", fps.Position.Y, fps.Position.X, fps.Position.Z)
		}
	}

	// Step 3: Walk South along West wall edge tiles to SW Corner (Col 3, Row 7)
	for i := 0; i < 70; i++ {
		fps.MoveWithCollisions(rl.NewVector3(0, 0, 0.08), 0.016, solidBoxes)
		if fps.Position.Y < -0.05 {
			t.Fatalf("player fell below 2nd floor walking South along West wall: Y=%f (X=%f, Z=%f)", fps.Position.Y, fps.Position.X, fps.Position.Z)
		}
	}

	// Step 4: Walk East along South wall edge tiles to SE Corner (Col 9, Row 7)
	for i := 0; i < 90; i++ {
		fps.MoveWithCollisions(rl.NewVector3(0.08, 0, 0), 0.016, solidBoxes)
		if fps.Position.Y < -0.05 {
			t.Fatalf("player fell below 2nd floor walking East along South wall: Y=%f (X=%f, Z=%f)", fps.Position.Y, fps.Position.X, fps.Position.Z)
		}
	}

	// Step 5: Walk North along East wall edge tiles to NE Corner (Col 9, Row 3)
	for i := 0; i < 70; i++ {
		fps.MoveWithCollisions(rl.NewVector3(0, 0, -0.08), 0.016, solidBoxes)
		if fps.Position.Y < -0.05 {
			t.Fatalf("player fell below 2nd floor walking North along East wall: Y=%f (X=%f, Z=%f)", fps.Position.Y, fps.Position.X, fps.Position.Z)
		}
	}

	if fps.Position.Y < -0.05 {
		t.Fatalf("player ended perimeter walk below 2nd floor: Y=%f", fps.Position.Y)
	}
	if !fps.IsGrounded {
		t.Fatalf("player should remain grounded throughout 2nd floor perimeter walk")
	}
}
