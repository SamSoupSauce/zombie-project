package console

import (
	"strings"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/camera"
	"zombie-project/internal/cube"
	"zombie-project/internal/scripting"
)

func setupTestConsole() (*Console, *cube.RainbowCube, *camera.OrbitCamera) {
	c := cube.NewRainbowCube(1.0)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := scripting.NewScriptManager(c, cam)
	con := NewConsole(sm)
	return con, c, cam
}

func TestConsoleToggle(t *testing.T) {
	con, _, _ := setupTestConsole()

	if con.IsOpen {
		t.Errorf("expected console initially closed")
	}

	con.Toggle()
	if !con.IsOpen {
		t.Errorf("expected console open after toggle")
	}

	con.Toggle()
	if con.IsOpen {
		t.Errorf("expected console closed after second toggle")
	}
}

func TestConsoleInputEditing(t *testing.T) {
	con, _, _ := setupTestConsole()

	// Insert runes
	for _, r := range "cube.set_size(3.0)" {
		con.insertRune(r)
	}

	if string(con.InputBuffer) != "cube.set_size(3.0)" {
		t.Errorf("expected input buffer 'cube.set_size(3.0)', got '%s'", string(con.InputBuffer))
	}
	if con.CursorPos != len("cube.set_size(3.0)") {
		t.Errorf("expected cursor at end, got %d", con.CursorPos)
	}

	// Backspace
	con.handleBackspace()
	if string(con.InputBuffer) != "cube.set_size(3.0" {
		t.Errorf("expected input buffer after backspace 'cube.set_size(3.0', got '%s'", string(con.InputBuffer))
	}

	// Insert back the parenthesis
	con.insertRune(')')

	// Submit input
	con.submitInput()
	if len(con.InputBuffer) != 0 {
		t.Errorf("expected empty buffer after submit, got '%s'", string(con.InputBuffer))
	}
	if len(con.History) != 1 || con.History[0] != "cube.set_size(3.0)" {
		t.Errorf("expected command recorded in history")
	}
}

func TestConsoleExecutionAndLuaREPL(t *testing.T) {
	con, cubeObj, _ := setupTestConsole()

	// 1. Test arithmetic expression evaluation
	con.Execute("20 + 22")
	lastLine := con.Output[len(con.Output)-1]
	if lastLine.Type != LineOutput || !strings.Contains(lastLine.Text, "42") {
		t.Errorf("expected output containing '42', got '%s'", lastLine.Text)
	}

	// 2. Test Lua cube manipulation
	con.Execute("cube.set_size(2.75)")
	if cubeObj.Size != 2.75 {
		t.Errorf("expected cube size updated to 2.75, got %f", cubeObj.Size)
	}

	// 3. Test Lua expression return value
	con.Execute("cube.get_size()")
	lastLine = con.Output[len(con.Output)-1]
	if !strings.Contains(lastLine.Text, "2.75") {
		t.Errorf("expected output to contain '2.75', got '%s'", lastLine.Text)
	}

	// 4. Test error handling
	con.Execute("this is bad syntax !!!")
	lastLine = con.Output[len(con.Output)-1]
	if lastLine.Type != LineError {
		t.Errorf("expected LineError for bad syntax, got %v with text '%s'", lastLine.Type, lastLine.Text)
	}
}

func TestConsoleBuiltins(t *testing.T) {
	con, cubeObj, _ := setupTestConsole()

	// help
	con.Execute("help")
	hasHelp := false
	for _, l := range con.Output {
		if strings.Contains(l.Text, "Built-in Console Commands") {
			hasHelp = true
			break
		}
	}
	if !hasHelp {
		t.Errorf("expected help output")
	}

	// reset
	cubeObj.Size = 4.0
	con.Execute("reset")
	if cubeObj.Size != 1.0 {
		t.Errorf("expected size reset to 1.0, got %f", cubeObj.Size)
	}

	// clear
	con.Execute("clear")
	if len(con.Output) != 1 || con.Output[0].Text != "Console output cleared." {
		t.Errorf("expected cleared output, got %d lines", len(con.Output))
	}

	// load kitchen_sink (direct command)
	con.Execute("load kitchen_sink")
	lastLine := con.Output[len(con.Output)-1]
	if !strings.Contains(lastLine.Text, "kitchen_sink") && !strings.Contains(lastLine.Text, "Kitchen Sink") {
		t.Errorf("expected load confirmation, got '%s'", lastLine.Text)
	}

	// map load kitchen_sink (backward-compatible subcommand)
	con.Execute("map load kitchen_sink")
	lastLine = con.Output[len(con.Output)-1]
	if !strings.Contains(lastLine.Text, "kitchen_sink") && !strings.Contains(lastLine.Text, "Kitchen Sink") {
		t.Errorf("expected map load confirmation, got '%s'", lastLine.Text)
	}

	// step_height command
	con.Execute("step_height")
	lastLine = con.Output[len(con.Output)-1]
	if !strings.Contains(lastLine.Text, "threshold") {
		t.Errorf("expected step_height query output, got '%s'", lastLine.Text)
	}

	con.Execute("step_height 0.85")
	lastLine = con.Output[len(con.Output)-1]
	if !strings.Contains(lastLine.Text, "0.85") {
		t.Errorf("expected step_height set output, got '%s'", lastLine.Text)
	}
}

