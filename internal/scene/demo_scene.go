package scene

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/camera"
	"zombie-project/internal/console"
	"zombie-project/internal/cube"
	"zombie-project/internal/paths"
	"zombie-project/internal/physics"
	"zombie-project/internal/scripting"
	"zombie-project/internal/tiled"
	"zombie-project/internal/ui"
)

func init() {
	paths.EnsureProjectRoot()
}

type starParticle struct {
	Pos   rl.Vector3
	Size  float32
	Color rl.Color
}

// DemoScene coordinates the 3D first-person game engine with Lua scripting,
// interactive developer console, Tiled TMX map levels, object picking, and prop lifting.
type DemoScene struct {
	Camera              *camera.OrbitCamera
	FPS                 *camera.FPSController
	ViewMode            camera.ViewMode
	Cube                *cube.RainbowCube
	Cubes               []*cube.RainbowCube
	SelectedCube        *cube.RainbowCube
	SelectedTile        *tiled.Tile3D
	Map3D               *tiled.Map3D
	ShowMap             bool
	HUD                 *ui.HUD
	ScriptManager       *scripting.ScriptManager
	Console             *console.Console
	ShowGrid            bool
	InteractDistance    float32
	PickupNotice        string
	PickupNoticeTimer   float32
	PickupNoticeIsError bool
	particles           []starParticle
	PhysicsWorld        *physics.World
	ShowCollisionBoxes  bool
	summonCount         int
}

// SetPickupNotice displays a temporary toast alert on the HUD and in the console.
func (s *DemoScene) SetPickupNotice(msg string, isError bool) {
	s.PickupNotice = msg
	s.PickupNoticeTimer = 2.8
	s.PickupNoticeIsError = isError
}

// ThrowHeldProp throws the currently carried prop forward with physical impulse, trajectory, and tumbling.
func (s *DemoScene) ThrowHeldProp(power float32) {
	if s.FPS.Human == nil || s.FPS.Human.HeldProp == nil {
		return
	}

	thrown := s.FPS.Human.DropProp()
	if thrown == nil {
		return
	}

	cam := s.FPS.Camera
	lookDir := rl.Vector3Normalize(rl.Vector3Subtract(cam.Target, cam.Position))

	halfSize := thrown.GetSize()
	throwPos := rl.Vector3Add(cam.Position, rl.Vector3Scale(lookDir, halfSize+0.70))
	minGroundY := s.PhysicsWorld.GroundY + halfSize + 0.05
	if throwPos.Y < minGroundY {
		throwPos.Y = minGroundY
	}
	thrown.SetPosition(throwPos)
	thrown.SetRotation(rl.NewVector3(0, -s.FPS.Yaw*(180.0/math.Pi), 0))

	// Forward velocity + upward arc + player movement contribution
	throwVel := rl.Vector3Add(rl.Vector3Scale(lookDir, power), rl.NewVector3(0, power*0.22+1.0, 0))
	throwVel = rl.Vector3Add(throwVel, rl.Vector3Scale(s.FPS.Velocity, 0.65))

	if rb, ok := thrown.(*cube.RainbowCube); ok {
		if rb.Body != nil {
			rb.Body.Type = physics.BodyDynamic
			rb.Body.UseGravity = true
			rb.Body.IsHeld = false
			rb.Body.IsGrounded = false
			rb.Body.Velocity = throwVel

			// Impart tumbling angular spin as it flies through the 3D air
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			spinX := (r.Float32() - 0.5) * 120.0 + 160.0
			spinY := (r.Float32() - 0.5) * 140.0
			spinZ := (r.Float32() - 0.5) * 120.0
			rb.Body.AngularVelocity = rl.NewVector3(spinX, spinY, spinZ)
		}
		rb.SetVelocity(throwVel)
	}

	// Visual hand push thrust animation
	s.FPS.Human.TriggerThrow()

	msg := fmt.Sprintf("Threw %s (%.1f kg)!", thrown.GetName(), thrown.GetMass())
	s.SetPickupNotice(msg, false)
	s.Console.AddLine("=> "+msg, console.LineOutput)
}

// DropHeldProp releases the currently carried prop gently to the ground in front of player.
func (s *DemoScene) DropHeldProp() {
	if s.FPS.Human == nil || s.FPS.Human.HeldProp == nil {
		return
	}

	dropped := s.FPS.Human.DropProp()
	if dropped == nil {
		return
	}

	cam := s.FPS.Camera
	lookDir := rl.Vector3Normalize(rl.Vector3Subtract(cam.Target, cam.Position))

	halfSize := dropped.GetSize()
	dropPos := rl.Vector3Add(cam.Position, rl.Vector3Scale(lookDir, halfSize+0.60))
	minGroundY := s.PhysicsWorld.GroundY + halfSize + 0.05
	if dropPos.Y < minGroundY {
		dropPos.Y = minGroundY
	}
	dropped.SetPosition(dropPos)
	dropped.SetRotation(rl.NewVector3(0, -s.FPS.Yaw*(180.0/math.Pi), 0))

	tossVel := rl.Vector3Add(rl.Vector3Scale(lookDir, 2.8), rl.NewVector3(0, 1.2, 0))
	if rb, ok := dropped.(*cube.RainbowCube); ok {
		if rb.Body != nil {
			rb.Body.Type = physics.BodyDynamic
			rb.Body.UseGravity = true
			rb.Body.IsHeld = false
			rb.Body.IsGrounded = false
			rb.Body.Velocity = tossVel
			rb.Body.AngularVelocity = rl.NewVector3(20, 10, 15)
		}
		rb.SetVelocity(tossVel)
	}

	msg := fmt.Sprintf("Dropped %s (%.1f kg)", dropped.GetName(), dropped.GetMass())
	s.SetPickupNotice(msg, false)
	s.Console.AddLine("=> "+msg, console.LineInfo)
}

