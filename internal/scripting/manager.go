package scripting

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	lua "github.com/yuin/gopher-lua"
	"zombie-project/internal/camera"
	"zombie-project/internal/cube"
	"zombie-project/internal/human"
	"zombie-project/internal/paths"
	"zombie-project/internal/tiled"
)

// ScriptManager manages the Lua state, engine bindings, and script lifecycle.
type ScriptManager struct {
	L            *lua.LState
	ScriptPath   string
	LastModTime  time.Time
	AutoReload   bool
	LastReloadErr string
	StatusMsg    string
	StatusTimer  float32
	Logs         []string

	cube          *cube.RainbowCube
	currentTarget *cube.RainbowCube
	camera        *camera.OrbitCamera
	fps           *camera.FPSController
	time          float32
	dt            float32

	CurrentMap           *tiled.Map3D
	OnMapLoad            func(*tiled.Map3D) error
	OnTogglePhysicsDebug func() bool
	OnThrowProp          func(power float32)
	OnDeleteCube         func(c *cube.RainbowCube) (bool, string)
	OnDeleteTile         func(t *tiled.Tile3D) (bool, string)
	OnSummonCube         func(cfg SummonConfig) *cube.RainbowCube
}

// SummonConfig defines properties for spawning a new 3D entity.
type SummonConfig struct {
	Name     string
	Position *rl.Vector3
	Size     float32
	Mass     float32
	Mode     cube.RainbowMode
}

// NewScriptManager creates a new Lua scripting manager with registered engine APIs.
func NewScriptManager(c *cube.RainbowCube, cam *camera.OrbitCamera) *ScriptManager {
	sm := &ScriptManager{
		L:             lua.NewState(),
		AutoReload:    true,
		cube:          c,
		currentTarget: c,
		camera:        cam,
		Logs:          make([]string, 0),
	}

	sm.registerAPIs()
	return sm
}

// SetFPSController attaches the first-person controller to the scripting engine.
func (sm *ScriptManager) SetFPSController(fps *camera.FPSController) {
	sm.fps = fps
}

// Close terminates the Lua state.
func (sm *ScriptManager) Close() {
	if sm.L != nil {
		sm.L.Close()
	}
}

// AddLog logs a message from script or engine.
func (sm *ScriptManager) AddLog(msg string) {
	entry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	sm.Logs = append(sm.Logs, entry)
	if len(sm.Logs) > 10 {
		sm.Logs = sm.Logs[len(sm.Logs)-10:]
	}
}

// LoadScript reads and executes a Lua script file.
func (sm *ScriptManager) LoadScript(path string) error {
	resolvedPath := paths.Resolve(path)
	info, err := os.Stat(resolvedPath)
	if err != nil {
		sm.LastReloadErr = err.Error()
		return err
	}

	sm.ScriptPath = resolvedPath
	sm.LastModTime = info.ModTime()

	if err := sm.L.DoFile(resolvedPath); err != nil {
		sm.LastReloadErr = err.Error()
		sm.AddLog("Script error: " + err.Error())
		return err
	}

	sm.LastReloadErr = ""
	sm.StatusMsg = fmt.Sprintf("Loaded %s", filepath.Base(resolvedPath))
	sm.StatusTimer = 3.0
	sm.AddLog("Loaded script: " + resolvedPath)

	// Call init() if defined
	sm.CallInit()
	return nil
}

// ReloadScript re-executes the current Lua script.
func (sm *ScriptManager) ReloadScript() error {
	if sm.ScriptPath == "" {
		return fmt.Errorf("no script loaded")
	}

	info, err := os.Stat(sm.ScriptPath)
	if err != nil {
		sm.LastReloadErr = err.Error()
		return err
	}
	sm.LastModTime = info.ModTime()

	if err := sm.L.DoFile(sm.ScriptPath); err != nil {
		sm.LastReloadErr = err.Error()
		sm.AddLog("Reload error: " + err.Error())
		return err
	}

	sm.LastReloadErr = ""
	sm.StatusMsg = "Script reloaded!"
	sm.StatusTimer = 2.5
	sm.AddLog("Hot-reloaded: " + sm.ScriptPath)

	// Call on_reload() or init()
	fn := sm.L.GetGlobal("on_reload")
	if fn.Type() == lua.LTFunction {
		_ = sm.L.CallByParam(lua.P{
			Fn:      fn,
			NRet:    0,
			Protect: true,
		})
	} else {
		sm.CallInit()
	}

	return nil
}

// CallInit invokes the global Lua init() function if present.
func (sm *ScriptManager) CallInit() {
	fn := sm.L.GetGlobal("init")
	if fn.Type() == lua.LTFunction {
		err := sm.L.CallByParam(lua.P{
			Fn:      fn,
			NRet:    0,
			Protect: true,
		})
		if err != nil {
			sm.LastReloadErr = err.Error()
			sm.AddLog("init() error: " + err.Error())
		}
	}
}

// Update handles delta time, hot reloading checks, and invokes Lua update(dt).
func (sm *ScriptManager) Update(dt float32) {
	sm.dt = dt
	sm.time += dt

	if sm.StatusTimer > 0 {
		sm.StatusTimer -= dt
		if sm.StatusTimer <= 0 {
			sm.StatusMsg = ""
		}
	}

	// Auto hot reload check (every ~0.5s or on change)
	if sm.AutoReload && sm.ScriptPath != "" {
		info, err := os.Stat(sm.ScriptPath)
		if err == nil && info.ModTime().After(sm.LastModTime) {
			_ = sm.ReloadScript()
		}
	}

	// Call global update(dt) in Lua
	fn := sm.L.GetGlobal("update")
	if fn.Type() == lua.LTFunction {
		err := sm.L.CallByParam(lua.P{
			Fn:      fn,
			NRet:    0,
			Protect: true,
		}, lua.LNumber(dt))
		if err != nil {
			sm.LastReloadErr = err.Error()
			sm.AddLog("update() error: " + err.Error())
		}
	}
}

