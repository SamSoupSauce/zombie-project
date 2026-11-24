package console

import (
	"fmt"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	lua "github.com/yuin/gopher-lua"
	"zombie-project/internal/scripting"
)

// LineType classifies output messages for color-coded rendering.
type LineType int

const (
	LineInput LineType = iota
	LineOutput
	LineInfo
	LineError
	LineSystem
)

// ConsoleLine represents a single line in the console buffer.
type ConsoleLine struct {
	Text      string
	Type      LineType
	Timestamp string
}

// Console implements an in-game Quake-style dropdown developer REPL console.
type Console struct {
	IsOpen            bool
	AnimationProgress float32 // 0.0 (closed) to 1.0 (open)
	SlideSpeed        float32

	InputBuffer  []rune
	CursorPos    int
	History      []string
	HistoryIndex int // -1 means currently editing new input

	Output       []ConsoleLine
	MaxLines     int
	ScrollOffset int // 0 means scrolled to bottom

	BlinkTimer float32
	ShowCursor bool

	scriptManager *scripting.ScriptManager

	// Auto-completion candidates
	completions []string

	// Custom built-in command handlers
	builtins map[string]func(args []string) string
}

// NewConsole initializes the in-game Lua console.
func NewConsole(sm *scripting.ScriptManager) *Console {
	c := &Console{
		IsOpen:            false,
		AnimationProgress: 0.0,
		SlideSpeed:        8.0,
		InputBuffer:       make([]rune, 0),
		CursorPos:         0,
		History:           make([]string, 0),
		HistoryIndex:      -1,
		Output:            make([]ConsoleLine, 0),
		MaxLines:          250,
		ScrollOffset:      0,
		BlinkTimer:        0.0,
		ShowCursor:        true,
		scriptManager:     sm,
		completions: []string{
			"help", "clear", "reload", "reset", "history", "delete", "destroy", "summon", "spawn", "step_height",
			"summon()", "spawn()", "engine.summon()", "engine.spawn()",
			"player.get_position()", "player.set_position(", "player.teleport(", "player.jump()", "player.get_speed()",
			"player.get_step_height()", "player.set_step_height(",
			"load maps/zombie_outpost.tmx", "load maps/city_ruins.tmx", "load maps/kitchen_sink.tmx", "load maps/conestoga.tmx", "load kitchen_sink", "load conestoga", "load zombie_outpost", "load city_ruins",
			"load(\"maps/zombie_outpost.tmx\")", "load(\"maps/city_ruins.tmx\")", "load(\"maps/kitchen_sink.tmx\")", "load(\"maps/conestoga.tmx\")", "load(\"kitchen_sink\")", "load(\"conestoga\")", "load(\"",
			"map load maps/zombie_outpost.tmx", "map load maps/city_ruins.tmx", "map load maps/kitchen_sink.tmx", "map load maps/conestoga.tmx", "map load kitchen_sink", "map load conestoga", "map reload", "map info",
			"map.load(\"maps/zombie_outpost.tmx\")", "map.load(\"maps/city_ruins.tmx\")", "map.load(\"maps/kitchen_sink.tmx\")", "map.load(\"maps/conestoga.tmx\")", "map.load(\"kitchen_sink\")", "map.load(\"conestoga\")",
			"map.get_title()", "map.get_size()", "map.get_tile(", "map.set_tile(", "map.reload()",
			"target.name", "target.id", "target.inspect()",
			"target.delete()", "target:delete()", "target.destroy()", "target:destroy()",
			"target.get_rotation()", "target.set_rotation(", "target.rotate(",
			"target.get_position()", "target.set_position(",
			"target.get_size()", "target.set_size(",
			"target.get_scale()", "target.set_scale(",
			"target.get_mode()", "target.set_mode(\"faces\")", "target.set_mode(\"gradient\")", "target.set_mode(\"chroma\")",
			"target.cycle_mode()", "target.get_speed()", "target.set_speed(",
			"target.set_auto_rotate(", "target.toggle_auto_rotate()",
			"target.set_wireframe(", "target.toggle_wireframe()",
			"cube.delete()", "cube:delete()",
			"cube.get_rotation()", "cube.set_rotation(", "cube.rotate(",
			"cube.get_position()", "cube.set_position(",
			"cube.get_size()", "cube.set_size(",
			"cube.get_scale()", "cube.set_scale(",
			"cube.get_mode()", "cube.set_mode(\"faces\")", "cube.set_mode(\"gradient\")", "cube.set_mode(\"chroma\")",
			"cube.cycle_mode()", "cube.get_speed()", "cube.set_speed(",
			"cube.set_auto_rotate(", "cube.toggle_auto_rotate()",
			"cube.set_wireframe(", "cube.toggle_wireframe()",
			"camera.get_distance()", "camera.set_distance(",
			"camera.get_angles()", "camera.set_angles(", "camera.reset()",
			"engine.log(\"", "engine.get_time()", "engine.get_fps()", "engine.reload_script()",
			"print(",
		},
	}

	c.initBuiltins()
	c.hookLuaPrint()

	// Welcome banner
	c.AddLine("=== ZOMBIE ENGINE 3D - LUA DEVELOPER CONSOLE ===", LineSystem)
	c.AddLine("Type 'help' for quick reference commands or enter any Lua expression.", LineInfo)
	c.AddLine("Examples: cube.set_mode(\"chroma\") | cube.set_size(2.0) | camera.set_distance(4)", LineInfo)

	return c
}

