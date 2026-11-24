package scripting

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	lua "github.com/yuin/gopher-lua"
	"zombie-project/internal/camera"
	"zombie-project/internal/cube"
	"zombie-project/internal/tiled"
)

func TestLuaScriptExecutionAndBindings(t *testing.T) {
	c := cube.NewRainbowCube(1.0)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := NewScriptManager(c, cam)
	defer sm.Close()

	luaCode := `
function init()
    cube.set_rotation(10, 20, 30)
    cube.set_size(1.5)
    cube.set_mode("gradient")
    cube.set_position(0.0, 1.25, 0.0)
    camera.set_distance(6.5)
    engine.log("Test init called")
end

function update(dt)
    cube.rotate(5 * dt, 10 * dt, 15 * dt)
end
`

	if err := sm.L.DoString(luaCode); err != nil {
		t.Fatalf("failed to execute Lua code: %v", err)
	}

	sm.CallInit()

	if c.Rotation.X != 10 || c.Rotation.Y != 20 || c.Rotation.Z != 30 {
		t.Errorf("expected rotation (10, 20, 30), got (%f, %f, %f)", c.Rotation.X, c.Rotation.Y, c.Rotation.Z)
	}
	if c.Size != 1.5 {
		t.Errorf("expected size 1.5, got %f", c.Size)
	}
	if c.Mode != cube.ModeVertexGradient {
		t.Errorf("expected mode ModeVertexGradient, got %v", c.Mode)
	}
	if c.Position.Y != 1.25 {
		t.Errorf("expected position Y 1.25, got %f", c.Position.Y)
	}
	if cam.Distance != 6.5 {
		t.Errorf("expected camera distance 6.5, got %f", cam.Distance)
	}

	// Test update call
	sm.Update(0.1)
	expectedPitch := float32(10.0 + 5.0*0.1)
	if math.Abs(float64(c.Rotation.X-expectedPitch)) > 0.001 {
		t.Errorf("expected pitch ~%f, got %f", expectedPitch, c.Rotation.X)
	}

	if len(sm.Logs) == 0 {
		t.Errorf("expected log entry from engine.log")
	}
}

func TestLuaScriptFileLoading(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	content := `
function init()
    cube.set_mode("chroma")
end
`
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	c := cube.NewRainbowCube(1.0)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := NewScriptManager(c, cam)
	defer sm.Close()

	if err := sm.LoadScript(scriptPath); err != nil {
		t.Fatalf("failed to load script: %v", err)
	}

	if c.Mode != cube.ModeChromaCycle {
		t.Errorf("expected mode ModeChromaCycle, got %v", c.Mode)
	}
}

func TestLuaTargetSelectionAndManipulation(t *testing.T) {
	cube1 := cube.NewRainbowCubeWithConfig("c1", "MainCube", 1.0, rl.NewVector3(0, 0, 0), cube.ModeFaceColors)
	cube2 := cube.NewRainbowCubeWithConfig("c2", "SecondaryCube", 2.0, rl.NewVector3(3, 0, 0), cube.ModeVertexGradient)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)

	sm := NewScriptManager(cube1, cam)
	defer sm.Close()

	// Initially, global 'target' points to cube1
	if err := sm.L.DoString(`target.set_size(1.75)`); err != nil {
		t.Fatalf("failed to execute target.set_size: %v", err)
	}
	if cube1.Size != 1.75 {
		t.Errorf("expected cube1 size 1.75, got %f", cube1.Size)
	}
	if cube2.Size != 2.0 {
		t.Errorf("expected cube2 size untouched at 2.0, got %f", cube2.Size)
	}

	// Switch target to cube2!
	sm.SetGlobalTarget(cube2)

	// Now modify target via Lua
	luaCode := `
target.set_size(5.0)
target.set_mode("chroma")
target.set_position(4.0, 1.0, -2.0)
`
	if err := sm.L.DoString(luaCode); err != nil {
		t.Fatalf("failed to execute Lua on target: %v", err)
	}

	// cube1 should remain 1.75
	if cube1.Size != 1.75 {
		t.Errorf("expected cube1 size to remain 1.75, got %f", cube1.Size)
	}

	// cube2 should have received all updates
	if cube2.Size != 5.0 {
		t.Errorf("expected cube2 size 5.0, got %f", cube2.Size)
	}
	if cube2.Mode != cube.ModeChromaCycle {
		t.Errorf("expected cube2 mode chroma, got %v", cube2.Mode)
	}
	if cube2.Position.X != 4.0 || cube2.Position.Y != 1.0 || cube2.Position.Z != -2.0 {
		t.Errorf("expected cube2 pos (4, 1, -2), got (%f, %f, %f)", cube2.Position.X, cube2.Position.Y, cube2.Position.Z)
	}

	// Test inspect() method
	if err := sm.L.DoString(`info = target.inspect()`); err != nil {
		t.Fatalf("failed to call target.inspect(): %v", err)
	}
	infoVal := sm.L.GetGlobal("info").String()
	if len(infoVal) == 0 {
		t.Errorf("expected inspect string, got empty")
	}
}

