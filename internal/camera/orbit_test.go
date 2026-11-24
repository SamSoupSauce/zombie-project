package camera

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestOrbitCameraCreation(t *testing.T) {
	target := rl.NewVector3(0, 0, 0)
	cam := NewOrbitCamera(target, 5.0)

	if cam.Distance != 5.0 {
		t.Errorf("expected distance 5.0, got %f", cam.Distance)
	}
	if cam.Target != target {
		t.Errorf("expected target %v, got %v", target, cam.Target)
	}

	// Calculate distance from position to target
	dx := cam.Camera.Position.X - target.X
	dy := cam.Camera.Position.Y - target.Y
	dz := cam.Camera.Position.Z - target.Z
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))

	if math.Abs(float64(dist-5.0)) > 0.01 {
		t.Errorf("expected calculated camera distance ~5.0, got %f", dist)
	}
}

func TestOrbitCameraReset(t *testing.T) {
	target := rl.NewVector3(0, 0, 0)
	cam := NewOrbitCamera(target, 5.0)

	cam.Distance = 10.0
	cam.Yaw = 2.0
	cam.Pitch = 1.0
	cam.Reset()

	if cam.Distance != 5.0 {
		t.Errorf("expected distance reset to 5.0, got %f", cam.Distance)
	}
	if cam.Yaw != cam.defaultYaw {
		t.Errorf("expected yaw reset to %f, got %f", cam.defaultYaw, cam.Yaw)
	}
}