// hookLuaPrint redirects Lua's standard print() to this console and stdout.
func (c *Console) hookLuaPrint() {
	if c.scriptManager == nil || c.scriptManager.L == nil {
		return
	}

	c.scriptManager.L.SetGlobal("print", c.scriptManager.L.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		var parts []string
		for i := 1; i <= top; i++ {
			parts = append(parts, L.ToStringMeta(L.Get(i)).String())
		}
		line := strings.Join(parts, "\t")
		c.AddLine(line, LineOutput)
		c.scriptManager.AddLog("[Lua Print] " + line)
		fmt.Println("[Lua Print]", line)
		return 0
	}))
}

func (c *Console) initBuiltins() {
	c.builtins = map[string]func(args []string) string{
		"help": func(args []string) string {
			c.AddLine("--- Built-in Console Commands ---", LineSystem)
			c.AddLine("  help               - Show this help reference", LineInfo)
			c.AddLine("  clear              - Clear console output history", LineInfo)
			c.AddLine("  reload             - Hot-reload scripts/cube_behavior.lua", LineInfo)
			c.AddLine("  reset              - Reset cube and camera to defaults", LineInfo)
			c.AddLine("  step_height [val]  - Get/set ledge & staircase walk-up threshold (default: 0.65)", LineInfo)
			c.AddLine("  load <path/name>   - Load TMX map (auto-saves existing snapshot & disposes it)", LineInfo)
			c.AddLine("  summon [name] [pos] - Summon new entity in front of player (spawn alias)", LineInfo)
			c.AddLine("  delete / destroy   - Delete currently targeted entity (target:delete())", LineInfo)
			c.AddLine("  history            - Display recent command history", LineInfo)
			c.AddLine("  map load <path>    - Import & build 3D level from TMX file", LineInfo)
			c.AddLine("  map reload         - Reload active TMX map", LineInfo)
			c.AddLine("  map info           - Show active TMX map metadata", LineInfo)
			c.AddLine("--- Lua Scripting API Quick Reference ---", LineSystem)
			c.AddLine("  load(\"maps/...\")  - Load TMX map (auto-saves existing map & disposes)", LineInfo)
			c.AddLine("  map.load(\"...\")  - Alias for load() (backward compatibility)", LineInfo)
			c.AddLine("  summon([name], [x,y,z]) - Summon entity into scene and sets 'target'", LineInfo)
			c.AddLine("  target:delete()    - Delete targeted entity (target.delete())", LineInfo)
			c.AddLine("  map.get_title() | map.get_size() | map.get_tile() | map.set_tile()", LineInfo)
			c.AddLine("  cube.set_mode(\"faces\"|\"gradient\"|\"chroma\")", LineInfo)
			c.AddLine("  cube.set_size(num) | cube.set_position(x, y, z)", LineInfo)
			c.AddLine("  cube.rotate(pitch, yaw, roll) | cube.set_speed(x, y, z)", LineInfo)
			c.AddLine("  cube.set_wireframe(bool) | cube.set_auto_rotate(bool)", LineInfo)
			c.AddLine("  camera.set_distance(dist) | camera.reset()", LineInfo)
			c.AddLine("  engine.log(\"msg\") | print(...) | 1 + 1", LineInfo)
			return ""
		},
		"summon": func(args []string) string {
			if c.scriptManager == nil || c.scriptManager.L == nil {
				return "Script manager not available."
			}
			var luaCode string
			switch len(args) {
			case 0:
				luaCode = `_summon_inst, _summon_msg = summon()`
			case 1:
				luaCode = fmt.Sprintf(`_summon_inst, _summon_msg = summon(%q)`, args[0])
			case 2:
				luaCode = fmt.Sprintf(`_summon_inst, _summon_msg = summon(%q, nil, nil, nil, %s)`, args[0], args[1])
			case 3:
				luaCode = fmt.Sprintf(`_summon_inst, _summon_msg = summon(%q, nil, nil, nil, %s, %s)`, args[0], args[1], args[2])
			case 4:
				luaCode = fmt.Sprintf(`_summon_inst, _summon_msg = summon(%q, %s, %s, %s)`, args[0], args[1], args[2], args[3])
			case 5:
				luaCode = fmt.Sprintf(`_summon_inst, _summon_msg = summon(%q, %s, %s, %s, %s)`, args[0], args[1], args[2], args[3], args[4])
			case 6:
				luaCode = fmt.Sprintf(`_summon_inst, _summon_msg = summon(%q, %s, %s, %s, %s, %s)`, args[0], args[1], args[2], args[3], args[4], args[5])
			default:
				luaCode = fmt.Sprintf(`_summon_inst, _summon_msg = summon(%q, %s, %s, %s, %s, %s, %q)`, args[0], args[1], args[2], args[3], args[4], args[5], args[6])
			}

			if err := c.scriptManager.L.DoString(luaCode); err != nil {
				return "Summon error: " + err.Error()
			}
			msgVal := c.scriptManager.L.GetGlobal("_summon_msg")
			if msgVal != nil && msgVal.Type() == lua.LTString {
				return msgVal.String()
			}
			return "Entity summoned. Target set to new instance."
		},
		"spawn": func(args []string) string {
			return c.builtins["summon"](args)
		},
		"delete": func(args []string) string {
			if c.scriptManager != nil {
				err := c.scriptManager.L.DoString(`
					if target and type(target.delete) == "function" then
						_del_ok, _del_msg = target:delete()
					else
						_del_ok = false
						_del_msg = "No target selected to delete"
					end
				`)
				if err != nil {
					return "Delete error: " + err.Error()
				}
				msg := c.scriptManager.L.GetGlobal("_del_msg").String()
				return msg
			}
			return "Script manager not available."
		},
		"destroy": func(args []string) string {
			return c.builtins["delete"](args)
		},
		"clear": func(args []string) string {
			c.Output = make([]ConsoleLine, 0)
			c.ScrollOffset = 0
			return "Console output cleared."
		},
		"reload": func(args []string) string {
			if c.scriptManager != nil {
				if err := c.scriptManager.ReloadScript(); err != nil {
					return "Reload error: " + err.Error()
				}
				return "Lua script reloaded successfully."
			}
			return "Script manager not available."
		},
		"history": func(args []string) string {
			if len(c.History) == 0 {
				return "No command history."
			}
			c.AddLine("--- Command History ---", LineSystem)
			for i, cmd := range c.History {
				c.AddLine(fmt.Sprintf("%3d: %s", i+1, cmd), LineInfo)
			}
			return ""
		},
		"reset": func(args []string) string {
			if c.scriptManager != nil {
				_ = c.scriptManager.L.DoString(`
					cube.set_position(0, 0, 0)
					cube.set_size(1.0)
					cube.set_rotation(20, 45, 0)
					cube.set_speed(30, 45, 20)
					cube.set_mode("faces")
					cube.set_wireframe(true)
					camera.reset()
				`)
				return "Reset cube and camera to defaults."
			}
			return "Reset completed."
		},
		"load": func(args []string) string {
			if len(args) == 0 {
				return "Usage: load <path/to/map.tmx or map_name>"
			}
			path := args[0]
			if c.scriptManager != nil {
				luaCmd := fmt.Sprintf(`load(%q)`, path)
				if err := c.scriptManager.L.DoString(luaCmd); err != nil {
					return "Error loading map: " + err.Error()
				}
				return fmt.Sprintf("Imported TMX map: %s", path)
			}
			return "Script manager not available."
		},
		"step_height": func(args []string) string {
			if c.scriptManager == nil || c.scriptManager.L == nil {
				return "Script manager not available."
			}
			if len(args) == 0 {
				err := c.scriptManager.L.DoString("val = player.get_step_height()")
				if err != nil {
					return "Error getting step height: " + err.Error()
				}
				val := c.scriptManager.L.GetGlobal("val").String()
				return fmt.Sprintf("Current ledge/staircase walk-up threshold: %s units", val)
			}
			valStr := args[0]
			err := c.scriptManager.L.DoString(fmt.Sprintf("player.set_step_height(%s)", valStr))
			if err != nil {
				return "Error setting step height: " + err.Error()
			}
			return fmt.Sprintf("Ledge/staircase walk-up threshold updated to %s units", valStr)
		},
		"map": func(args []string) string {
			if len(args) == 0 {
				return "Usage: map load <path.tmx> | map reload | map info"
			}
			sub := strings.ToLower(args[0])
			switch sub {
			case "load":
				if len(args) < 2 {
					return "Usage: map load <path/to/map.tmx or map_name>"
				}
				path := args[1]
				if c.scriptManager != nil {
					luaCmd := fmt.Sprintf(`load(%q)`, path)
					if err := c.scriptManager.L.DoString(luaCmd); err != nil {
						return "Error loading map: " + err.Error()
					}
					return fmt.Sprintf("Imported TMX map: %s", path)
				}
				return "Script manager not available."
			case "reload":
				if c.scriptManager != nil {
					if err := c.scriptManager.L.DoString(`map.reload()`); err != nil {
						return "Error reloading map: " + err.Error()
					}
					return "TMX map reloaded."
				}
				return "Script manager not available."
			case "info":
				if c.scriptManager != nil && c.scriptManager.CurrentMap != nil && c.scriptManager.CurrentMap.SourceMap != nil {
					sm := c.scriptManager.CurrentMap.SourceMap
					m3d := c.scriptManager.CurrentMap
					return fmt.Sprintf("Map: %s | Size: %dx%d tiles | 3D Tiles: %d | Spawns: %d",
						sm.FilePath, sm.Width, sm.Height, len(m3d.Tiles), len(m3d.Spawns))
				}
				return "No map currently active."
			default:
				return "Unknown map command: " + sub
			}
		},
	}
}