// DeleteCube removes the specified cube entity from the active scene, physics simulation, and player hands.
func (s *DemoScene) DeleteCube(c *cube.RainbowCube) (bool, string) {
	if c == nil {
		return false, "No entity to delete"
	}
	if c.IsDeleted {
		return false, fmt.Sprintf("Entity %s is already deleted", c.Name)
	}

	cName := c.Name
	cID := c.ID
	c.IsDeleted = true

	// 1. If carried by player, clear hands
	if s.FPS != nil && s.FPS.Human != nil && s.FPS.Human.HeldProp == c {
		s.FPS.Human.HeldProp = nil
	}

	// 2. Remove physics body from physics world
	if s.PhysicsWorld != nil && c.Body != nil {
		s.PhysicsWorld.RemoveBody(c.ID)
	}

	// 3. Remove from s.Cubes
	for i, existing := range s.Cubes {
		if existing == c || existing.ID == cID {
			s.Cubes = append(s.Cubes[:i], s.Cubes[i+1:]...)
			break
		}
	}

	// 4. Remove from s.Map3D.SpawnedCubes (if present)
	if s.Map3D != nil {
		for i, sc := range s.Map3D.SpawnedCubes {
			if sc == c || sc.ID == cID {
				s.Map3D.SpawnedCubes = append(s.Map3D.SpawnedCubes[:i], s.Map3D.SpawnedCubes[i+1:]...)
				break
			}
		}
	}

	// 5. Update selected cube and global Lua target
	if s.SelectedCube == c {
		if len(s.Cubes) > 0 {
			s.SelectedCube = s.Cubes[0]
			s.ScriptManager.SetGlobalTarget(s.SelectedCube)
		} else {
			s.SelectedCube = nil
			s.ScriptManager.ClearGlobalTarget()
		}
	}

	// 6. Update s.Cube (primary cube reference)
	if s.Cube == c {
		if len(s.Cubes) > 0 {
			s.Cube = s.Cubes[0]
			s.ScriptManager.SetMainCube(s.Cube)
		} else {
			s.Cube = nil
			s.ScriptManager.SetMainCube(nil)
		}
	}

	msg := fmt.Sprintf("Deleted entity: %s (%s)", cName, cID)
	s.SetPickupNotice(msg, false)
	s.Console.AddLine("=> "+msg, console.LineOutput)
	return true, msg
}

// DeleteTile removes the given map tile from Map3D and updates environment physics collisions.
func (s *DemoScene) DeleteTile(t *tiled.Tile3D) (bool, string) {
	if t == nil || s.Map3D == nil {
		return false, "No map tile to delete"
	}
	col := t.Col
	row := t.Row
	ok := s.Map3D.RemoveTileAt(col, row)
	if !ok {
		return false, fmt.Sprintf("Tile at (%d, %d) not found", col, row)
	}

	// Rebuild static obstacle bounding boxes for physics
	if s.PhysicsWorld != nil {
		s.PhysicsWorld.SetSolidBoxes(s.Map3D.GetSolidBoxes())
	}

	if s.SelectedTile == t {
		s.SelectedTile = nil
		s.ScriptManager.ClearGlobalTarget()
	}

	msg := fmt.Sprintf("Deleted map tile at (col: %d, row: %d)", col, row)
	s.SetPickupNotice(msg, false)
	s.Console.AddLine("=> "+msg, console.LineOutput)
	return true, msg
}

// SummonCube creates a new dynamic physics cube, places it into the scene and physics world,
// and sets it as the active selected entity.
func (s *DemoScene) SummonCube(cfg scripting.SummonConfig) *cube.RainbowCube {
	s.summonCount++
	id := fmt.Sprintf("cube_summon_%d", s.summonCount)
	name := cfg.Name
	if name == "" {
		name = fmt.Sprintf("Cube_%d", s.summonCount)
	}

	size := cfg.Size
	if size <= 0 {
		size = 0.75
	}

	mass := cfg.Mass
	if mass <= 0 {
		mass = 25.0
	}

	mode := cfg.Mode

	var pos rl.Vector3
	if cfg.Position != nil {
		pos = *cfg.Position
	} else if s.ViewMode == camera.ViewModeFirstPerson && s.FPS != nil {
		lookRay := s.FPS.GetLookRay()
		pos = rl.Vector3Add(lookRay.Position, rl.Vector3Scale(lookRay.Direction, 2.5))
		minY := s.FPS.GroundY + size + 0.05
		if pos.Y < minY {
			pos.Y = minY
		}
	} else if s.Camera != nil {
		pos = rl.Vector3Add(s.Camera.Target, rl.NewVector3(0, 1.2, 0))
	} else if s.FPS != nil {
		lookRay := s.FPS.GetLookRay()
		pos = rl.Vector3Add(lookRay.Position, rl.Vector3Scale(lookRay.Direction, 2.5))
	} else {
		pos = rl.NewVector3(0, 0, 0)
	}

	newCube := cube.NewRainbowCubeWithConfig(id, name, size, pos, mode)
	newCube.Mass = mass
	newCube.AutoRotate = false

	if s.ShowCollisionBoxes {
		newCube.ShowCollisionBox = true
	}

	if s.PhysicsWorld != nil && newCube.Body != nil {
		newCube.Body.SetMass(mass)
		newCube.Body.Type = physics.BodyDynamic
		newCube.Body.UseGravity = true
		newCube.Body.IsGrounded = false
		if cfg.Position == nil && s.ViewMode == camera.ViewModeFirstPerson && s.FPS != nil {
			lookRay := s.FPS.GetLookRay()
			newCube.Body.Velocity = rl.Vector3Scale(lookRay.Direction, 1.2)
		}
		s.PhysicsWorld.AddBody(newCube.Body)
	}

	s.Cubes = append(s.Cubes, newCube)
	if s.Cube == nil || s.Cube.IsDeleted {
		s.Cube = newCube
	}
	s.SelectedCube = newCube
	s.SelectedTile = nil

	notice := fmt.Sprintf("Summoned '%s' (ID: %s, Mass: %.1fkg) at (%.2f, %.2f, %.2f). Target set to new instance.",
		newCube.Name, newCube.ID, newCube.Mass, pos.X, pos.Y, pos.Z)
	s.SetPickupNotice(notice, false)
	s.Console.AddLine("=> "+notice, console.LineOutput)

	return newCube
}

