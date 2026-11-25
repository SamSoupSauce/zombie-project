package engine

import (
	core "github.com/SamSoupSauce/project-engine/engine"
	"zombie-project/internal/scene"
)

// Config holds the window and display settings for the engine.
type Config = core.Config

// DefaultConfig returns modern, polished defaults for the 3D engine.
var DefaultConfig = core.DefaultConfig

// Engine coordinates the game loop, graphics context, and active scenes.
type Engine struct {
	coreEng *core.Engine
}

// NewEngine creates a new Engine instance with the given configuration.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		coreEng: core.NewEngine(cfg),
	}
}

// SetScene sets the active scene and initializes it.
func (e *Engine) SetScene(s scene.Scene) {
	e.coreEng.SetScene(s)
}

// Run starts the window and enters the main rendering and game loop.
func (e *Engine) Run() {
	e.coreEng.Run()
}