// registerAPIs binds engine primitives to Lua tables (cube, camera, input, engine, map, player).
func (sm *ScriptManager) registerAPIs() {
	sm.registerCubeAPI()
	sm.registerCameraAPI()
	sm.registerInputAPI()
	sm.registerEngineAPI()
	sm.registerMapAPI()
	sm.registerPlayerAPI()
	sm.registerSummonAPI()
}

func (sm *ScriptManager) registerCubeAPI() {
	cubeTable := sm.createCubeTable(sm.cube)
	sm.L.SetGlobal("cube", cubeTable)
	sm.L.SetGlobal("target", cubeTable)
}

// SetGlobalTarget sets the active Lua global 'target' variable to the given cube.
func (sm *ScriptManager) SetGlobalTarget(c *cube.RainbowCube) {
	if sm.L == nil || c == nil {
		return
	}
	sm.currentTarget = c
	targetTable := sm.createCubeTable(c)
	sm.L.SetGlobal("target", targetTable)
	sm.AddLog(fmt.Sprintf("Global 'target' set to: %s (%s)", c.Name, c.ID))
}

// ClearGlobalTarget clears the active Lua global 'target' variable.
func (sm *ScriptManager) ClearGlobalTarget() {
	if sm.L == nil {
		return
	}
	sm.currentTarget = nil
	emptyTable := sm.L.NewTable()
	sm.L.SetField(emptyTable, "name", lua.LString("None"))
	sm.L.SetField(emptyTable, "id", lua.LString("none"))
	noTargetFunc := sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(false))
		L.Push(lua.LString("No active target to delete"))
		return 2
	})
	sm.L.SetField(emptyTable, "delete", noTargetFunc)
	sm.L.SetField(emptyTable, "destroy", noTargetFunc)
	sm.L.SetGlobal("target", emptyTable)
}

// SetGlobalTileTarget sets global 'target' in Lua to represent a map tile entity.
func (sm *ScriptManager) SetGlobalTileTarget(t *tiled.Tile3D) {
	if sm.L == nil || t == nil {
		return
	}
	sm.currentTarget = nil
	tileTable := sm.L.NewTable()
	sm.L.SetField(tileTable, "name", lua.LString(t.Name))
	sm.L.SetField(tileTable, "id", lua.LString(fmt.Sprintf("tile_%d_%d", t.Col, t.Row)))
	sm.L.SetField(tileTable, "type", lua.LString("tile"))
	sm.L.SetField(tileTable, "col", lua.LNumber(t.Col))
	sm.L.SetField(tileTable, "row", lua.LNumber(t.Row))
	sm.L.SetField(tileTable, "gid", lua.LNumber(t.GID))
	sm.L.SetField(tileTable, "is_solid", lua.LBool(t.IsSolid))
	sm.L.SetField(tileTable, "has_flooring", lua.LBool(t.HasFlooring))
	sm.L.SetField(tileTable, "floor_type", lua.LString(t.FloorName))
	sm.L.SetField(tileTable, "floor_gid", lua.LNumber(t.FloorGID))
	sm.L.SetField(tileTable, "walls", lua.LNumber(t.Walls))
	sm.L.SetField(tileTable, "wall_count", lua.LNumber(t.WallCount()))
	sm.L.SetField(tileTable, "wall_thickness", lua.LNumber(tiled.WallThickness))
	sm.L.SetField(tileTable, "is_exterior", lua.LBool(t.IsExterior))
	sm.L.SetField(tileTable, "is_interior", lua.LBool(t.IsInterior))

	sm.L.SetField(tileTable, "get_wall_count", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(t.WallCount()))
		return 1
	}))
	sm.L.SetField(tileTable, "has_wall", sm.L.NewFunction(func(L *lua.LState) int {
		edgeStr := strings.ToLower(L.OptString(1, ""))
		if L.GetTop() >= 2 && L.Get(1).Type() == lua.LTTable {
			edgeStr = strings.ToLower(L.OptString(2, ""))
		}
		var edge uint8
		switch edgeStr {
		case "north", "n", "top", "0":
			edge = tiled.WallEdgeNorth
		case "south", "s", "bottom", "1":
			edge = tiled.WallEdgeSouth
		case "east", "e", "right", "2":
			edge = tiled.WallEdgeEast
		case "west", "w", "left", "3":
			edge = tiled.WallEdgeWest
		default:
			L.Push(lua.LBool(false))
			return 1
		}
		L.Push(lua.LBool(t.HasWall(edge)))
		return 1
	}))

	deleteFunc := sm.L.NewFunction(func(L *lua.LState) int {
		if sm.OnDeleteTile != nil {
			ok, msg := sm.OnDeleteTile(t)
			L.Push(lua.LBool(ok))
			L.Push(lua.LString(msg))
			return 2
		}
		L.Push(lua.LBool(false))
		L.Push(lua.LString("No tile deletion handler registered"))
		return 2
	})
	sm.L.SetField(tileTable, "delete", deleteFunc)
	sm.L.SetField(tileTable, "destroy", deleteFunc)

	sm.L.SetGlobal("target", tileTable)
	sm.AddLog(fmt.Sprintf("Global 'target' set to map tile: %s (Col:%d, Row:%d)", t.Name, t.Col, t.Row))
}

// SetMainCube updates the primary cube bound to the global 'cube' Lua variable.
func (sm *ScriptManager) SetMainCube(c *cube.RainbowCube) {
	sm.cube = c
	if sm.L != nil {
		if c != nil {
			sm.L.SetGlobal("cube", sm.createCubeTable(c))
		} else {
			sm.L.SetGlobal("cube", sm.L.NewTable())
		}
	}
}

// GetCurrentTarget returns the currently targeted cube.
func (sm *ScriptManager) GetCurrentTarget() *cube.RainbowCube {
	if sm.currentTarget != nil {
		return sm.currentTarget
	}
	return sm.cube
}