// NewDemoScene creates and initializes the first-person 3D game scene.
func NewDemoScene() *DemoScene {
	paths.EnsureProjectRoot()
	// Primary central cube (35.0 kg: < 50kg, liftable)
	cubeMain := cube.NewRainbowCubeWithConfig("cube_1", "RainbowCube", 1.0, rl.NewVector3(0.0, 0.0, 0.0), cube.ModeVertexGradient)
	cubeMain.AutoRotate = false // Lua script manages rotation and dynamic hover
	cubeMain.Mass = 35.0

	// Secondary companion cube (24.0 kg: < 50kg, liftable)
	cubeCompanion := cube.NewRainbowCubeWithConfig("cube_2", "CompanionCube", 0.75, rl.NewVector3(3.4, 0.0, 0.0), cube.ModeFaceColors)
	cubeCompanion.RotationSpeed = rl.NewVector3(35.0, 30.0, 15.0)
	cubeCompanion.Mass = 24.0

	// Mini cube (12.0 kg: < 50kg, liftable)
	cubeMini := cube.NewRainbowCubeWithConfig("cube_3", "MiniCube", 0.6, rl.NewVector3(-3.4, 0.0, 0.0), cube.ModeChromaCycle)
	cubeMini.RotationSpeed = rl.NewVector3(20.0, 50.0, 35.0)
	cubeMini.Mass = 12.0

	// Heavy crate (160.0 kg: >= 50kg, demonstrates the strict < 50kg pickup rule!)
	cubeHeavy := cube.NewRainbowCubeWithConfig("cube_heavy", "HeavyCrate", 1.1, rl.NewVector3(0.0, -1.1, 4.5), cube.ModeFaceColors)
	cubeHeavy.AutoRotate = false
	cubeHeavy.Mass = 160.0

	cubes := []*cube.RainbowCube{cubeMain, cubeCompanion, cubeMini, cubeHeavy}

	cam := camera.NewOrbitCamera(rl.NewVector3(0.0, 0.0, 0.0), 10.0)

	sm := scripting.NewScriptManager(cubeMain, cam)
	sm.SetGlobalTarget(cubeMain)

	// Load default TMX map (Pioneer house map with dual-face walls, stairs, and loft)
	var map3d *tiled.Map3D
	tmxMap, err := tiled.LoadTMX("maps/conestoga.tmx")
	if err == nil {
		map3d = tiled.NewMap3D(tmxMap, 1.0, -2.2)
		sm.CurrentMap = map3d

		// Append map spawned cubes to the scene
		for _, sc := range map3d.SpawnedCubes {
			cubes = append(cubes, sc)
		}
	}

	// Determine First-Person Player Spawn Position
	playerSpawn := rl.NewVector3(0.0, -2.2, -4.5)
	if map3d != nil {
		for _, sp := range map3d.Spawns {
			if sp.Type == "player" {
				playerSpawn = rl.NewVector3(sp.Position.X, -2.2, sp.Position.Z)
				break
			}
		}
	}

	// Initialize First Person Controller at spawn position looking toward origin
	fps := camera.NewFPSController(playerSpawn, -2.2)
	dx := -playerSpawn.X
	dz := -playerSpawn.Z
	fps.Yaw = float32(math.Atan2(float64(-dx), float64(dz)))

	sm.SetFPSController(fps)

	// Load default behavior script
	_ = sm.LoadScript("scripts/cube_behavior.lua")

	con := console.NewConsole(sm)

	// Initialize 3D physics world with ground plane at Y = -2.2
	physicsWorld := physics.NewWorld(-2.2)
	if cubeMain.Body != nil {
		cubeMain.Body.Type = physics.BodyDynamic // Dynamic body with full gravity and throwing physics
		cubeMain.Body.UseGravity = true
		cubeMain.Body.IsGrounded = false
	}
	if cubeHeavy.Body != nil {
		cubeHeavy.Body.SetMass(160.0)
	}

	for _, c := range cubes {
		if c.Body != nil {
			physicsWorld.AddBody(c.Body)
		}
	}

	s := &DemoScene{
		Camera:              cam,
		FPS:                 fps,
		ViewMode:            camera.ViewModeFirstPerson, // Default First Person perspective!
		Cube:                cubeMain,
		Cubes:               cubes,
		SelectedCube:        cubeMain,
		Map3D:               map3d,
		ShowMap:             true,
		HUD:                 ui.NewHUD(),
		ScriptManager:       sm,
		Console:             con,
		ShowGrid:            true,
		InteractDistance:    3.5,
		PhysicsWorld:        physicsWorld,
		ShowCollisionBoxes:  false,
	}

	// Hook Lua toggle_physics_debug to scene collision box wireframes
	sm.OnTogglePhysicsDebug = func() bool {
		s.ShowCollisionBoxes = !s.ShowCollisionBoxes
		s.PhysicsWorld.ShowDebug = s.ShowCollisionBoxes
		for _, c := range s.Cubes {
			c.ShowCollisionBox = s.ShowCollisionBoxes
		}
		return s.ShowCollisionBoxes
	}

	// Hook Lua player.throw_prop
	sm.OnThrowProp = func(power float32) {
		s.ThrowHeldProp(power)
	}

	// Hook entity deletion on target (target:delete() / target.delete())
	sm.OnDeleteCube = func(c *cube.RainbowCube) (bool, string) {
		return s.DeleteCube(c)
	}
	sm.OnDeleteTile = func(t *tiled.Tile3D) (bool, string) {
		return s.DeleteTile(t)
	}

	// Hook entity summoning (summon() / spawn())
	sm.OnSummonCube = func(cfg scripting.SummonConfig) *cube.RainbowCube {
		return s.SummonCube(cfg)
	}

	// Register OnMapLoad callback to hot-switch maps
	sm.OnMapLoad = func(newMap *tiled.Map3D) error {
		// Clean up and dispose existing map and its props
		if s.Map3D != nil {
			for _, oldCube := range s.Map3D.SpawnedCubes {
				if oldCube == nil {
					continue
				}
				// 1. Drop if held in player hands
				if s.FPS != nil && s.FPS.Human != nil && s.FPS.Human.HeldProp == oldCube {
					s.FPS.Human.DropProp()
				}
				// 2. Remove physics body
				if s.PhysicsWorld != nil && oldCube.Body != nil {
					s.PhysicsWorld.RemoveBody(oldCube.ID)
				}
				oldCube.IsDeleted = true

				// 3. Remove from scene cubes
				for i, existing := range s.Cubes {
					if existing == oldCube || existing.ID == oldCube.ID {
						s.Cubes = append(s.Cubes[:i], s.Cubes[i+1:]...)
						break
					}
				}
			}

			// Clear selected tile from old map
			s.SelectedTile = nil

			// If current selected cube was deleted, update selection
			if s.SelectedCube != nil && s.SelectedCube.IsDeleted {
				if len(s.Cubes) > 0 {
					s.SelectedCube = s.Cubes[0]
					s.ScriptManager.SetGlobalTarget(s.SelectedCube)
				} else {
					s.SelectedCube = nil
					s.ScriptManager.ClearGlobalTarget()
				}
			}

			// Clear old collision boxes from physics world
			if s.PhysicsWorld != nil {
				s.PhysicsWorld.SetSolidBoxes(nil)
			}

			// Dispose of old map geometry
			s.Map3D.Dispose()
		}

		s.Map3D = newMap
		if s.PhysicsWorld != nil {
			s.PhysicsWorld.SetSolidBoxes(newMap.GetSolidBoxes())
		}
		for _, sc := range newMap.SpawnedCubes {
			found := false
			for _, existing := range s.Cubes {
				if existing.ID == sc.ID {
					found = true
					break
				}
			}
			if !found {
				s.Cubes = append(s.Cubes, sc)
				if sc.Body != nil {
					s.PhysicsWorld.AddBody(sc.Body)
				}
			}
		}
		if s.SelectedCube == nil && len(s.Cubes) > 0 {
			s.SelectedCube = s.Cubes[0]
			s.ScriptManager.SetGlobalTarget(s.SelectedCube)
		}
		// Teleport player to new map player spawn if present
		for _, sp := range newMap.Spawns {
			if sp.Type == "player" {
				s.FPS.Teleport(rl.NewVector3(sp.Position.X, -2.2, sp.Position.Z))
				dx := -sp.Position.X
				dz := -sp.Position.Z
				s.FPS.Yaw = float32(math.Atan2(float64(-dx), float64(dz)))
				break
			}
		}
		return nil
	}

	s.initParticles()
	return s
}