func TestLuaMapAPI(t *testing.T) {
	c := cube.NewRainbowCube(1.0)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := NewScriptManager(c, cam)
	defer sm.Close()

	var loadedMap *tiled.Map3D
	sm.OnMapLoad = func(m3d *tiled.Map3D) error {
		loadedMap = m3d
		return nil
	}

	tempDir := t.TempDir()
	SetSaveDir(tempDir)
	defer SetSaveDir("")

	// 1. Test global load(filePath) syntax
	luaCode := `
ok, err = load("../../maps/zombie_outpost.tmx")
if not ok then error("Failed: " .. tostring(err)) end
title = map.get_title()
w, h = map.get_size()
first_tile = map.get_tile("BaseLayout", 0, 0)
map.set_tile("BaseLayout", 0, 0, 2)
new_tile = map.get_tile("BaseLayout", 0, 0)
`
	if err := sm.L.DoString(luaCode); err != nil {
		t.Fatalf("failed executing Lua load command: %v", err)
	}

	titleVal := sm.L.GetGlobal("title").String()
	if !strings.Contains(titleVal, "Zombie") {
		t.Errorf("expected title containing 'Zombie', got '%s'", titleVal)
	}

	wVal := int(sm.L.GetGlobal("w").(interface{ String() string }).String()[0] - '0')
	if loadedMap == nil || loadedMap.SourceMap == nil {
		t.Fatalf("expected loadedMap callback to be invoked")
	}

	if loadedMap.SourceMap.Width != 14 {
		t.Errorf("expected map width 14, got %d (lua: %d)", loadedMap.SourceMap.Width, wVal)
	}

	newTileVal := sm.L.GetGlobal("new_tile").String()
	if newTileVal != "2" {
		t.Errorf("expected new_tile '2' after set_tile, got '%s'", newTileVal)
	}

	firstMapRef := sm.CurrentMap

	// 2. Test loading kitchen_sink map via global load("kitchen_sink")
	// This MUST auto-save the previous map (zombie_outpost) to JSON snapshot and previous_map.json,
	// and promptly dispose of firstMapRef!
	luaCodeKS := `
ok_ks, err_ks = load("kitchen_sink")
if not ok_ks then error("Failed loading kitchen_sink: " .. tostring(err_ks)) end
ks_title = map.get_title()
ks_w, ks_h = map.get_size()
`
	if err := sm.L.DoString(luaCodeKS); err != nil {
		t.Fatalf("failed executing Lua load(\"kitchen_sink\"): %v", err)
	}

	ksTitle := sm.L.GetGlobal("ks_title").String()
	if ksTitle != "Kitchen Sink Gallery" {
		t.Errorf("expected 'Kitchen Sink Gallery', got '%s'", ksTitle)
	}
	if sm.CurrentMap == nil || sm.CurrentMap.SourceMap == nil {
		t.Fatalf("expected CurrentMap loaded for kitchen_sink")
	}
	if sm.CurrentMap.SourceMap.Width != 24 || sm.CurrentMap.SourceMap.Height != 1 {
		t.Errorf("expected 24x1 for kitchen_sink, got %dx%d", sm.CurrentMap.SourceMap.Width, sm.CurrentMap.SourceMap.Height)
	}

	// Verify old map was promptly disposed
	if firstMapRef.Tiles != nil || firstMapRef.Spawns != nil || firstMapRef.SourceMap != nil {
		t.Errorf("expected previous map to be promptly disposed of")
	}

	// Verify JSON snapshot and previous_map.json exist in saves/ (from step 2 saving step 1's map)
	saveDir := getSaveDir()
	prevMapPath := filepath.Join(saveDir, "previous_map.json")
	prevBytes, err := os.ReadFile(prevMapPath)
	if err != nil {
		t.Fatalf("expected previous_map.json to be created at %s, error: %v", prevMapPath, err)
	}
	if !strings.Contains(string(prevBytes), "Zombie Survival Outpost") {
		t.Errorf("expected previous_map.json to contain 'Zombie Survival Outpost', got: %s", string(prevBytes))
	}

	// 3. Test backward-compatible map.load alias
	luaCodeAlias := `
ok_alias, err_alias = map.load("kitchen_sink")
if not ok_alias then error("Failed loading via map.load: " .. tostring(err_alias)) end
`
	if err := sm.L.DoString(luaCodeAlias); err != nil {
		t.Fatalf("failed executing backward-compatible map.load: %v", err)
	}

	// Now previous_map.json should have saved kitchen_sink!
	prevBytesKS, err := os.ReadFile(prevMapPath)
	if err != nil {
		t.Fatalf("failed reading previous_map.json: %v", err)
	}
	if !strings.Contains(string(prevBytesKS), "Kitchen Sink Gallery") {
		t.Errorf("expected previous_map.json to contain 'Kitchen Sink Gallery', got: %s", string(prevBytesKS))
	}

	// 4. Test loading conestoga map via global load("conestoga")
	luaCodeConestoga := `
ok_cone, err_cone = load("conestoga")
if not ok_cone then error("Failed loading conestoga: " .. tostring(err_cone)) end
title_cone = map.get_title()
w_cone, h_cone = map.get_size()
`
	if err := sm.L.DoString(luaCodeConestoga); err != nil {
		t.Fatalf("failed executing Lua load(\"conestoga\"): %v", err)
	}
	if sm.CurrentMap == nil || sm.CurrentMap.SourceMap == nil {
		t.Fatalf("expected CurrentMap loaded for conestoga")
	}
	if sm.CurrentMap.SourceMap.Width != 16 || sm.CurrentMap.SourceMap.Height != 16 {
		t.Errorf("expected 16x16 for conestoga, got %dx%d", sm.CurrentMap.SourceMap.Width, sm.CurrentMap.SourceMap.Height)
	}
	titleCone := sm.L.GetGlobal("title_cone").String()
	if !strings.Contains(titleCone, "Conestoga") {
		t.Errorf("expected title to contain 'Conestoga', got '%s'", titleCone)
	}
}