func (sm *ScriptManager) createCubeTable(c *cube.RainbowCube) *lua.LTable {
	cubeTable := sm.L.NewTable()

	argOffset := func(L *lua.LState) int {
		if L.GetTop() >= 1 && L.Get(1).Type() == lua.LTTable {
			return 2
		}
		return 1
	}

	sm.L.SetField(cubeTable, "name", lua.LString(c.Name))
	sm.L.SetField(cubeTable, "id", lua.LString(c.ID))

	// get_name() -> string
	sm.L.SetField(cubeTable, "get_name", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(c.Name))
		return 1
	}))

	// get_id() -> string
	sm.L.SetField(cubeTable, "get_id", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(c.ID))
		return 1
	}))

	// delete() -> bool, string (Method of target to delete entity)
	// Also aliased as destroy()
	deleteFunc := sm.L.NewFunction(func(L *lua.LState) int {
		if c == nil || c.IsDeleted {
			name := "Unknown"
			if c != nil {
				name = c.Name
			}
			msg := fmt.Sprintf("Entity '%s' is already deleted", name)
			L.Push(lua.LBool(false))
			L.Push(lua.LString(msg))
			return 2
		}
		if sm.OnDeleteCube != nil {
			ok, msg := sm.OnDeleteCube(c)
			L.Push(lua.LBool(ok))
			L.Push(lua.LString(msg))
			return 2
		}
		c.IsDeleted = true
		msg := fmt.Sprintf("Deleted entity: %s (%s)", c.Name, c.ID)
		sm.AddLog("=> " + msg)
		L.Push(lua.LBool(true))
		L.Push(lua.LString(msg))
		return 2
	})
	sm.L.SetField(cubeTable, "delete", deleteFunc)
	sm.L.SetField(cubeTable, "destroy", deleteFunc)

	// get_rotation() -> pitch, yaw, roll
	sm.L.SetField(cubeTable, "get_rotation", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.Rotation.X))
		L.Push(lua.LNumber(c.Rotation.Y))
		L.Push(lua.LNumber(c.Rotation.Z))
		return 3
	}))

	// set_rotation(pitch, yaw, roll)
	sm.L.SetField(cubeTable, "set_rotation", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		pitch := float32(L.CheckNumber(off))
		yaw := float32(L.CheckNumber(off + 1))
		roll := float32(L.CheckNumber(off + 2))
		c.Rotation = rl.NewVector3(pitch, yaw, roll)
		return 0
	}))

	// rotate(dpitch, dyaw, droll)
	sm.L.SetField(cubeTable, "rotate", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		dpitch := float32(L.CheckNumber(off))
		dyaw := float32(L.CheckNumber(off + 1))
		droll := float32(L.CheckNumber(off + 2))
		c.Rotation.X += dpitch
		c.Rotation.Y += dyaw
		c.Rotation.Z += droll
		return 0
	}))

	// get_speed() -> speedX, speedY, speedZ
	sm.L.SetField(cubeTable, "get_speed", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.RotationSpeed.X))
		L.Push(lua.LNumber(c.RotationSpeed.Y))
		L.Push(lua.LNumber(c.RotationSpeed.Z))
		return 3
	}))

	// set_speed(speedX, speedY, speedZ)
	sm.L.SetField(cubeTable, "set_speed", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		sx := float32(L.CheckNumber(off))
		sy := float32(L.CheckNumber(off + 1))
		sz := float32(L.CheckNumber(off + 2))
		c.RotationSpeed = rl.NewVector3(sx, sy, sz)
		return 0
	}))

	// get_position() -> x, y, z
	sm.L.SetField(cubeTable, "get_position", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.Position.X))
		L.Push(lua.LNumber(c.Position.Y))
		L.Push(lua.LNumber(c.Position.Z))
		return 3
	}))

	// set_position(x, y, z)
	sm.L.SetField(cubeTable, "set_position", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		x := float32(L.CheckNumber(off))
		y := float32(L.CheckNumber(off + 1))
		z := float32(L.CheckNumber(off + 2))
		c.Position = rl.NewVector3(x, y, z)
		return 0
	}))

	// get_size() -> number
	sm.L.SetField(cubeTable, "get_size", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.Size))
		return 1
	}))

	// set_size(size)
	sm.L.SetField(cubeTable, "set_size", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		size := float32(L.CheckNumber(off))
		if size > 0.0001 && size <= 100000.0 {
			c.Size = size
			if c.Body != nil {
				eff := c.EffectiveSize()
				c.Body.HalfExtents = rl.NewVector3(eff, eff, eff)
			}
		}
		return 0
	}))

	// get_scale() -> number
	sm.L.SetField(cubeTable, "get_scale", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.Scale))
		return 1
	}))

	// set_scale(scale)
	sm.L.SetField(cubeTable, "set_scale", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		scale := float32(L.CheckNumber(off))
		if scale > 0.0001 && scale <= 100000.0 {
			c.Scale = scale
			if c.Body != nil {
				eff := c.EffectiveSize()
				c.Body.HalfExtents = rl.NewVector3(eff, eff, eff)
			}
		}
		return 0
	}))

	// get_mode() -> string ("faces", "gradient", "chroma")
	sm.L.SetField(cubeTable, "get_mode", sm.L.NewFunction(func(L *lua.LState) int {
		switch c.Mode {
		case cube.ModeFaceColors:
			L.Push(lua.LString("faces"))
		case cube.ModeVertexGradient:
			L.Push(lua.LString("gradient"))
		case cube.ModeChromaCycle:
			L.Push(lua.LString("chroma"))
		default:
			L.Push(lua.LString("faces"))
		}
		return 1
	}))

	// set_mode(modeStr)
	sm.L.SetField(cubeTable, "set_mode", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		m := strings.ToLower(L.CheckString(off))
		switch m {
		case "faces", "face", "0":
			c.SetMode(cube.ModeFaceColors)
		case "gradient", "spectrum", "1":
			c.SetMode(cube.ModeVertexGradient)
		case "chroma", "cycle", "2":
			c.SetMode(cube.ModeChromaCycle)
		}
		return 0
	}))

	// cycle_mode()
	sm.L.SetField(cubeTable, "cycle_mode", sm.L.NewFunction(func(L *lua.LState) int {
		c.CycleMode()
		return 0
	}))

	// get_auto_rotate() -> bool
	sm.L.SetField(cubeTable, "get_auto_rotate", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(c.AutoRotate))
		return 1
	}))

	// set_auto_rotate(bool)
	sm.L.SetField(cubeTable, "set_auto_rotate", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		c.AutoRotate = L.CheckBool(off)
		return 0
	}))

	// toggle_auto_rotate()
	sm.L.SetField(cubeTable, "toggle_auto_rotate", sm.L.NewFunction(func(L *lua.LState) int {
		c.ToggleAutoRotate()
		return 0
	}))

	// get_wireframe() -> bool
	sm.L.SetField(cubeTable, "get_wireframe", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(c.ShowWireframe))
		return 1
	}))

	// set_wireframe(bool)
	sm.L.SetField(cubeTable, "set_wireframe", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		c.ShowWireframe = L.CheckBool(off)
		return 0
	}))

	// toggle_wireframe()
	sm.L.SetField(cubeTable, "toggle_wireframe", sm.L.NewFunction(func(L *lua.LState) int {
		c.ToggleWireframe()
		return 0
	}))

	// inspect() -> string
	sm.L.SetField(cubeTable, "inspect", sm.L.NewFunction(func(L *lua.LState) int {
		canLift := "Yes (<50kg)"
		if c.Mass >= human.MaxPickupMass {
			canLift = "No (>=50kg)"
		}
		info := fmt.Sprintf("[%s (%s)] Pos:(%.1f, %.1f, %.1f) Mass:%.1fkg (Liftable:%s) Size:%.2f Scale:%.2f Mode:%s",
			c.Name, c.ID, c.Position.X, c.Position.Y, c.Position.Z, c.Mass, canLift, c.Size, c.Scale, c.Mode.String())
		L.Push(lua.LString(info))
		return 1
	}))

	// get_mass() -> number
	sm.L.SetField(cubeTable, "get_mass", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.Mass))
		return 1
	}))

	// set_mass(mass)
	sm.L.SetField(cubeTable, "set_mass", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		mass := float32(L.CheckNumber(off))
		if mass >= 0.0 && mass <= 100000.0 {
			c.Mass = mass
		}
		return 0
	}))

	// can_pickup() -> bool
	sm.L.SetField(cubeTable, "can_pickup", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(c.Mass < human.MaxPickupMass))
		return 1
	}))

	// is_held() -> bool
	sm.L.SetField(cubeTable, "is_held", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(c.IsCarried))
		return 1
	}))

	// get_velocity() -> vx, vy, vz
	sm.L.SetField(cubeTable, "get_velocity", sm.L.NewFunction(func(L *lua.LState) int {
		vel := c.GetVelocity()
		L.Push(lua.LNumber(vel.X))
		L.Push(lua.LNumber(vel.Y))
		L.Push(lua.LNumber(vel.Z))
		return 3
	}))

	// set_velocity(vx, vy, vz)
	sm.L.SetField(cubeTable, "set_velocity", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		vx := float32(L.CheckNumber(off))
		vy := float32(L.CheckNumber(off + 1))
		vz := float32(L.CheckNumber(off + 2))
		c.SetVelocity(rl.NewVector3(vx, vy, vz))
		return 0
	}))

	// apply_impulse(ix, iy, iz)
	sm.L.SetField(cubeTable, "apply_impulse", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		ix := float32(L.CheckNumber(off))
		iy := float32(L.CheckNumber(off + 1))
		iz := float32(L.CheckNumber(off + 2))
		c.ApplyImpulse(rl.NewVector3(ix, iy, iz))
		return 0
	}))

	// get_bounciness() -> number
	sm.L.SetField(cubeTable, "get_bounciness", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.GetRestitution()))
		return 1
	}))

	// set_bounciness(restitution)
	sm.L.SetField(cubeTable, "set_bounciness", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		restitution := float32(L.CheckNumber(off))
		c.SetRestitution(restitution)
		return 0
	}))

	// get_friction() -> number
	sm.L.SetField(cubeTable, "get_friction", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.GetFriction()))
		return 1
	}))

	// set_friction(friction)
	sm.L.SetField(cubeTable, "set_friction", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		friction := float32(L.CheckNumber(off))
		c.SetFriction(friction)
		return 0
	}))

	// is_grounded() -> bool
	sm.L.SetField(cubeTable, "is_grounded", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(c.IsGrounded()))
		return 1
	}))

	// is_static() -> bool
	sm.L.SetField(cubeTable, "is_static", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(c.IsStatic()))
		return 1
	}))

	// set_static(bool)
	sm.L.SetField(cubeTable, "set_static", sm.L.NewFunction(func(L *lua.LState) int {
		off := argOffset(L)
		isStatic := L.CheckBool(off)
		c.SetStatic(isStatic)
		return 0
	}))

	// toggle_collision_box() -> bool
	sm.L.SetField(cubeTable, "toggle_collision_box", sm.L.NewFunction(func(L *lua.LState) int {
		c.ShowCollisionBox = !c.ShowCollisionBox
		L.Push(lua.LBool(c.ShowCollisionBox))
		return 1
	}))

	// get_collision_box() -> table
	sm.L.SetField(cubeTable, "get_collision_box", sm.L.NewFunction(func(L *lua.LState) int {
		box := c.GetBoundingBox()
		t := sm.L.NewTable()
		minT := sm.L.NewTable()
		minT.RawSetString("x", lua.LNumber(box.Min.X))
		minT.RawSetString("y", lua.LNumber(box.Min.Y))
		minT.RawSetString("z", lua.LNumber(box.Min.Z))
		maxT := sm.L.NewTable()
		maxT.RawSetString("x", lua.LNumber(box.Max.X))
		maxT.RawSetString("y", lua.LNumber(box.Max.Y))
		maxT.RawSetString("z", lua.LNumber(box.Max.Z))
		t.RawSetString("min", minT)
		t.RawSetString("max", maxT)
		L.Push(t)
		return 1
	}))

	return cubeTable
}

