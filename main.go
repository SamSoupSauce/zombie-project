package main

import (
	"zombie-project/internal/engine"
	"zombie-project/internal/paths"
	"zombie-project/internal/scene"
)

func main() {
	paths.EnsureProjectRoot()

	cfg := engine.DefaultConfig()
	eng := engine.NewEngine(cfg)

	demoScene := scene.NewDemoScene()
	eng.SetScene(demoScene)

	eng.Run()
}