func TestLuaPlayerAPI(t *testing.T) {
	c := cube.NewRainbowCube(1.0)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	fps := camera.NewFPSController(rl.NewVector3(0, -2.2, -4.0), -2.2)

	sm := NewScriptManager(c, cam)
	sm.SetFPSController(fps)
	defer sm.Close()

	luaCode := `
x, y, z = player.get_position()
player.teleport(2.5, 0.0, 5.0)
player.set_angles(1.5, 0.25)
player.set_speed(7.5)
speed = player.get_speed()
step_h = player.get_step_height()
player.set_step_height(0.75)
new_step_h = player.get_step_height()
`
	if err := sm.L.DoString(luaCode); err != nil {
		t.Fatalf("failed executing player API: %v", err)
	}

	if fps.Position.X != 2.5 || fps.Position.Y != 0.0 || fps.Position.Z != 5.0 {
		t.Errorf("expected teleport to (2.5, 0, 5), got %v", fps.Position)
	}
	if fps.Yaw != 1.5 || fps.Pitch != 0.25 {
		t.Errorf("expected angles (1.5, 0.25), got (%f, %f)", fps.Yaw, fps.Pitch)
	}
	if fps.WalkSpeed != 7.5 {
		t.Errorf("expected speed 7.5, got %f", fps.WalkSpeed)
	}
	if fps.StepHeight != 0.75 {
		t.Errorf("expected step height updated to 0.75, got %f", fps.StepHeight)
	}
}