func (sm *ScriptManager) registerCameraAPI() {
	camTable := sm.L.NewTable()

	// camera.get_distance() -> number
	sm.L.SetField(camTable, "get_distance", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(sm.camera.Distance))
		return 1
	}))

	// camera.set_distance(dist)
	sm.L.SetField(camTable, "set_distance", sm.L.NewFunction(func(L *lua.LState) int {
		dist := float32(L.CheckNumber(1))
		if dist >= sm.camera.MinDistance && dist <= sm.camera.MaxDistance {
			sm.camera.Distance = dist
		}
		return 0
	}))

	// camera.get_angles() -> yaw, pitch
	sm.L.SetField(camTable, "get_angles", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(sm.camera.Yaw))
		L.Push(lua.LNumber(sm.camera.Pitch))
		return 2
	}))

	// camera.set_angles(yaw, pitch)
	sm.L.SetField(camTable, "set_angles", sm.L.NewFunction(func(L *lua.LState) int {
		sm.camera.Yaw = float32(L.CheckNumber(1))
		sm.camera.Pitch = float32(L.CheckNumber(2))
		return 0
	}))

	// camera.reset()
	sm.L.SetField(camTable, "reset", sm.L.NewFunction(func(L *lua.LState) int {
		sm.camera.Reset()
		return 0
	}))

	sm.L.SetGlobal("camera", camTable)
}