// Toggle flips the open/close state of the console.
func (c *Console) Toggle() {
	c.IsOpen = !c.IsOpen
	if c.IsOpen {
		c.BlinkTimer = 0.0
		c.ShowCursor = true
	}
}

// AddLine adds a timestamped message line to the console buffer.
func (c *Console) AddLine(text string, lType LineType) {
	c.Output = append(c.Output, ConsoleLine{
		Text:      text,
		Type:      lType,
		Timestamp: time.Now().Format("15:04:05"),
	})
	if len(c.Output) > c.MaxLines {
		c.Output = c.Output[len(c.Output)-c.MaxLines:]
	}
	// Reset scroll to bottom when new output arrives
	c.ScrollOffset = 0
}

// Update handles smooth open/close animation and keyboard/mouse input.
func (c *Console) Update(dt float32) {
	// Smooth slide animation
	target := float32(0.0)
	if c.IsOpen {
		target = 1.0
	}
	if c.AnimationProgress < target {
		c.AnimationProgress += c.SlideSpeed * dt
		if c.AnimationProgress > target {
			c.AnimationProgress = target
		}
	} else if c.AnimationProgress > target {
		c.AnimationProgress -= c.SlideSpeed * dt
		if c.AnimationProgress < target {
			c.AnimationProgress = target
		}
	}

	if !c.IsOpen {
		return
	}

	// Cursor blink
	c.BlinkTimer += dt
	if c.BlinkTimer >= 0.5 {
		c.BlinkTimer = 0.0
		c.ShowCursor = !c.ShowCursor
	}

	// Handle close via ESC
	if rl.IsKeyPressed(rl.KeyEscape) {
		c.Toggle()
		return
	}

	// Navigation & Editing keys
	if rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter) {
		c.submitInput()
		return
	}

	if rl.IsKeyPressed(rl.KeyBackspace) || (rl.IsKeyDown(rl.KeyBackspace) && c.BlinkTimer > 0.45) {
		c.handleBackspace()
	}

	if rl.IsKeyPressed(rl.KeyDelete) {
		c.handleDelete()
	}

	if rl.IsKeyPressed(rl.KeyLeft) {
		if c.CursorPos > 0 {
			c.CursorPos--
			c.ShowCursor = true
			c.BlinkTimer = 0
		}
	}

	if rl.IsKeyPressed(rl.KeyRight) {
		if c.CursorPos < len(c.InputBuffer) {
			c.CursorPos++
			c.ShowCursor = true
			c.BlinkTimer = 0
		}
	}

	if rl.IsKeyPressed(rl.KeyHome) {
		c.CursorPos = 0
	}

	if rl.IsKeyPressed(rl.KeyEnd) {
		c.CursorPos = len(c.InputBuffer)
	}

	// History Navigation (Up/Down)
	if rl.IsKeyPressed(rl.KeyUp) {
		c.historyPrevious()
	}
	if rl.IsKeyPressed(rl.KeyDown) {
		c.historyNext()
	}

	// Tab Auto-Completion
	if rl.IsKeyPressed(rl.KeyTab) {
		c.handleTabComplete()
	}

	// Clipboard Paste (Ctrl+V)
	ctrlDown := rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)
	if ctrlDown && rl.IsKeyPressed(rl.KeyV) {
		clip := rl.GetClipboardText()
		for _, r := range clip {
			if r >= 32 && r != '`' && r != '~' {
				c.insertRune(r)
			}
		}
	}

	// Mouse Wheel Scrolling in Console
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 {
		c.ScrollOffset += int(wheel * 3)
		if c.ScrollOffset < 0 {
			c.ScrollOffset = 0
		}
		maxScroll := len(c.Output) - 5
		if maxScroll < 0 {
			maxScroll = 0
		}
		if c.ScrollOffset > maxScroll {
			c.ScrollOffset = maxScroll
		}
	}

	// Capture text characters
	for {
		ch := rl.GetCharPressed()
		if ch <= 0 {
			break
		}
		// Ignore tilde / grave accent if pressed to toggle
		if ch == '`' || ch == '~' {
			continue
		}
		if ch >= 32 {
			c.insertRune(ch)
			c.ShowCursor = true
			c.BlinkTimer = 0
		}
	}
}