// Init sets up scene resources and ensures cursor is captured for first person view.
func (s *DemoScene) Init() {
	if s.ViewMode == camera.ViewModeFirstPerson && !s.Console.IsOpen {
		s.FPS.LockCursor()
	}
}

func (s *DemoScene) initParticles() {
	r := rand.New(rand.NewSource(42))
	s.particles = make([]starParticle, 100)
	for i := range s.particles {
		s.particles[i] = starParticle{
			Pos: rl.NewVector3(
				(r.Float32()-0.5)*36.0,
				(r.Float32()-0.5)*22.0,
				(r.Float32()-0.5)*36.0,
			),
			Size: 0.03 + r.Float32()*0.04,
			Color: rl.NewColor(
				uint8(180+r.Intn(75)),
				uint8(200+r.Intn(55)),
				255,
				uint8(90+r.Intn(120)),
			),
		}
	}
}

// Update handles first-person locomotion, mouse look, console interaction, and raycast picking.
func (s *DemoScene) Update(dt float32) {
	// Toggle Console on '`' (Grave / Tilde key)
	if rl.IsKeyPressed(rl.KeyGrave) {
		s.Console.Toggle()
		if s.Console.IsOpen {
			s.FPS.UnlockCursor()
		} else if s.ViewMode == camera.ViewModeFirstPerson {
			s.FPS.LockCursor()
		}
	}

	// Always advance console animations and input
	s.Console.Update(dt)

	// When console is open, check for mouse clicks on 3D objects to set global 'target'
	if s.Console.IsOpen {
		s.FPS.UnlockCursor()

		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			mousePos := rl.GetMousePosition()
			screenHeight := float32(rl.GetScreenHeight())
			consoleHeight := screenHeight * 0.55 * s.Console.AnimationProgress
			inputBarY := consoleHeight - 36.0

			// Only pick objects if click is not inside the bottom text input bar
			if mousePos.Y < inputBarY || mousePos.Y > consoleHeight {
				activeCam := s.GetCamera()
				ray := rl.GetScreenToWorldRay(mousePos, activeCam)

				var closestCube *cube.RainbowCube
				minDist := float32(math.MaxFloat32)

				for _, c := range s.Cubes {
					hit, dist := c.RayIntersects(ray)
					if hit && dist < minDist {
						minDist = dist
						closestCube = c
					}
				}

				if closestCube != nil {
					s.SelectedCube = closestCube
					s.SelectedTile = nil
					s.ScriptManager.SetGlobalTarget(closestCube)
					s.Console.AddLine(fmt.Sprintf("=> Selected target: %s (%s) | set global 'target'", closestCube.Name, closestCube.ID), console.LineOutput)
				} else if s.ShowMap && s.Map3D != nil {
					hitTile, _ := s.Map3D.RayIntersectsClosest(ray)
					if hitTile != nil {
						s.SelectedTile = hitTile
						s.SelectedCube = nil
						s.ScriptManager.SetGlobalTileTarget(hitTile)
						s.Console.AddLine(fmt.Sprintf("=> Map Tile: %s (Col:%d, Row:%d, GID:%d, Solid:%v) | set global 'target'",
							hitTile.Name, hitTile.Col, hitTile.Row, hitTile.GID, hitTile.IsSolid), console.LineInfo)
					}
				}
			}
		}
	} else {
		// When Console is closed:
		// Toggle perspective mode with [V] (First Person <-> Orbit)
		if rl.IsKeyPressed(rl.KeyV) {
			if s.ViewMode == camera.ViewModeFirstPerson {
				s.ViewMode = camera.ViewModeOrbit
				s.FPS.UnlockCursor()
			} else {
				s.ViewMode = camera.ViewModeFirstPerson
				s.FPS.LockCursor()
			}
		}

		if s.ViewMode == camera.ViewModeFirstPerson {
			// Extract accurate solid boxes for collision detection from Map3D (including stair steps and wall segments)
			var mapSolidBoxes []rl.BoundingBox
			if s.ShowMap && s.Map3D != nil {
				mapSolidBoxes = s.Map3D.GetSolidBoxes()
			}
			s.PhysicsWorld.SetSolidBoxes(mapSolidBoxes)

			// Player collides with solid map obstacles AND heavy/static cubes (>= 50kg)
			playerSolidBoxes := make([]rl.BoundingBox, len(mapSolidBoxes), len(mapSolidBoxes)+len(s.Cubes))
			copy(playerSolidBoxes, mapSolidBoxes)
			for _, c := range s.Cubes {
				if !c.IsHeld() && (c.Mass >= 50.0 || c.IsStatic()) {
					playerSolidBoxes = append(playerSolidBoxes, c.GetBoundingBox())
				}
			}

			// First-person locomotion (WASD, shift-sprint, space-jump, mouse look)
			s.FPS.Update(dt, playerSolidBoxes)

			// Dynamic player pushing of lighter dynamic cubes (< 50kg)
			playerPos := s.FPS.Position
			playerRadius := s.FPS.PlayerRadius
			for _, c := range s.Cubes {
				if c.IsHeld() || c.Mass >= 50.0 || c.IsStatic() || c.Body == nil {
					continue
				}
				cPos := c.Position
				dx := cPos.X - playerPos.X
				dz := cPos.Z - playerPos.Z
				distH := float32(math.Sqrt(float64(dx*dx + dz*dz)))
				minDist := playerRadius + c.EffectiveSize()
				if distH < minDist && playerPos.Y <= cPos.Y+c.EffectiveSize() && playerPos.Y+1.8 >= cPos.Y-c.EffectiveSize() {
					if distH > 0.001 {
						nx := dx / distH
						nz := dz / distH
						overlap := minDist - distH
						c.Body.Position.X += nx * overlap * 0.5
						c.Body.Position.Z += nz * overlap * 0.5
						pushSpeed := float32(3.4)
						c.Body.Velocity.X = nx * pushSpeed
						c.Body.Velocity.Z = nz * pushSpeed
					}
				}
			}

			// First-person click interaction along look crosshair
			if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				if s.FPS.Human != nil && s.FPS.Human.HeldProp != nil {
					// Left click throws the held cube forward with velocity and gravity!
					s.ThrowHeldProp(19.0)
				} else {
					lookRay := s.FPS.GetLookRay()
					var closestCube *cube.RainbowCube
					minDist := float32(math.MaxFloat32)

					for _, c := range s.Cubes {
						hit, dist := c.RayIntersects(lookRay)
						if hit && dist < minDist {
							minDist = dist
							closestCube = c
						}
					}

					if closestCube != nil {
						s.SelectedCube = closestCube
						s.ScriptManager.SetGlobalTarget(closestCube)
					}
				}
			}

			// Right click interaction: gentle toss / drop if holding a prop
			if rl.IsMouseButtonPressed(rl.MouseButtonRight) {
				if s.FPS.Human != nil && s.FPS.Human.HeldProp != nil {
					s.DropHeldProp()
				}
			}
		} else {
			// Orbit mode camera controls
			s.Camera.Update(dt)
		}

		// F5 manual hot reload
		if rl.IsKeyPressed(rl.KeyF5) {
			_ = s.ScriptManager.ReloadScript()
		}

		// [M] key toggles 3D Tiled map level rendering
		if rl.IsKeyPressed(rl.KeyM) {
			s.ShowMap = !s.ShowMap
		}

		// Keyboard shortcuts operate on the currently selected cube
		targetCube := s.SelectedCube
		if targetCube == nil {
			targetCube = s.Cube
		}

		// [E] interact key (pick up prop if < 50kg, or drop held prop)
		if rl.IsKeyPressed(rl.KeyE) {
			if s.FPS.Human != nil && s.FPS.Human.HeldProp != nil {
				s.DropHeldProp()
			} else {
				// Raycast from player crosshair to pick up prop
				lookRay := s.FPS.GetLookRay()
				var closestProp *cube.RainbowCube
				minDist := float32(math.MaxFloat32)

				for _, c := range s.Cubes {
					hit, dist := c.RayIntersects(lookRay)
					if hit && dist < minDist {
						minDist = dist
						closestProp = c
					}
				}

				if closestProp != nil {
					if minDist <= s.InteractDistance {
						ok, msg := s.FPS.Human.PickUp(closestProp)
						if ok {
							s.SelectedCube = closestProp
							s.ScriptManager.SetGlobalTarget(closestProp)
							s.SetPickupNotice(msg, false)
							s.Console.AddLine("=> "+msg, console.LineOutput)
						} else {
							s.SetPickupNotice(msg, true)
							s.Console.AddLine("! "+msg, console.LineError)
						}
					} else {
						s.SetPickupNotice(fmt.Sprintf("%s is too far away (%.1fm > %.1fm reach)",
							closestProp.GetName(), minDist, s.InteractDistance), true)
					}
				}
			}
		}

		if targetCube != nil {
			if s.ViewMode != camera.ViewModeFirstPerson && rl.IsKeyPressed(rl.KeySpace) {
				targetCube.ToggleAutoRotate()
			}
			if rl.IsKeyPressed(rl.KeyR) {
				targetCube.CycleMode()
			}
			if rl.IsKeyPressed(rl.KeyOne) {
				targetCube.SetMode(cube.ModeFaceColors)
			}
			if rl.IsKeyPressed(rl.KeyTwo) {
				targetCube.SetMode(cube.ModeVertexGradient)
			}
			if rl.IsKeyPressed(rl.KeyThree) {
				targetCube.SetMode(cube.ModeChromaCycle)
			}
			if rl.IsKeyPressed(rl.KeyW) && s.ViewMode != camera.ViewModeFirstPerson {
				targetCube.ToggleWireframe()
			}
		}
		// [G] key: Throw held prop forward with high velocity, or toggle floor grid
		if rl.IsKeyPressed(rl.KeyG) {
			if s.FPS.Human != nil && s.FPS.Human.HeldProp != nil {
				s.ThrowHeldProp(19.0)
			} else {
				s.ShowGrid = !s.ShowGrid
			}
		}

		// [B] key: Toggle physics collision box debug wireframes
		if rl.IsKeyPressed(rl.KeyB) {
			s.ShowCollisionBoxes = !s.ShowCollisionBoxes
			s.PhysicsWorld.ShowDebug = s.ShowCollisionBoxes
			for _, c := range s.Cubes {
				c.ShowCollisionBox = s.ShowCollisionBoxes
			}
			stateStr := "ENABLED"
			if !s.ShowCollisionBoxes {
				stateStr = "DISABLED"
			}
			s.Console.AddLine("=> Physics collision wireframes: "+stateStr, console.LineInfo)
		}
		if rl.IsKeyPressed(rl.KeyC) {
			if s.ViewMode == camera.ViewModeFirstPerson {
				s.FPS.Teleport(rl.NewVector3(0.0, -2.2, -4.5))
				s.FPS.Yaw = 0
				s.FPS.Pitch = 0
			} else {
				s.Camera.Reset()
			}
		}
		if rl.IsKeyPressed(rl.KeyH) {
			s.HUD.ToggleHelp()
		}
		if targetCube != nil {
			if rl.IsKeyPressed(rl.KeyEqual) || rl.IsKeyPressed(rl.KeyKpAdd) {
				targetCube.AdjustSpeed(1.25)
			}
			if rl.IsKeyPressed(rl.KeyMinus) {
				targetCube.AdjustSpeed(0.8)
			}
		}
	}

	// Update Pickup Notice timer
	if s.PickupNoticeTimer > 0 {
		s.PickupNoticeTimer -= dt
		if s.PickupNoticeTimer <= 0 {
			s.PickupNotice = ""
		}
	}

	// Update held prop position & orientation in player's hands
	if s.FPS.Human != nil && s.FPS.Human.HeldProp != nil {
		held := s.FPS.Human.HeldProp
		cam := s.FPS.Camera
		lookDir := rl.Vector3Normalize(rl.Vector3Subtract(cam.Target, cam.Position))
		rightDir := rl.Vector3Normalize(rl.Vector3CrossProduct(lookDir, cam.Up))
		camUp := rl.Vector3CrossProduct(rightDir, lookDir)

		heldPos := cam.Position
		heldPos = rl.Vector3Add(heldPos, rl.Vector3Scale(lookDir, 0.70))
		heldPos = rl.Vector3Add(heldPos, rl.Vector3Scale(camUp, -0.18+s.FPS.HeadBob))
		held.SetPosition(heldPos)

		yawDeg := s.FPS.Yaw * (180.0 / math.Pi)
		held.SetRotation(rl.NewVector3(0, -yawDeg, 0))
	}

	// Advance physics world simulation (sub-stepped gravity, restitution bouncing, friction, and cube collisions)
	if s.PhysicsWorld != nil {
		s.PhysicsWorld.Step(dt)
	}

	// World continues simulating in the background
	s.ScriptManager.Update(dt)
	for _, c := range s.Cubes {
		c.Update(dt)
	}
}