func (sm *ScriptManager) registerInputAPI() {
	inputTable := sm.L.NewTable()

	// input.is_key_down(keyName)
	sm.L.SetField(inputTable, "is_key_down", sm.L.NewFunction(func(L *lua.LState) int {
		keyName := strings.ToUpper(L.CheckString(1))
		keyCode := keyNameToCode(keyName)
		isDown := rl.IsKeyDown(keyCode)
		L.Push(lua.LBool(isDown))
		return 1
	}))

	// input.is_key_pressed(keyName)
	sm.L.SetField(inputTable, "is_key_pressed", sm.L.NewFunction(func(L *lua.LState) int {
		keyName := strings.ToUpper(L.CheckString(1))
		keyCode := keyNameToCode(keyName)
		isPressed := rl.IsKeyPressed(keyCode)
		L.Push(lua.LBool(isPressed))
		return 1
	}))

	sm.L.SetGlobal("input", inputTable)
}

func (sm *ScriptManager) registerEngineAPI() {
	engineTable := sm.L.NewTable()

	// engine.log(msg)
	sm.L.SetField(engineTable, "log", sm.L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		sm.AddLog("[Lua] " + msg)
		return 0
	}))

	// engine.get_time() -> number
	sm.L.SetField(engineTable, "get_time", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(sm.time))
		return 1
	}))

	// engine.get_fps() -> number
	sm.L.SetField(engineTable, "get_fps", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(rl.GetFPS()))
		return 1
	}))

	// engine.get_delta_time() -> number
	sm.L.SetField(engineTable, "get_delta_time", sm.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(sm.dt))
		return 1
	}))

	// engine.reload_script()
	sm.L.SetField(engineTable, "reload_script", sm.L.NewFunction(func(L *lua.LState) int {
		_ = sm.ReloadScript()
		return 0
	}))

	// engine.toggle_physics_debug() -> bool
	sm.L.SetField(engineTable, "toggle_physics_debug", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.OnTogglePhysicsDebug != nil {
			res := sm.OnTogglePhysicsDebug()
			L.Push(lua.LBool(res))
			return 1
		}
		L.Push(lua.LBool(false))
		return 1
	}))

	sm.L.SetGlobal("engine", engineTable)
}

var customSaveDir string

// SetSaveDir sets a custom directory for saving map snapshots (useful for tests).
func SetSaveDir(dir string) {
	customSaveDir = dir
}

// getSaveDir locates or creates the saves directory relative to workspace or test root.
func getSaveDir() string {
	if customSaveDir != "" {
		_ = os.MkdirAll(customSaveDir, 0755)
		return customSaveDir
	}
	candidates := []string{"saves", "../../saves", "../saves"}
	for _, cand := range candidates {
		if info, err := os.Stat(cand); err == nil && info.IsDir() {
			if abs, err := filepath.Abs(cand); err == nil {
				return abs
			}
			return cand
		}
	}
	if _, err := os.Stat("maps"); err == nil {
		_ = os.MkdirAll("saves", 0755)
		if abs, err := filepath.Abs("saves"); err == nil {
			return abs
		}
		return "saves"
	}
	if _, err := os.Stat("../../maps"); err == nil {
		_ = os.MkdirAll("../../saves", 0755)
		if abs, err := filepath.Abs("../../saves"); err == nil {
			return abs
		}
		return "../../saves"
	}
	if _, err := os.Stat("../maps"); err == nil {
		_ = os.MkdirAll("../saves", 0755)
		if abs, err := filepath.Abs("../saves"); err == nil {
			return abs
		}
		return "../saves"
	}
	_ = os.MkdirAll("saves", 0755)
	if abs, err := filepath.Abs("saves"); err == nil {
		return abs
	}
	return "saves"
}