func TestLuaPropMassAndPickupAPI(t *testing.T) {
	c := cube.NewRainbowCubeWithConfig("test_prop", "TestProp", 1.0, rl.NewVector3(0, 0, 0), cube.ModeFaceColors)
	c.Mass = 35.0 // Initial 35kg (< 50kg)

	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	fps := camera.NewFPSController(rl.NewVector3(0, -2.2, 0), -2.2)

	sm := NewScriptManager(c, cam)
	sm.SetFPSController(fps)
	sm.SetGlobalTarget(c)
	defer sm.Close()

	luaCode := `
m1 = target.get_mass()
can1 = target.can_pickup()
target.set_mass(85.0)
m2 = target.get_mass()
can2 = target.can_pickup()
target.set_mass(25.0)
m3 = target.get_mass()
can3 = target.can_pickup()
`
	if err := sm.L.DoString(luaCode); err != nil {
		t.Fatalf("failed executing Lua prop mass script: %v", err)
	}

	if sm.L.GetGlobal("m1").String() != "35" {
		t.Errorf("expected initial mass 35, got %s", sm.L.GetGlobal("m1").String())
	}
	if sm.L.GetGlobal("can1").String() != "true" {
		t.Errorf("expected can1 true for 35kg (< 50kg)")
	}
	if sm.L.GetGlobal("m2").String() != "85" {
		t.Errorf("expected mass 85, got %s", sm.L.GetGlobal("m2").String())
	}
	if sm.L.GetGlobal("can2").String() != "false" {
		t.Errorf("expected can2 false for 85kg (>= 50kg)")
	}
	if sm.L.GetGlobal("m3").String() != "25" {
		t.Errorf("expected mass 25, got %s", sm.L.GetGlobal("m3").String())
	}
	if sm.L.GetGlobal("can3").String() != "true" {
		t.Errorf("expected can3 true for 25kg (< 50kg)")
	}

	// Test player.get_held_prop when nothing is held
	if err := sm.L.DoString(`held_none = player.get_held_prop()`); err != nil {
		t.Fatalf("failed getting held prop: %v", err)
	}
	if sm.L.GetGlobal("held_none") != lua.LNil {
		t.Errorf("expected held_none to be nil")
	}

	// Human picks up the 25kg prop
	ok, _ := fps.Human.PickUp(c)
	if !ok {
		t.Fatalf("expected pickup of 25kg prop to succeed")
	}

	// Test player.get_held_prop when prop is held
	luaCodeHeld := `
held = player.get_held_prop()
held_name = held.name
held_mass = held.mass
`
	if err := sm.L.DoString(luaCodeHeld); err != nil {
		t.Fatalf("failed getting held prop info: %v", err)
	}
	if sm.L.GetGlobal("held_name").String() != "TestProp" {
		t.Errorf("expected held_name 'TestProp', got '%s'", sm.L.GetGlobal("held_name").String())
	}

	// Test player.drop_prop()
	if err := sm.L.DoString(`dropped_ok = player.drop_prop()`); err != nil {
		t.Fatalf("failed dropping prop via Lua: %v", err)
	}
	if sm.L.GetGlobal("dropped_ok").String() != "true" {
		t.Errorf("expected dropped_ok true")
	}
	if fps.Human.HeldProp != nil {
		t.Errorf("expected HeldProp to be nil after drop_prop")
	}

	// Test player.throw_prop()
	okPick, _ := fps.Human.PickUp(c)
	if !okPick {
		t.Fatalf("expected pickup of prop before throw")
	}
	thrownPower := float32(0)
	sm.OnThrowProp = func(p float32) {
		thrownPower = p
		fps.Human.DropProp()
	}
	if err := sm.L.DoString(`throw_ok = player.throw_prop(22.5)`); err != nil {
		t.Fatalf("failed throwing prop via Lua: %v", err)
	}
	if sm.L.GetGlobal("throw_ok").String() != "true" {
		t.Errorf("expected throw_ok true")
	}
	if thrownPower != 22.5 {
		t.Errorf("expected thrownPower 22.5, got %f", thrownPower)
	}
	if fps.Human.HeldProp != nil {
		t.Errorf("expected HeldProp to be nil after throw_prop")
	}
}