// Draw3D renders all 3D geometry in the scene.
func (s *DemoScene) Draw3D() {
	// Render background depth particles
	for _, p := range s.particles {
		rl.DrawCube(p.Pos, p.Size, p.Size, p.Size, p.Color)
	}

	// Floor grid & shadows
	if s.ShowGrid {
		gridY := float32(-2.2)

		// Draw floor grid at gridY
		rl.PushMatrix()
		rl.Translatef(0.0, gridY, 0.0)
		rl.DrawGrid(32, 0.5)
		rl.PopMatrix()

		// Ground shadows directly below each cube (unless carried in player hands)
		for _, c := range s.Cubes {
			if c.IsHeld() {
				continue
			}
			shadowColor := rl.NewColor(0, 0, 0, 80)
			effSize := c.EffectiveSize()
			rl.DrawCircle3D(rl.NewVector3(c.Position.X, gridY+0.01, c.Position.Z), 1.6*effSize, rl.NewVector3(1, 0, 0), 90.0, shadowColor)

			glowColor := rl.NewColor(30, 60, 90, 40)
			rl.DrawCircle3D(rl.NewVector3(c.Position.X, gridY+0.005, c.Position.Z), 2.8*effSize, rl.NewVector3(1, 0, 0), 90.0, glowColor)
		}
	}

	// Render imported Tiled TMX 3D Map
	if s.ShowMap && s.Map3D != nil {
		s.Map3D.Draw()

		// Highlight selected tile if any
		if s.SelectedTile != nil {
			st := s.SelectedTile
			wireSize := rl.NewVector3(st.Size.X*1.04, st.Size.Y*1.04, st.Size.Z*1.04)
			rl.DrawCubeWiresV(st.Position, wireSize, rl.NewColor(255, 220, 60, 255))
		}
	}

	// Render all cubes
	for _, c := range s.Cubes {
		c.Draw()
	}

	// Render glowing selection highlight around the active target
	if s.SelectedCube != nil && !s.SelectedCube.IsDeleted {
		sel := s.SelectedCube
		sSize := sel.EffectiveSize() * 1.06
		gridY := float32(-2.2)

		rl.PushMatrix()
		rl.Translatef(sel.Position.X, sel.Position.Y, sel.Position.Z)
		rl.Rotatef(sel.Rotation.Y, 0.0, 1.0, 0.0)
		rl.Rotatef(sel.Rotation.X, 1.0, 0.0, 0.0)
		rl.Rotatef(sel.Rotation.Z, 0.0, 0.0, 1.0)
		rl.DrawCubeWires(rl.NewVector3(0, 0, 0), sSize*2.0, sSize*2.0, sSize*2.0, rl.NewColor(80, 255, 140, 255))
		rl.PopMatrix()

		// Selection target ring on the floor
		if s.ShowGrid {
			rl.DrawCircle3D(
				rl.NewVector3(sel.Position.X, gridY+0.02, sel.Position.Z),
				sel.EffectiveSize()*1.8,
				rl.NewVector3(1, 0, 0),
				90.0,
				rl.NewColor(80, 255, 140, 160),
			)
		}
	}

	// Render Universal Human Stick Figure
	// First-Person Mode: renders visible hands with 5 articulating fingers and held-prop grasping
	// Orbit Mode: renders full 3D stick-figure body (head, eyes, smile, torso, arms, hands, legs, feet)
	if s.ViewMode == camera.ViewModeFirstPerson {
		s.FPS.DrawFirstPersonHands()
	} else {
		s.FPS.DrawThirdPerson()
	}

	// Render physics collision wireframes when debug mode is enabled
	if s.ShowCollisionBoxes && s.PhysicsWorld != nil {
		s.PhysicsWorld.DrawDebug()
	}
}