// LoadMap loads a TMX map by relative path or name.
// When a map is loaded, the existing active map is serialized to JSON snapshot in saves/
// and promptly disposed of before the new map becomes active.
func (sm *ScriptManager) LoadMap(path string) (*tiled.Map3D, error) {
	tmxMap, err := tiled.LoadTMX(path)
	if err != nil {
		candidates := []string{
			"maps/" + path,
			path + ".tmx",
			"maps/" + path + ".tmx",
			"../../maps/" + path,
			"../../maps/" + path + ".tmx",
			"../maps/" + path,
			"../maps/" + path + ".tmx",
		}
		for _, cand := range candidates {
			if m, err2 := tiled.LoadTMX(cand); err2 == nil {
				tmxMap = m
				path = cand
				err = nil
				break
			}
		}
	}
	if err != nil {
		return nil, err
	}

	// 1. Serialize existing map to JSON and save before disposing
	oldMap := sm.CurrentMap
	if oldMap != nil {
		saveDir := getSaveDir()
		baseName := "map"
		if oldMap.SourceMap != nil && oldMap.SourceMap.FilePath != "" {
			base := filepath.Base(oldMap.SourceMap.FilePath)
			ext := filepath.Ext(base)
			baseName = strings.TrimSuffix(base, ext)
		}
		ts := time.Now().Format("20060102_150405")
		snapshotPath := filepath.Join(saveDir, fmt.Sprintf("%s_snapshot_%s.json", baseName, ts))
		prevMapPath := filepath.Join(saveDir, "previous_map.json")

		if err := oldMap.SaveJSON(snapshotPath); err != nil {
			sm.AddLog(fmt.Sprintf("Warning: failed saving snapshot %s: %v", snapshotPath, err))
		} else {
			sm.AddLog(fmt.Sprintf("Saved map snapshot to %s", snapshotPath))
		}
		_ = oldMap.SaveJSON(prevMapPath)
	}

	// 2. Build new 3D map
	m3d := tiled.NewMap3D(tmxMap, 1.0, -2.2)
	sm.CurrentMap = m3d

	// 3. Notify scene callback to switch geometry, solid boxes, and spawns
	if sm.OnMapLoad != nil {
		if err := sm.OnMapLoad(m3d); err != nil {
			return nil, err
		}
	}

	// 4. Promptly dispose of the old map resources
	if oldMap != nil {
		oldMap.Dispose()
	}

	sm.AddLog(fmt.Sprintf("Loaded TMX map: %s (%dx%d)", path, tmxMap.Width, tmxMap.Height))
	return m3d, nil
}

func (sm *ScriptManager) registerMapAPI() {
	mapTable := sm.L.NewTable()

	// loadFunc handles map loading (saves current map to JSON snapshot and disposes it)
	loadFunc := sm.L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		_, err := sm.LoadMap(path)
		if err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LBool(true))
		return 1
	})

	// Global load(filePath) -> bool, err
	sm.L.SetGlobal("load", loadFunc)

	// map.load(filePath) -> bool, err (backward compatibility)
	sm.L.SetField(mapTable, "load", loadFunc)

	// map.get_title() -> string
	sm.L.SetField(mapTable, "get_title", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.CurrentMap == nil || sm.CurrentMap.SourceMap == nil {
			L.Push(lua.LString("No map loaded"))
			return 1
		}
		title := sm.CurrentMap.SourceMap.GetProperty("title", sm.CurrentMap.SourceMap.FilePath)
		L.Push(lua.LString(title))
		return 1
	}))

	// map.get_size() -> width, height
	sm.L.SetField(mapTable, "get_size", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.CurrentMap == nil || sm.CurrentMap.SourceMap == nil {
			L.Push(lua.LNumber(0))
			L.Push(lua.LNumber(0))
			return 2
		}
		L.Push(lua.LNumber(sm.CurrentMap.SourceMap.Width))
		L.Push(lua.LNumber(sm.CurrentMap.SourceMap.Height))
		return 2
	}))

	// map.get_tile(layerName, col, row) -> gid
	sm.L.SetField(mapTable, "get_tile", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.CurrentMap == nil || sm.CurrentMap.SourceMap == nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		layerName := L.CheckString(1)
		col := L.CheckInt(2)
		row := L.CheckInt(3)

		layer := sm.CurrentMap.SourceMap.GetLayerByName(layerName)
		if layer == nil || row < 0 || row >= len(layer.Tiles) || col < 0 || col >= len(layer.Tiles[row]) {
			L.Push(lua.LNumber(0))
			return 1
		}

		L.Push(lua.LNumber(layer.Tiles[row][col]))
		return 1
	}))

	// map.set_tile(layerName, col, row, gid) -> bool
	sm.L.SetField(mapTable, "set_tile", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.CurrentMap == nil || sm.CurrentMap.SourceMap == nil {
			L.Push(lua.LBool(false))
			return 1
		}
		layerName := L.CheckString(1)
		col := L.CheckInt(2)
		row := L.CheckInt(3)
		gid := uint32(L.CheckInt(4))

		layer := sm.CurrentMap.SourceMap.GetLayerByName(layerName)
		if layer == nil || row < 0 || row >= len(layer.Tiles) || col < 0 || col >= len(layer.Tiles[row]) {
			L.Push(lua.LBool(false))
			return 1
		}

		layer.Tiles[row][col] = gid
		sm.CurrentMap.Build()
		if sm.OnMapLoad != nil {
			_ = sm.OnMapLoad(sm.CurrentMap)
		}
		L.Push(lua.LBool(true))
		return 1
	}))

	// map.reload() -> bool
	sm.L.SetField(mapTable, "reload", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.CurrentMap == nil || sm.CurrentMap.SourceMap == nil || sm.CurrentMap.SourceMap.FilePath == "" {
			L.Push(lua.LBool(false))
			return 1
		}
		path := sm.CurrentMap.SourceMap.FilePath
		_, err := sm.LoadMap(path)
		if err != nil {
			sm.AddLog("Map reload error: " + err.Error())
			L.Push(lua.LBool(false))
			return 1
		}
		sm.AddLog("Map reloaded: " + path)
		L.Push(lua.LBool(true))
		return 1
	}))

	sm.L.SetGlobal("map", mapTable)
}