func (c *Console) insertRune(r rune) {
	if c.CursorPos == len(c.InputBuffer) {
		c.InputBuffer = append(c.InputBuffer, r)
		c.CursorPos++
	} else {
		c.InputBuffer = append(c.InputBuffer[:c.CursorPos], append([]rune{r}, c.InputBuffer[c.CursorPos:]...)...)
		c.CursorPos++
	}
}

func (c *Console) handleBackspace() {
	if c.CursorPos > 0 && len(c.InputBuffer) > 0 {
		c.InputBuffer = append(c.InputBuffer[:c.CursorPos-1], c.InputBuffer[c.CursorPos:]...)
		c.CursorPos--
		c.ShowCursor = true
		c.BlinkTimer = 0
	}
}

func (c *Console) handleDelete() {
	if c.CursorPos < len(c.InputBuffer) {
		c.InputBuffer = append(c.InputBuffer[:c.CursorPos], c.InputBuffer[c.CursorPos+1:]...)
		c.ShowCursor = true
		c.BlinkTimer = 0
	}
}

func (c *Console) historyPrevious() {
	if len(c.History) == 0 {
		return
	}
	if c.HistoryIndex == -1 {
		c.HistoryIndex = len(c.History) - 1
	} else if c.HistoryIndex > 0 {
		c.HistoryIndex--
	}
	c.setInputString(c.History[c.HistoryIndex])
}