func TestLuaPhysicsBodyAndCollisionScripting(t *testing.T) {
	c := cube.NewRainbowCubeWithConfig("c_phys", "PhysCube", 0.8, rl.NewVector3(0, 0, 0), cube.ModeFaceColors)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := NewScriptManager(c, cam)
	defer sm.Close()

	debugToggled := false
	sm.OnTogglePhysicsDebug = func() bool {
		debugToggled = !debugToggled
		return debugToggled
	}

	luaScript := `
target.set_velocity(4.0, 5.5, -2.0)
vx, vy, vz = target.get_velocity()

target.apply_impulse(35.0, 0.0, 0.0) -- mass is 35 => vx should increase by 1.0 to 5.0
vx2, vy2, vz2 = target.get_velocity()

target.set_bounciness(0.72)
bounciness = target.get_bounciness()

target.set_friction(0.45)
friction = target.get_friction()

grounded = target.is_grounded()
is_static = target.is_static()
target.set_static(true)
is_static_now = target.is_static()

col_box_toggled = target.toggle_collision_box()
box = target.get_collision_box()
box_min_x = box.min.x
box_max_x = box.max.x

engine_debug = engine.toggle_physics_debug()
`
	if err := sm.L.DoString(luaScript); err != nil {
		t.Fatalf("failed executing Lua physics script: %v", err)
	}

	if sm.L.GetGlobal("vx").String() != "4" || sm.L.GetGlobal("vy").String() != "5.5" || sm.L.GetGlobal("vz").String() != "-2" {
		t.Errorf("expected velocity (4, 5.5, -2), got (%s, %s, %s)",
			sm.L.GetGlobal("vx"), sm.L.GetGlobal("vy"), sm.L.GetGlobal("vz"))
	}

	if sm.L.GetGlobal("vx2").String() != "5" {
		t.Errorf("expected vx2 to be 5 after impulse, got %s", sm.L.GetGlobal("vx2"))
	}

	bounciness := float64(sm.L.GetGlobal("bounciness").(lua.LNumber))
	if math.Abs(bounciness-0.72) > 0.001 {
		t.Errorf("expected bounciness ~0.72, got %f", bounciness)
	}

	friction := float64(sm.L.GetGlobal("friction").(lua.LNumber))
	if math.Abs(friction-0.45) > 0.001 {
		t.Errorf("expected friction ~0.45, got %f", friction)
	}

	if sm.L.GetGlobal("is_static_now").String() != "true" {
		t.Errorf("expected is_static_now to be true")
	}

	if sm.L.GetGlobal("col_box_toggled").String() != "true" {
		t.Errorf("expected col_box_toggled to be true")
	}

	boxMinX := float64(sm.L.GetGlobal("box_min_x").(lua.LNumber))
	boxMaxX := float64(sm.L.GetGlobal("box_max_x").(lua.LNumber))
	if math.Abs(boxMinX-(-0.8)) > 0.001 || math.Abs(boxMaxX-0.8) > 0.001 {
		t.Errorf("expected box bounds [-0.8, 0.8], got [%f, %f]", boxMinX, boxMaxX)
	}

	if sm.L.GetGlobal("engine_debug").String() != "true" || !debugToggled {
		t.Errorf("expected engine.toggle_physics_debug to return true and set debugToggled")
	}
}