func (sm *ScriptManager) registerPlayerAPI() {
	playerTable := sm.L.NewTable()

	// player.get_position() -> x, y, z
	sm.L.SetField(playerTable, "get_position", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			L.Push(lua.LNumber(0))
			L.Push(lua.LNumber(0))
			L.Push(lua.LNumber(0))
			return 3
		}
		L.Push(lua.LNumber(sm.fps.Position.X))
		L.Push(lua.LNumber(sm.fps.Position.Y))
		L.Push(lua.LNumber(sm.fps.Position.Z))
		return 3
	}))

	// player.set_position(x, y, z)
	sm.L.SetField(playerTable, "set_position", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			return 0
		}
		x := float32(L.CheckNumber(1))
		y := float32(L.CheckNumber(2))
		z := float32(L.CheckNumber(3))
		sm.fps.Teleport(rl.NewVector3(x, y, z))
		return 0
	}))

	// player.teleport(x, y, z)
	sm.L.SetField(playerTable, "teleport", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			return 0
		}
		x := float32(L.CheckNumber(1))
		y := float32(L.CheckNumber(2))
		z := float32(L.CheckNumber(3))
		sm.fps.Teleport(rl.NewVector3(x, y, z))
		return 0
	}))

	// player.get_angles() -> yaw, pitch
	sm.L.SetField(playerTable, "get_angles", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			L.Push(lua.LNumber(0))
			L.Push(lua.LNumber(0))
			return 2
		}
		L.Push(lua.LNumber(sm.fps.Yaw))
		L.Push(lua.LNumber(sm.fps.Pitch))
		return 2
	}))

	// player.set_angles(yaw, pitch)
	sm.L.SetField(playerTable, "set_angles", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			return 0
		}
		sm.fps.Yaw = float32(L.CheckNumber(1))
		sm.fps.Pitch = float32(L.CheckNumber(2))
		return 0
	}))

	// player.get_speed() -> number
	sm.L.SetField(playerTable, "get_speed", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		L.Push(lua.LNumber(sm.fps.WalkSpeed))
		return 1
	}))

	// player.set_speed(speed)
	sm.L.SetField(playerTable, "set_speed", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			return 0
		}
		speed := float32(L.CheckNumber(1))
		if speed > 0 && speed < 100 {
			sm.fps.WalkSpeed = speed
		}
		return 0
	}))

	// player.is_grounded() -> bool
	sm.L.SetField(playerTable, "is_grounded", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			L.Push(lua.LBool(false))
			return 1
		}
		L.Push(lua.LBool(sm.fps.IsGrounded))
		return 1
	}))

	// player.jump()
	sm.L.SetField(playerTable, "jump", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps != nil && sm.fps.IsGrounded {
			sm.fps.Velocity.Y = sm.fps.JumpSpeed
			sm.fps.IsGrounded = false
		}
		return 0
	}))

	// player.get_held_prop() -> table or nil
	sm.L.SetField(playerTable, "get_held_prop", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil || sm.fps.Human == nil || sm.fps.Human.HeldProp == nil {
			L.Push(lua.LNil)
			return 1
		}
		held := sm.fps.Human.HeldProp
		t := sm.L.NewTable()
		sm.L.SetField(t, "id", lua.LString(held.GetID()))
		sm.L.SetField(t, "name", lua.LString(held.GetName()))
		sm.L.SetField(t, "mass", lua.LNumber(held.GetMass()))
		L.Push(t)
		return 1
	}))

	// player.drop_prop() -> bool
	sm.L.SetField(playerTable, "drop_prop", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil || sm.fps.Human == nil || sm.fps.Human.HeldProp == nil {
			L.Push(lua.LBool(false))
			return 1
		}
		sm.fps.Human.DropProp()
		L.Push(lua.LBool(true))
		return 1
	}))

	// player.throw_prop([power]) -> bool
	sm.L.SetField(playerTable, "throw_prop", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil || sm.fps.Human == nil || sm.fps.Human.HeldProp == nil {
			L.Push(lua.LBool(false))
			return 1
		}
		power := float32(19.0)
		if L.GetTop() >= 1 {
			power = float32(L.CheckNumber(1))
		}
		if sm.OnThrowProp != nil {
			sm.OnThrowProp(power)
			L.Push(lua.LBool(true))
			return 1
		}
		sm.fps.Human.DropProp()
		L.Push(lua.LBool(true))
		return 1
	}))

	// player.get_step_height() -> number
	sm.L.SetField(playerTable, "get_step_height", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			L.Push(lua.LNumber(0.65))
			return 1
		}
		L.Push(lua.LNumber(sm.fps.GetStepHeight()))
		return 1
	}))

	// player.set_step_height(height) -> bool
	sm.L.SetField(playerTable, "set_step_height", sm.L.NewFunction(func(L *lua.LState) int {
		if sm.fps == nil {
			L.Push(lua.LBool(false))
			return 1
		}
		h := float32(L.CheckNumber(1))
		sm.fps.SetStepHeight(h)
		L.Push(lua.LBool(true))
		return 1
	}))

	sm.L.SetGlobal("player", playerTable)
}

func keyNameToCode(name string) int32 {
	switch name {
	case "SPACE":
		return rl.KeySpace
	case "R":
		return rl.KeyR
	case "W":
		return rl.KeyW
	case "G":
		return rl.KeyG
	case "C":
		return rl.KeyC
	case "F":
		return rl.KeyF
	case "H":
		return rl.KeyH
	case "UP":
		return rl.KeyUp
	case "DOWN":
		return rl.KeyDown
	case "LEFT":
		return rl.KeyLeft
	case "RIGHT":
		return rl.KeyRight
	case "1":
		return rl.KeyOne
	case "2":
		return rl.KeyTwo
	case "3":
		return rl.KeyThree
	default:
		if len(name) == 1 {
			return int32(name[0])
		}
		return 0
	}
}