func TestConsoleHistoryNavigation(t *testing.T) {
	con, _, _ := setupTestConsole()

	con.History = []string{"cmd1", "cmd2", "cmd3"}

	con.historyPrevious()
	if string(con.InputBuffer) != "cmd3" {
		t.Errorf("expected 'cmd3', got '%s'", string(con.InputBuffer))
	}

	con.historyPrevious()
	if string(con.InputBuffer) != "cmd2" {
		t.Errorf("expected 'cmd2', got '%s'", string(con.InputBuffer))
	}

	con.historyNext()
	if string(con.InputBuffer) != "cmd3" {
		t.Errorf("expected 'cmd3', got '%s'", string(con.InputBuffer))
	}

	con.historyNext()
	if string(con.InputBuffer) != "" {
		t.Errorf("expected empty buffer after next past end, got '%s'", string(con.InputBuffer))
	}
}

func TestConsoleTabCompletion(t *testing.T) {
	con, _, _ := setupTestConsole()

	con.setInputString("cube.get_m")
	con.handleTabComplete()

	if string(con.InputBuffer) != "cube.get_mode()" {
		t.Errorf("expected auto-completion to 'cube.get_mode()', got '%s'", string(con.InputBuffer))
	}

	con.setInputString("load kit")
	con.handleTabComplete()
	if string(con.InputBuffer) != "load kitchen_sink" {
		t.Errorf("expected auto-completion to 'load kitchen_sink', got '%s'", string(con.InputBuffer))
	}
}

func TestConsoleDeleteCommand(t *testing.T) {
	c := cube.NewRainbowCubeWithConfig("del_c", "DeletableCube", 1.0, rl.NewVector3(0, 0, 0), cube.ModeFaceColors)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := scripting.NewScriptManager(c, cam)
	sm.SetGlobalTarget(c)
	con := NewConsole(sm)

	// Execute 'delete' builtin command
	con.Execute("delete")
	if !c.IsDeleted {
		t.Errorf("expected cube to be deleted after 'delete' console command")
	}

	// Tab completion for target:delete()
	con.setInputString("target:del")
	con.handleTabComplete()
	if string(con.InputBuffer) != "target:delete()" {
		t.Errorf("expected auto-completion to 'target:delete()', got '%s'", string(con.InputBuffer))
	}

	// Tab completion for target.delete()
	con.setInputString("target.del")
	con.handleTabComplete()
	if string(con.InputBuffer) != "target.delete()" {
		t.Errorf("expected auto-completion to 'target.delete()', got '%s'", string(con.InputBuffer))
	}

	// Tab completion for builtin delete
	con.setInputString("del")
	con.handleTabComplete()
	if string(con.InputBuffer) != "delete" {
		t.Errorf("expected auto-completion to 'delete', got '%s'", string(con.InputBuffer))
	}
}

func TestConsoleSummonCommand(t *testing.T) {
	c := cube.NewRainbowCubeWithConfig("base_c", "BaseCube", 1.0, rl.NewVector3(0, 0, 0), cube.ModeFaceColors)
	cam := camera.NewOrbitCamera(rl.NewVector3(0, 0, 0), 5.0)
	sm := scripting.NewScriptManager(c, cam)
	sm.SetGlobalTarget(c)
	con := NewConsole(sm)

	// 1. Execute 'summon' command without args
	con.Execute("summon")
	lastLine := con.Output[len(con.Output)-1]
	if !strings.Contains(lastLine.Text, "Summoned entity") {
		t.Errorf("expected output to contain 'Summoned entity', got '%s'", lastLine.Text)
	}
	if sm.GetCurrentTarget().Name == "BaseCube" {
		t.Errorf("expected target to change from BaseCube after summon")
	}

	// 2. Execute 'summon' command with name and arguments
	con.Execute("summon MagicBox 1.0 2.5 -3.0 1.25 45.0")
	lastLine = con.Output[len(con.Output)-1]
	if !strings.Contains(lastLine.Text, "MagicBox") {
		t.Errorf("expected output to contain 'MagicBox', got '%s'", lastLine.Text)
	}
	if sm.GetCurrentTarget().Name != "MagicBox" {
		t.Errorf("expected target to be 'MagicBox', got '%s'", sm.GetCurrentTarget().Name)
	}
	if sm.GetCurrentTarget().Mass != 45.0 {
		t.Errorf("expected target mass 45.0, got %f", sm.GetCurrentTarget().Mass)
	}

	// 3. Execute 'spawn' alias command
	con.Execute("spawn FastCube")
	if sm.GetCurrentTarget().Name != "FastCube" {
		t.Errorf("expected target to be 'FastCube', got '%s'", sm.GetCurrentTarget().Name)
	}

	// 4. Tab completions
	con.setInputString("sum")
	con.handleTabComplete()
	if string(con.InputBuffer) != "summon" {
		t.Errorf("expected auto-completion to 'summon', got '%s'", string(con.InputBuffer))
	}

	con.setInputString("spaw")
	con.handleTabComplete()
	if string(con.InputBuffer) != "spawn" {
		t.Errorf("expected auto-completion to 'spawn', got '%s'", string(con.InputBuffer))
	}

	con.setInputString("engine.sum")
	con.handleTabComplete()
	if string(con.InputBuffer) != "engine.summon()" {
		t.Errorf("expected auto-completion to 'engine.summon()', got '%s'", string(con.InputBuffer))
	}
}
