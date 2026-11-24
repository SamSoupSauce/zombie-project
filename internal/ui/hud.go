package ui

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/camera"
	"zombie-project/internal/cube"
	"zombie-project/internal/scripting"
)

// HUD renders modern on-screen telemetry, mode badges, Lua status, and controls help.
type HUD struct {
	ShowHelp bool
}

// NewHUD creates a new HUD instance.
func NewHUD() *HUD {
	return &HUD{
		ShowHelp: true,
	}
}

// ToggleHelp toggles the visibility of the keybindings help box.
func (h *HUD) ToggleHelp() {
	h.ShowHelp = !h.ShowHelp
}

// Draw renders all 2D HUD elements onto the current screen frame.
func (h *HUD) Draw(c *cube.RainbowCube, cam *camera.OrbitCamera, showGrid bool, sm *scripting.ScriptManager) {
	// Top-left title & telemetry badge
	drawCard(16, 16, 340, 158, rl.NewColor(15, 18, 25, 215), rl.NewColor(50, 70, 95, 255))

	// Title
	rl.DrawText("ZOMBIE ENGINE 3D", 28, 26, 18, rl.NewColor(80, 250, 120, 255))
	rl.DrawText("Tiled TMX 3D Map & Lua Console Demo", 28, 48, 12, rl.NewColor(180, 190, 205, 255))

	// FPS & frame time counter
	fps := rl.GetFPS()
	dtMs := rl.GetFrameTime() * 1000.0

	fpsColor := rl.NewColor(80, 250, 120, 255)
	if fps < 30 {
		fpsColor = rl.NewColor(255, 70, 70, 255)
	} else if fps < 55 {
		fpsColor = rl.NewColor(255, 210, 60, 255)
	}

	fpsStr := fmt.Sprintf("%d FPS (%.1f ms)", fps, dtMs)
	rl.DrawText(fpsStr, 28, 68, 13, fpsColor)

	// Mode badge line
	modeStr := "N/A (No target)"
	if c != nil {
		modeStr = c.Mode.String()
	}
	rl.DrawText(fmt.Sprintf("Mode: %s", modeStr), 28, 88, 12, rl.NewColor(220, 230, 245, 255))

	// Map badge line
	mapTitle := "No TMX Map Loaded"
	if sm != nil && sm.CurrentMap != nil && sm.CurrentMap.SourceMap != nil {
		mapTitle = fmt.Sprintf("Map: %s (%dx%d TMX)",
			sm.CurrentMap.SourceMap.GetProperty("title", "Level"),
			sm.CurrentMap.SourceMap.Width,
			sm.CurrentMap.SourceMap.Height,
		)
	}
	rl.DrawText(mapTitle, 28, 106, 11, rl.NewColor(255, 210, 70, 255))

	// Lua Script status badge
	luaBadgeColor := rl.NewColor(70, 180, 255, 255)
	luaText := "Lua: Inactive"
	if sm != nil && sm.ScriptPath != "" {
		luaText = fmt.Sprintf("Lua: %s (F5 to reload)", sm.ScriptPath)
	}
	rl.DrawText(luaText, 28, 124, 11, luaBadgeColor)

	if sm != nil && sm.StatusMsg != "" {
		rl.DrawText("✓ "+sm.StatusMsg, 28, 140, 11, rl.NewColor(80, 250, 140, 255))
	} else if sm != nil && sm.LastReloadErr != "" {
		rl.DrawText("✗ Error: "+sm.LastReloadErr, 28, 140, 10, rl.NewColor(255, 80, 80, 255))
	}

	// Top-right status metrics card
	screenWidth := int32(rl.GetScreenWidth())
	metricsCardWidth := int32(310)
	metricsX := screenWidth - metricsCardWidth - 16
	drawCard(metricsX, 16, metricsCardWidth, 190, rl.NewColor(15, 18, 25, 215), rl.NewColor(50, 70, 95, 255))

	if c == nil {
		rl.DrawText("TARGET: None", metricsX+14, 26, 14, rl.NewColor(150, 160, 175, 255))
		rl.DrawText("No entity currently selected.", metricsX+14, 50, 12, rl.NewColor(130, 140, 155, 255))
		rl.DrawText("Click an object or tile in console", metricsX+14, 72, 11, rl.NewColor(100, 200, 255, 255))
		rl.DrawText("to set active 'target'.", metricsX+14, 90, 11, rl.NewColor(100, 200, 255, 255))
		return
	}

	rl.DrawText(fmt.Sprintf("TARGET: %s", c.Name), metricsX+14, 26, 14, rl.NewColor(100, 220, 255, 255))

	rl.DrawText(fmt.Sprintf("Position: (%.2f, %.2f, %.2f)", c.Position.X, c.Position.Y, c.Position.Z),
		metricsX+14, 46, 12, rl.NewColor(200, 215, 230, 255))

	rl.DrawText(fmt.Sprintf("Size: %.2f (Scale: %.2fx)", c.Size, c.Scale),
		metricsX+14, 64, 12, rl.NewColor(200, 215, 230, 255))

	// Velocity & Physics Telemetry
	vel := c.GetVelocity()
	rl.DrawText(fmt.Sprintf("Velocity: (%.2f, %.2f, %.2f)", vel.X, vel.Y, vel.Z),
		metricsX+14, 82, 11, rl.NewColor(130, 215, 255, 255))

	// Mass & Pickup Status
	massStr := fmt.Sprintf("Mass: %.1f kg (Liftable: < 50kg)", c.Mass)
	massColor := rl.NewColor(80, 250, 140, 255)
	if c.Mass >= 50.0 {
		massStr = fmt.Sprintf("Mass: %.1f kg [TOO HEAVY >= 50kg]", c.Mass)
		massColor = rl.NewColor(255, 95, 80, 255)
	}
	if c.IsCarried {
		massStr += " [HELD]"
		massColor = rl.NewColor(255, 215, 70, 255)
	}
	rl.DrawText(massStr, metricsX+14, 100, 11, massColor)

	rl.DrawText(fmt.Sprintf("Bounce: %.2f | Frict: %.2f | Grounded: %v", c.GetRestitution(), c.GetFriction(), c.IsGrounded()),
		metricsX+14, 118, 11, rl.NewColor(200, 225, 140, 255))

	rl.DrawText(fmt.Sprintf("Rotation: Y:%.0f° X:%.0f° Z:%.0f°", c.Rotation.Y, c.Rotation.X, c.Rotation.Z),
		metricsX+14, 138, 11, rl.NewColor(180, 195, 215, 255))

	wireText := "OFF"
	if c.ShowWireframe {
		wireText = "ON"
	}
	boxText := "OFF"
	if c.ShowCollisionBox {
		boxText = "ON"
	}
	gridText := "OFF"
	if showGrid {
		gridText = "ON"
	}
	rl.DrawText(fmt.Sprintf("Wire: %s | Collider: %s | Grid: %s", wireText, boxText, gridText),
		metricsX+14, 156, 11, rl.NewColor(160, 175, 195, 255))

	// Bottom-right Lua Live Log Console
	if sm != nil && len(sm.Logs) > 0 {
		consoleWidth := int32(380)
		consoleHeight := int32(110)
		consoleX := screenWidth - consoleWidth - 16
		screenHeight := int32(rl.GetScreenHeight())
		consoleY := screenHeight - consoleHeight - 16

		drawCard(consoleX, consoleY, consoleWidth, consoleHeight, rl.NewColor(10, 12, 18, 220), rl.NewColor(45, 60, 80, 240))
		rl.DrawText("LUA SCRIPT LOGS", consoleX+14, consoleY+10, 11, rl.NewColor(100, 220, 255, 255))

		startIdx := 0
		if len(sm.Logs) > 4 {
			startIdx = len(sm.Logs) - 4
		}
		yLog := consoleY + 28
		for i := startIdx; i < len(sm.Logs); i++ {
			rl.DrawText(sm.Logs[i], consoleX+14, yLog, 10, rl.NewColor(180, 195, 210, 255))
			yLog += 18
		}
	}

	// Bottom-left controls panel (collapsible with [H])
	screenHeight := int32(rl.GetScreenHeight())
	if h.ShowHelp {
		panelHeight := int32(272)
		panelWidth := int32(370)
		panelY := screenHeight - panelHeight - 16

		drawCard(16, panelY, panelWidth, panelHeight, rl.NewColor(12, 15, 22, 225), rl.NewColor(45, 60, 85, 255))

		rl.DrawText("FIRST PERSON CONTROLS & SHORTCUTS", 28, panelY+12, 13, rl.NewColor(255, 205, 70, 255))

		items := []struct {
			Key  string
			Desc string
		}{
			{"[` / ~]", "Open Lua Console (Unlocks Mouse)"},
			{"WASD", "Walk / Move Player"},
			{"Left Shift", "Sprint (8.2 m/s)"},
			{"SPACE", "Jump / Leap"},
			{"Mouse", "Look Around (First Person)"},
			{"[E] / [RMB]", "Pick Up (< 50kg) / Drop Prop"},
			{"[LMB] / [G]", "Throw Held Prop with Gravity"},
			{"[B]", "Toggle Collision Boxes Wireframe"},
			{"[V]", "Toggle First Person / Orbit Mode"},
			{"[M]", "Toggle 3D TMX Map Level"},
			{"[F5]", "Hot-Reload Lua Script"},
			{"[R] / [1, 2, 3]", "Switch Rainbow Render Mode"},
			{"[C]", "Reset Player to Spawn Position"},
			{"[H]", "Hide / Show Controls Help"},
		}

		yOffset := panelY + 32
		for _, item := range items {
			rl.DrawText(item.Key, 28, yOffset, 11, rl.NewColor(100, 210, 255, 255))
			rl.DrawText(item.Desc, 160, yOffset, 11, rl.NewColor(210, 220, 235, 255))
			yOffset += 16
		}
	} else {
		// Minimized hint
		hintY := screenHeight - 34
		drawCard(16, hintY, 150, 24, rl.NewColor(15, 18, 25, 180), rl.NewColor(50, 70, 95, 200))
		rl.DrawText("Press [H] for Help", 24, hintY+6, 11, rl.NewColor(180, 200, 220, 255))
	}
}

// drawCard draws a rounded card panel with an accent border.
func drawCard(x, y, width, height int32, bgCol, borderCol rl.Color) {
	rec := rl.NewRectangle(float32(x), float32(y), float32(width), float32(height))
	rl.DrawRectangleRounded(rec, 0.10, 6, bgCol)
	rl.DrawRectangleRoundedLines(rec, 0.10, 6, borderCol)
}