func TestLuaEntityDeletionTargetMethods(t *testing.T) {
	cube1 := cube.NewRainbowCubeWithConfig("cube_1", "TestCube1", 1.0, rl.NewVector3(0, 0, 0), cube.ModeFaceColors)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := NewScriptManager(cube1, cam)
	defer sm.Close()

	// 1. Colon syntax target:delete()
	sm.SetGlobalTarget(cube1)
	script1 := `
ok1, msg1 = target:delete()
`
	if err := sm.L.DoString(script1); err != nil {
		t.Fatalf("failed calling target:delete(): %v", err)
	}
	if sm.L.GetGlobal("ok1").String() != "true" {
		t.Errorf("expected ok1 to be true, got %s", sm.L.GetGlobal("ok1"))
	}
	if !cube1.IsDeleted {
		t.Errorf("expected cube1.IsDeleted to be true")
	}

	// 2. Calling delete on an already deleted entity returns false
	script2 := `
ok2, msg2 = target:delete()
`
	if err := sm.L.DoString(script2); err != nil {
		t.Fatalf("failed calling second target:delete(): %v", err)
	}
	if sm.L.GetGlobal("ok2").String() != "false" {
		t.Errorf("expected ok2 to be false when already deleted, got %s", sm.L.GetGlobal("ok2"))
	}

	// 3. Dot syntax target.delete() on a new cube
	cube2 := cube.NewRainbowCubeWithConfig("cube_2", "TestCube2", 1.0, rl.NewVector3(2, 0, 0), cube.ModeFaceColors)
	sm.SetGlobalTarget(cube2)
	script3 := `
ok3, msg3 = target.delete()
`
	if err := sm.L.DoString(script3); err != nil {
		t.Fatalf("failed calling target.delete(): %v", err)
	}
	if sm.L.GetGlobal("ok3").String() != "true" {
		t.Errorf("expected ok3 to be true, got %s", sm.L.GetGlobal("ok3"))
	}
	if !cube2.IsDeleted {
		t.Errorf("expected cube2.IsDeleted to be true")
	}

	// 4. Colon syntax target:destroy() alias
	cube3 := cube.NewRainbowCubeWithConfig("cube_3", "TestCube3", 1.0, rl.NewVector3(4, 0, 0), cube.ModeFaceColors)
	sm.SetGlobalTarget(cube3)
	script4 := `
ok4, msg4 = target:destroy()
`
	if err := sm.L.DoString(script4); err != nil {
		t.Fatalf("failed calling target:destroy(): %v", err)
	}
	if sm.L.GetGlobal("ok4").String() != "true" {
		t.Errorf("expected ok4 to be true, got %s", sm.L.GetGlobal("ok4"))
	}
	if !cube3.IsDeleted {
		t.Errorf("expected cube3.IsDeleted to be true")
	}

	// 5. Test with OnDeleteCube hook callback
	cube4 := cube.NewRainbowCubeWithConfig("cube_4", "TestCube4", 1.0, rl.NewVector3(6, 0, 0), cube.ModeFaceColors)
	hookCalled := false
	sm.OnDeleteCube = func(c *cube.RainbowCube) (bool, string) {
		hookCalled = true
		c.IsDeleted = true
		return true, "Hook deleted: " + c.Name
	}
	sm.SetGlobalTarget(cube4)
	script5 := `
ok5, msg5 = target:delete()
`
	if err := sm.L.DoString(script5); err != nil {
		t.Fatalf("failed calling target:delete() with hook: %v", err)
	}
	if !hookCalled {
		t.Errorf("expected OnDeleteCube hook to be called")
	}
	if sm.L.GetGlobal("ok5").String() != "true" {
		t.Errorf("expected ok5 to be true, got %s", sm.L.GetGlobal("ok5"))
	}

	// 6. Test ClearGlobalTarget() prevents deleting non-existent target
	sm.ClearGlobalTarget()
	script6 := `
ok6, msg6 = target:delete()
`
	if err := sm.L.DoString(script6); err != nil {
		t.Fatalf("failed calling target:delete() when target cleared: %v", err)
	}
	if sm.L.GetGlobal("ok6").String() != "false" {
		t.Errorf("expected ok6 to be false when target is cleared, got %s", sm.L.GetGlobal("ok6"))
	}

	// 7. Test map tile deletion via target:delete()
	tile := &tiled.Tile3D{
		Col:     3,
		Row:     5,
		Name:    "Crate Barricade",
		IsSolid: true,
	}
	tileHookCalled := false
	sm.OnDeleteTile = func(t *tiled.Tile3D) (bool, string) {
		tileHookCalled = true
		return true, "Deleted tile"
	}
	sm.SetGlobalTileTarget(tile)
	script7 := `
ok7, msg7 = target:delete()
`
	if err := sm.L.DoString(script7); err != nil {
		t.Fatalf("failed calling tile target:delete(): %v", err)
	}
	if !tileHookCalled {
		t.Errorf("expected OnDeleteTile hook to be called")
	}
	if sm.L.GetGlobal("ok7").String() != "true" {
		t.Errorf("expected ok7 to be true, got %s", sm.L.GetGlobal("ok7"))
	}
}