func (c *Console) historyNext() {
	if len(c.History) == 0 || c.HistoryIndex == -1 {
		return
	}
	if c.HistoryIndex < len(c.History)-1 {
		c.HistoryIndex++
		c.setInputString(c.History[c.HistoryIndex])
	} else {
		c.HistoryIndex = -1
		c.setInputString("")
	}
}

func (c *Console) setInputString(str string) {
	c.InputBuffer = []rune(str)
	c.CursorPos = len(c.InputBuffer)
}

func (c *Console) handleTabComplete() {
	current := string(c.InputBuffer[:c.CursorPos])
	if current == "" {
		return
	}

	for _, cand := range c.completions {
		if strings.HasPrefix(cand, current) && len(cand) > len(current) {
			// Append remainder
			remainder := cand[len(current):]
			for _, r := range remainder {
				c.insertRune(r)
			}
			return
		}
	}
}

func (c *Console) submitInput() {
	raw := strings.TrimSpace(string(c.InputBuffer))
	c.InputBuffer = make([]rune, 0)
	c.CursorPos = 0
	c.HistoryIndex = -1

	if raw == "" {
		return
	}

	// Echo command to console
	c.AddLine("> "+raw, LineInput)

	// Add to history if unique from previous
	if len(c.History) == 0 || c.History[len(c.History)-1] != raw {
		c.History = append(c.History, raw)
		if len(c.History) > 100 {
			c.History = c.History[len(c.History)-100:]
		}
	}

	c.Execute(raw)
}