// Draw2D renders HUD, First-Person Crosshair, 3D target labels, and the developer console.
func (s *DemoScene) Draw2D() {
	activeCube := s.SelectedCube
	if activeCube == nil {
		activeCube = s.Cube
	}
	s.HUD.Draw(activeCube, s.Camera, s.ShowGrid, s.ScriptManager)

	screenWidth := int32(rl.GetScreenWidth())
	screenHeight := int32(rl.GetScreenHeight())
	centerX := screenWidth / 2
	centerY := screenHeight / 2

	// Render First-Person Reticle & Crosshair
	if s.ViewMode == camera.ViewModeFirstPerson && !s.Console.IsOpen {
		crosshairCol := rl.NewColor(240, 245, 255, 190)

		if s.FPS.Human != nil && s.FPS.Human.HeldProp != nil {
			held := s.FPS.Human.HeldProp
			heldText := fmt.Sprintf("Holding %s (%.1f kg) | [LMB / G] Throw  [RMB / E] Drop", held.GetName(), held.GetMass())
			textW := rl.MeasureText(heldText, 12)
			rl.DrawText(heldText, centerX-(textW/2), centerY+18, 12, rl.NewColor(255, 220, 80, 255))
		} else {
			// Check if crosshair is hovering over a targetable prop
			lookRay := s.FPS.GetLookRay()
			var hoveredCube *cube.RainbowCube
			minDist := float32(math.MaxFloat32)
			for _, c := range s.Cubes {
				if c.IsHeld() {
					continue
				}
				hit, dist := c.RayIntersects(lookRay)
				if hit && dist < minDist {
					minDist = dist
					hoveredCube = c
				}
			}

			if hoveredCube != nil {
				if minDist <= s.InteractDistance {
					if hoveredCube.Mass < 50.0 {
						crosshairCol = rl.NewColor(80, 255, 140, 255)
						hoverText := fmt.Sprintf("[E] Pick Up %s (%.1f kg)", hoveredCube.Name, hoveredCube.Mass)
						textW := rl.MeasureText(hoverText, 12)
						rl.DrawText(hoverText, centerX-(textW/2), centerY+18, 12, crosshairCol)
					} else {
						crosshairCol = rl.NewColor(255, 95, 80, 255)
						hoverText := fmt.Sprintf("[E] TOO HEAVY: %s (%.1f kg >= 50kg Limit)", hoveredCube.Name, hoveredCube.Mass)
						textW := rl.MeasureText(hoverText, 12)
						rl.DrawText(hoverText, centerX-(textW/2), centerY+18, 12, crosshairCol)
					}
				} else {
					crosshairCol = rl.NewColor(160, 215, 255, 200)
					hoverText := fmt.Sprintf("[%s (%.1f kg) - Reach: %.1fm]", hoveredCube.Name, hoveredCube.Mass, minDist)
					textW := rl.MeasureText(hoverText, 11)
					rl.DrawText(hoverText, centerX-(textW/2), centerY+18, 11, crosshairCol)
				}
			}
		}

		// Precision dot and crosshair brackets
		rl.DrawCircle(centerX, centerY, 2.5, crosshairCol)
		rl.DrawLine(centerX-9, centerY, centerX-3, centerY, crosshairCol)
		rl.DrawLine(centerX+3, centerY, centerX+9, centerY, crosshairCol)
		rl.DrawLine(centerX, centerY-9, centerX, centerY-3, crosshairCol)
		rl.DrawLine(centerX, centerY+3, centerX, centerY+9, crosshairCol)
	}

	// Render Pickup Notice Toast Banner
	if s.PickupNotice != "" && s.PickupNoticeTimer > 0 {
		noticeText := s.PickupNotice
		fontSize := int32(13)
		textW := rl.MeasureText(noticeText, fontSize)
		bannerW := textW + 32
		bannerH := int32(30)
		bannerX := centerX - (bannerW / 2)
		bannerY := centerY + 46

		bgColor := rl.NewColor(15, 30, 20, 225)
		borderColor := rl.NewColor(80, 255, 140, 255)
		textColor := rl.NewColor(90, 255, 150, 255)

		if s.PickupNoticeIsError {
			bgColor = rl.NewColor(35, 15, 18, 225)
			borderColor = rl.NewColor(255, 80, 80, 255)
			textColor = rl.NewColor(255, 120, 110, 255)
		}

		rl.DrawRectangle(bannerX, bannerY, bannerW, bannerH, bgColor)
		rl.DrawRectangleLines(bannerX, bannerY, bannerW, bannerH, borderColor)
		rl.DrawText(noticeText, bannerX+16, bannerY+8, fontSize, textColor)
	}

	// Draw floating 3D screen billboard label above the currently targeted cube
	if s.SelectedCube != nil && !s.SelectedCube.IsDeleted {
		sel := s.SelectedCube
		activeCam := s.GetCamera()
		labelWorldPos := rl.NewVector3(sel.Position.X, sel.Position.Y+sel.EffectiveSize()+0.55, sel.Position.Z)
		screenPos := rl.GetWorldToScreen(labelWorldPos, activeCam)

		if screenPos.X > 0 && screenPos.X < float32(screenWidth) && screenPos.Y > 0 && screenPos.Y < float32(screenHeight) {
			tagText := fmt.Sprintf("TARGET: %s", sel.Name)
			tagWidth := rl.MeasureText(tagText, 11)
			tagX := int32(screenPos.X) - (tagWidth / 2) - 8
			tagY := int32(screenPos.Y) - 10

			rl.DrawRectangle(tagX, tagY, tagWidth+16, 20, rl.NewColor(10, 18, 25, 220))
			rl.DrawRectangleLines(tagX, tagY, tagWidth+16, 20, rl.NewColor(80, 255, 140, 255))
			rl.DrawText(tagText, tagX+8, tagY+4, 11, rl.NewColor(80, 255, 140, 255))
		}
	}

	// Render Developer Console
	s.Console.Draw()
}

// GetCamera returns the active camera based on current view mode.
func (s *DemoScene) GetCamera() rl.Camera3D {
	if s.ViewMode == camera.ViewModeFirstPerson {
		return s.FPS.Camera
	}
	return s.Camera.Camera
}

// Unload cleans up scene resources and unlocks cursor.
func (s *DemoScene) Unload() {
	if s.FPS != nil {
		s.FPS.UnlockCursor()
	}
	if s.ScriptManager != nil {
		s.ScriptManager.Close()
	}
}