func TestLuaEntitySummoningAndTargetBinding(t *testing.T) {
	c := cube.NewRainbowCubeWithConfig("cube_orig", "OriginalCube", 1.0, rl.NewVector3(0, 0, 0), cube.ModeFaceColors)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := NewScriptManager(c, cam)
	defer sm.Close()

	// Initially, target is OriginalCube
	if sm.GetCurrentTarget().Name != "OriginalCube" {
		t.Fatalf("expected initial target OriginalCube, got %s", sm.GetCurrentTarget().Name)
	}

	// 1. summon() with no arguments
	script1 := `
inst1, msg1 = summon()
target_name1 = target:get_name()
target_id1 = target.id
`
	if err := sm.L.DoString(script1); err != nil {
		t.Fatalf("failed executing summon(): %v", err)
	}
	targetName1 := sm.L.GetGlobal("target_name1").String()
	if targetName1 == "" || targetName1 == "OriginalCube" {
		t.Errorf("expected target to be updated to newly summoned cube, got %s", targetName1)
	}
	if sm.GetCurrentTarget().Name != targetName1 {
		t.Errorf("expected sm.GetCurrentTarget() to match targetName1 (%s), got %s", targetName1, sm.GetCurrentTarget().Name)
	}

	// 2. summon(name, x, y, z, size, mass)
	script2 := `
inst2, msg2 = summon("HeavyBox", 2.0, 3.5, -4.0, 1.25, 48.0)
target_name2 = target:get_name()
target_mass2 = target:get_mass()
target_size2 = target:get_size()
tx, ty, tz = target:get_position()
`
	if err := sm.L.DoString(script2); err != nil {
		t.Fatalf("failed executing summon with position and mass: %v", err)
	}
	if sm.L.GetGlobal("target_name2").String() != "HeavyBox" {
		t.Errorf("expected target HeavyBox, got %s", sm.L.GetGlobal("target_name2"))
	}
	if sm.GetCurrentTarget().Name != "HeavyBox" {
		t.Errorf("expected current target to be HeavyBox, got %s", sm.GetCurrentTarget().Name)
	}
	mass2 := float64(sm.L.GetGlobal("target_mass2").(lua.LNumber))
	if math.Abs(mass2-48.0) > 0.001 {
		t.Errorf("expected mass 48.0, got %f", mass2)
	}
	size2 := float64(sm.L.GetGlobal("target_size2").(lua.LNumber))
	if math.Abs(size2-1.25) > 0.001 {
		t.Errorf("expected size 1.25, got %f", size2)
	}
	posX := float64(sm.L.GetGlobal("tx").(lua.LNumber))
	posY := float64(sm.L.GetGlobal("ty").(lua.LNumber))
	posZ := float64(sm.L.GetGlobal("tz").(lua.LNumber))
	if math.Abs(posX-2.0) > 0.001 || math.Abs(posY-3.5) > 0.001 || math.Abs(posZ-(-4.0)) > 0.001 {
		t.Errorf("expected pos (2.0, 3.5, -4.0), got (%f, %f, %f)", posX, posY, posZ)
	}

	// 3. Modifying target right after summoning operates directly on the newly summoned instance
	script3 := `
target:set_size(2.5)
new_size = target:get_size()
`
	if err := sm.L.DoString(script3); err != nil {
		t.Fatalf("failed modifying target after summon: %v", err)
	}
	newSize := float64(sm.L.GetGlobal("new_size").(lua.LNumber))
	if math.Abs(newSize-2.5) > 0.001 {
		t.Errorf("expected new size 2.5, got %f", newSize)
	}
	if sm.GetCurrentTarget().Size != 2.5 {
		t.Errorf("expected HeavyBox Size to be 2.5, got %f", sm.GetCurrentTarget().Size)
	}

	// 4. summon({ table })
	script4 := `
inst4, msg4 = summon({ name = "TableCube", x = 10.0, y = 5.0, z = 0.0, size = 0.9, mass = 15.0, mode = "chroma" })
target_name4 = target:get_name()
target_mode4 = target:get_mode()
`
	if err := sm.L.DoString(script4); err != nil {
		t.Fatalf("failed executing table summon: %v", err)
	}
	if sm.L.GetGlobal("target_name4").String() != "TableCube" {
		t.Errorf("expected TableCube, got %s", sm.L.GetGlobal("target_name4"))
	}
	if sm.L.GetGlobal("target_mode4").String() != "chroma" {
		t.Errorf("expected chroma mode, got %s", sm.L.GetGlobal("target_mode4"))
	}

	// 5. spawn() alias
	script5 := `
inst5, msg5 = spawn("SpawnedEntity")
target_name5 = target:get_name()
`
	if err := sm.L.DoString(script5); err != nil {
		t.Fatalf("failed executing spawn(): %v", err)
	}
	if sm.L.GetGlobal("target_name5").String() != "SpawnedEntity" {
		t.Errorf("expected SpawnedEntity, got %s", sm.L.GetGlobal("target_name5"))
	}

	// 6. engine.summon()
	script6 := `
inst6, msg6 = engine.summon("EngineSummon")
target_name6 = target:get_name()
`
	if err := sm.L.DoString(script6); err != nil {
		t.Fatalf("failed executing engine.summon(): %v", err)
	}
	if sm.L.GetGlobal("target_name6").String() != "EngineSummon" {
		t.Errorf("expected EngineSummon, got %s", sm.L.GetGlobal("target_name6"))
	}

	// 7. OnSummonCube Hook Callback
	hookCalled := false
	sm.OnSummonCube = func(cfg SummonConfig) *cube.RainbowCube {
		hookCalled = true
		pos := rl.NewVector3(99, 99, 99)
		if cfg.Position != nil {
			pos = *cfg.Position
		}
		newC := cube.NewRainbowCubeWithConfig("hook_cube", cfg.Name, 0.5, pos, cfg.Mode)
		return newC
	}

	script7 := `
summon("HookCube", 1, 2, 3)
target_name7 = target:get_name()
`
	if err := sm.L.DoString(script7); err != nil {
		t.Fatalf("failed executing summon with hook: %v", err)
	}
	if !hookCalled {
		t.Errorf("expected OnSummonCube hook to be called")
	}
	if sm.L.GetGlobal("target_name7").String() != "HookCube" {
		t.Errorf("expected HookCube, got %s", sm.L.GetGlobal("target_name7"))
	}
	if sm.GetCurrentTarget().ID != "hook_cube" {
		t.Errorf("expected ID 'hook_cube', got %s", sm.GetCurrentTarget().ID)
	}
}