// Execute evaluates a command string (either built-in or Lua code).
func (c *Console) Execute(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmdName := strings.ToLower(parts[0])
	if fn, exists := c.builtins[cmdName]; exists {
		res := fn(parts[1:])
		if res != "" {
			c.AddLine(res, LineSystem)
		}
		return
	}

	if c.scriptManager == nil || c.scriptManager.L == nil {
		c.AddLine("Error: Lua scripting engine not available", LineError)
		return
	}

	L := c.scriptManager.L

	// 1. Try evaluating as an expression: "return <input>"
	exprCode := "return " + input
	err := L.DoString(exprCode)
	if err == nil {
		// Expression succeeded! Read return values from stack
		top := L.GetTop()
		if top > 0 {
			var rets []string
			for i := 1; i <= top; i++ {
				val := L.Get(i)
				rets = append(rets, formatLuaValue(val))
			}
			L.SetTop(0) // Clear stack
			c.AddLine("=> "+strings.Join(rets, ", "), LineOutput)
			return
		}
		return
	}

	// 2. If "return <input>" failed, evaluate as standard statement/block
	L.SetTop(0) // Clear any leftover syntax error state
	if err := L.DoString(input); err != nil {
		c.AddLine("Error: "+err.Error(), LineError)
		return
	}
}

func formatLuaValue(val lua.LValue) string {
	switch val.Type() {
	case lua.LTString:
		return fmt.Sprintf("%q", val.String())
	case lua.LTNumber:
		return val.String()
	case lua.LTBool:
		return val.String()
	case lua.LTNil:
		return "nil"
	case lua.LTTable:
		return fmt.Sprintf("table: %p", val.(*lua.LTable))
	case lua.LTFunction:
		return "function"
	default:
		return val.String()
	}
}

