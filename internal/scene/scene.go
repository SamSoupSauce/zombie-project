package scene

import rl "github.com/gen2brain/raylib-go/raylib"

// Scene defines the lifecycle methods for a game scene.
type Scene interface {
	Init()
	Update(dt float32)
	Draw3D()
	Draw2D()
	GetCamera() rl.Camera3D
	Unload()
}
