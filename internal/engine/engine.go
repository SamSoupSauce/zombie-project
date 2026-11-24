package engine

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/scene"
)

// Config holds the window and display settings for the engine.
type Config struct {
	Width       int32
	Height      int32
	Title       string
	TargetFPS   int32
	ClearColor  rl.Color
	EnableMSAA  bool
	Resizable   bool
}

// DefaultConfig returns modern, polished defaults for the 3D engine.
func DefaultConfig() Config {
	return Config{
		Width:      1280,
		Height:     720,
		Title:      "Zombie 3D Engine - Rainbow Cube Demo",
		TargetFPS:  60,
		ClearColor: rl.NewColor(10, 14, 22, 255), // Midnight slate space background
		EnableMSAA: true,
		Resizable:  true,
	}
}

// Engine coordinates the game loop, graphics context, and active scenes.
type Engine struct {
	Config      Config
	ActiveScene scene.Scene
}

// NewEngine creates a new Engine instance with the given configuration.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		Config: cfg,
	}
}

// SetScene sets the active scene and initializes it.
func (e *Engine) SetScene(s scene.Scene) {
	if e.ActiveScene != nil {
		e.ActiveScene.Unload()
	}
	e.ActiveScene = s
	if e.ActiveScene != nil {
		e.ActiveScene.Init()
	}
}

// Run starts the window and enters the main rendering and game loop.
func (e *Engine) Run() {
	var flags uint32 = rl.FlagVsyncHint
	if e.Config.EnableMSAA {
		flags |= rl.FlagMsaa4xHint
	}
	if e.Config.Resizable {
		flags |= rl.FlagWindowResizable
	}

	rl.SetConfigFlags(flags)
	rl.InitWindow(e.Config.Width, e.Config.Height, e.Config.Title)
	defer rl.CloseWindow()

	rl.SetTargetFPS(e.Config.TargetFPS)

	// Initialize the active scene now that the Raylib window is created and ready
	if e.ActiveScene != nil {
		e.ActiveScene.Init()
	}

	for !rl.WindowShouldClose() {
		// Toggle fullscreen shortcut
		if rl.IsKeyPressed(rl.KeyF) {
			rl.ToggleFullscreen()
		}

		dt := rl.GetFrameTime()

		// Update active scene
		if e.ActiveScene != nil {
			e.ActiveScene.Update(dt)
		}

		// Begin Frame
		rl.BeginDrawing()
		rl.ClearBackground(e.Config.ClearColor)

		if e.ActiveScene != nil {
			// 3D Pass
			cam := e.ActiveScene.GetCamera()
			rl.BeginMode3D(cam)
			e.ActiveScene.Draw3D()
			rl.EndMode3D()

			// 2D Pass (HUD / UI)
			e.ActiveScene.Draw2D()
		}

		rl.EndDrawing()
	}

	if e.ActiveScene != nil {
		e.ActiveScene.Unload()
	}
}