// Draw renders the in-game developer console overlay onto the screen.
func (c *Console) Draw() {
	if c.AnimationProgress <= 0.001 {
		return
	}

	screenWidth := int32(rl.GetScreenWidth())
	screenHeight := int32(rl.GetScreenHeight())

	// Dropdown height (up to 55% of window height)
	maxConsoleHeight := float32(screenHeight) * 0.55
	consoleHeight := int32(maxConsoleHeight * c.AnimationProgress)
	if consoleHeight < 50 {
		return
	}

	// Dark semi-transparent background
	rl.DrawRectangle(0, 0, screenWidth, consoleHeight, rl.NewColor(10, 14, 22, 248))

	// Bottom border accent line
	borderColor := rl.NewColor(50, 160, 230, 255)
	rl.DrawRectangle(0, consoleHeight-3, screenWidth, 3, borderColor)

	// Top Title Bar
	headerHeight := int32(28)
	rl.DrawRectangle(0, 0, screenWidth, headerHeight, rl.NewColor(16, 22, 34, 255))
	rl.DrawText("LUA DEVELOPER REPL CONSOLE", 16, 7, 13, rl.NewColor(80, 250, 120, 255))

	targetName := "None"
	if c.scriptManager != nil && c.scriptManager.GetCurrentTarget() != nil {
		targetName = c.scriptManager.GetCurrentTarget().Name
	}
	targetNotice := fmt.Sprintf("[Target: %s (Click 3D Object to Select)]", targetName)
	rl.DrawText(targetNotice, 250, 8, 11, rl.NewColor(100, 220, 255, 255))

	hintText := "[` or ESC: Close | Tab: Complete | Click Object: Set 'target']"
	hintWidth := rl.MeasureText(hintText, 11)
	rl.DrawText(hintText, screenWidth-hintWidth-16, 8, 11, rl.NewColor(160, 180, 205, 255))

	// Command Input Bar at bottom of console
	inputBarHeight := int32(32)
	inputBarY := consoleHeight - inputBarHeight - 4
	rl.DrawRectangle(8, inputBarY, screenWidth-16, inputBarHeight, rl.NewColor(18, 24, 38, 255))
	rl.DrawRectangleLines(8, inputBarY, screenWidth-16, inputBarHeight, rl.NewColor(40, 60, 90, 255))

	promptText := "lua> "
	promptX := int32(18)
	promptY := inputBarY + 8
	rl.DrawText(promptText, promptX, promptY, 14, rl.NewColor(80, 250, 120, 255))

	textX := promptX + rl.MeasureText(promptText, 14)

	// Input Text
	inputText := string(c.InputBuffer)
	rl.DrawText(inputText, textX, promptY, 14, rl.NewColor(240, 245, 255, 255))

	// Cursor
	if c.ShowCursor {
		cursorSubstr := string(c.InputBuffer[:c.CursorPos])
		cursorOffset := rl.MeasureText(cursorSubstr, 14)
		cursorX := textX + cursorOffset
		rl.DrawRectangle(cursorX, promptY, 2, 16, rl.NewColor(80, 250, 120, 255))
	}

	// Auto-completion preview hint (ghost text)
	if len(c.InputBuffer) > 0 && c.CursorPos == len(c.InputBuffer) {
		for _, cand := range c.completions {
			if strings.HasPrefix(cand, inputText) && len(cand) > len(inputText) {
				ghostText := cand[len(inputText):]
				ghostX := textX + rl.MeasureText(inputText, 14)
				rl.DrawText(ghostText, ghostX, promptY, 14, rl.NewColor(100, 120, 150, 180))
				break
			}
		}
	}

	// Output text lines area
	outputStartY := headerHeight + 6
	outputHeight := inputBarY - outputStartY - 6
	lineHeight := int32(18)
	visibleLines := int(outputHeight / lineHeight)

	if len(c.Output) > 0 {
		endIdx := len(c.Output) - c.ScrollOffset
		if endIdx > len(c.Output) {
			endIdx = len(c.Output)
		}
		startIdx := endIdx - visibleLines
		if startIdx < 0 {
			startIdx = 0
		}

		currY := outputStartY
		for i := startIdx; i < endIdx; i++ {
			line := c.Output[i]
			lineCol := rl.NewColor(220, 230, 240, 255)
			switch line.Type {
			case LineInput:
				lineCol = rl.NewColor(100, 220, 255, 255) // Cyan
			case LineOutput:
				lineCol = rl.NewColor(80, 250, 120, 255) // Green
			case LineInfo:
				lineCol = rl.NewColor(170, 190, 215, 255) // Gray-blue
			case LineError:
				lineCol = rl.NewColor(255, 80, 80, 255) // Red
			case LineSystem:
				lineCol = rl.NewColor(255, 210, 60, 255) // Gold
			}

			disp := line.Text
			rl.DrawText(disp, 18, currY, 12, lineCol)
			currY += lineHeight
		}

		// Scroll indicator if scrolled up
		if c.ScrollOffset > 0 {
			scrollNotice := fmt.Sprintf("[Scrolled up +%d lines | Wheel to scroll]", c.ScrollOffset)
			rl.DrawText(scrollNotice, screenWidth-260, outputStartY+4, 10, rl.NewColor(255, 200, 50, 255))
		}
	}
}