// SummonCube creates a new cube entity, sets it as the active global target, and returns the entity and its Lua table.
func (sm *ScriptManager) SummonCube(cfg SummonConfig) (*cube.RainbowCube, *lua.LTable) {
	var newCube *cube.RainbowCube
	if sm.OnSummonCube != nil {
		newCube = sm.OnSummonCube(cfg)
	} else {
		// Fallback creation for headless or unit test execution
		pos := rl.NewVector3(0, 0, 0)
		if cfg.Position != nil {
			pos = *cfg.Position
		}
		size := cfg.Size
		if size <= 0 {
			size = 0.75
		}
		mass := cfg.Mass
		if mass <= 0 {
			mass = 25.0
		}
		name := cfg.Name
		if name == "" {
			name = "SummonedCube"
		}
		id := fmt.Sprintf("cube_summon_%d", time.Now().UnixNano())
		newCube = cube.NewRainbowCubeWithConfig(id, name, size, pos, cfg.Mode)
		newCube.Mass = mass
	}

	if newCube == nil {
		return nil, nil
	}

	// Update main cube if missing or deleted
	if sm.cube == nil || sm.cube.IsDeleted {
		sm.SetMainCube(newCube)
	}

	// Explicitly bind the newly summoned entity as the global 'target'
	sm.SetGlobalTarget(newCube)
	targetTable, ok := sm.L.GetGlobal("target").(*lua.LTable)
	if !ok || targetTable == nil {
		targetTable = sm.createCubeTable(newCube)
	}
	return newCube, targetTable
}

func (sm *ScriptManager) registerSummonAPI() {
	summonFunc := sm.L.NewFunction(func(L *lua.LState) int {
		var cfg SummonConfig
		top := L.GetTop()

		getFloat := func(idx int) (float32, bool) {
			if idx > top {
				return 0, false
			}
			v := L.Get(idx)
			if v.Type() == lua.LTNumber {
				return float32(v.(lua.LNumber)), true
			}
			if v.Type() == lua.LTString {
				if f, err := strconv.ParseFloat(v.String(), 32); err == nil {
					return float32(f), true
				}
			}
			return 0, false
		}

		if top >= 1 && L.Get(1).Type() == lua.LTTable {
			tbl := L.ToTable(1)
			if nameVal := tbl.RawGetString("name"); nameVal.Type() == lua.LTString {
				cfg.Name = nameVal.String()
			}
			hasX := tbl.RawGetString("x").Type() == lua.LTNumber
			hasY := tbl.RawGetString("y").Type() == lua.LTNumber
			hasZ := tbl.RawGetString("z").Type() == lua.LTNumber
			if hasX && hasY && hasZ {
				x := float32(tbl.RawGetString("x").(lua.LNumber))
				y := float32(tbl.RawGetString("y").(lua.LNumber))
				z := float32(tbl.RawGetString("z").(lua.LNumber))
				pos := rl.NewVector3(x, y, z)
				cfg.Position = &pos
			}
			if sizeVal := tbl.RawGetString("size"); sizeVal.Type() == lua.LTNumber {
				cfg.Size = float32(sizeVal.(lua.LNumber))
			}
			if massVal := tbl.RawGetString("mass"); massVal.Type() == lua.LTNumber {
				cfg.Mass = float32(massVal.(lua.LNumber))
			}
			if modeVal := tbl.RawGetString("mode"); modeVal.Type() == lua.LTString {
				switch strings.ToLower(modeVal.String()) {
				case "gradient":
					cfg.Mode = cube.ModeVertexGradient
				case "chroma":
					cfg.Mode = cube.ModeChromaCycle
				default:
					cfg.Mode = cube.ModeFaceColors
				}
			}
		} else {
			// Positional arguments:
			// summon([name], [x, y, z], [size], [mass], [mode])
			if top >= 1 && L.Get(1).Type() == lua.LTString {
				cfg.Name = L.CheckString(1)
			}
			x, hasX := getFloat(2)
			y, hasY := getFloat(3)
			z, hasZ := getFloat(4)
			if hasX && hasY && hasZ {
				pos := rl.NewVector3(x, y, z)
				cfg.Position = &pos

				if sVal, hasS := getFloat(5); hasS {
					cfg.Size = sVal
				}
				if mVal, hasM := getFloat(6); hasM {
					cfg.Mass = mVal
				}
				if top >= 7 && L.Get(7).Type() == lua.LTString {
					switch strings.ToLower(L.CheckString(7)) {
					case "gradient":
						cfg.Mode = cube.ModeVertexGradient
					case "chroma":
						cfg.Mode = cube.ModeChromaCycle
					default:
						cfg.Mode = cube.ModeFaceColors
					}
				}
			} else if hasX && !hasY {
				// E.g. summon("Name", size)
				cfg.Size = x
			} else if hasX && hasY && !hasZ {
				// E.g. summon("Name", size, mass)
				cfg.Size = x
				cfg.Mass = y
			}
		}

		newCube, cubeTable := sm.SummonCube(cfg)
		if newCube == nil || cubeTable == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("Failed to summon entity"))
			return 2
		}

		L.Push(cubeTable)
		pos := newCube.GetPosition()
		msg := fmt.Sprintf("Summoned entity '%s' (ID: %s, Mass: %.1fkg) at (%.2f, %.2f, %.2f). Target set to new instance.",
			newCube.Name, newCube.ID, newCube.Mass, pos.X, pos.Y, pos.Z)
		L.Push(lua.LString(msg))
		return 2
	})

	sm.L.SetGlobal("summon", summonFunc)
	sm.L.SetGlobal("spawn", summonFunc)

	engineVal := sm.L.GetGlobal("engine")
	if engineTable, ok := engineVal.(*lua.LTable); ok {
		sm.L.SetField(engineTable, "summon", summonFunc)
		sm.L.SetField(engineTable, "spawn", summonFunc)
	}
}
