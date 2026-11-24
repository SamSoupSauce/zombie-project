package camera

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// OrbitCamera manages an orbital 3D camera that orbits around a target point.
type OrbitCamera struct {
	Camera      rl.Camera3D
	Target      rl.Vector3
	Distance    float32
	Yaw         float32 // Horizontal angle in radians
	Pitch       float32 // Vertical angle in radians
	MinDistance float32
	MaxDistance float32
	Sensitivity float32
	ZoomSpeed   float32

	defaultDistance float32
	defaultYaw      float32
	defaultPitch    float32
}

// NewOrbitCamera creates an orbital camera with specified target and distance.
func NewOrbitCamera(target rl.Vector3, distance float32) *OrbitCamera {
	cam := &OrbitCamera{
		Target:          target,
		Distance:        distance,
		Yaw:             0.785398, // ~45 degrees
		Pitch:           0.523599, // ~30 degrees
		MinDistance:     2.0,
		MaxDistance:     25.0,
		Sensitivity:     0.005,
		ZoomSpeed:       1.0,
		defaultDistance: distance,
		defaultYaw:      0.785398,
		defaultPitch:    0.523599,
	}

	cam.Camera = rl.Camera3D{
		Target:     target,
		Up:         rl.NewVector3(0.0, 1.0, 0.0),
		Fovy:       50.0,
		Projection: rl.CameraPerspective,
	}

	cam.updateCameraPosition()
	return cam
}

// Reset returns the camera to its initial orientation and zoom.
func (c *OrbitCamera) Reset() {
	c.Distance = c.defaultDistance
	c.Yaw = c.defaultYaw
	c.Pitch = c.defaultPitch
	c.updateCameraPosition()
}

// Update handles mouse and keyboard input for orbiting and zooming.
func (c *OrbitCamera) Update(dt float32) {
	// Mouse drag to orbit (Left or Right button)
	if rl.IsMouseButtonDown(rl.MouseButtonLeft) || rl.IsMouseButtonDown(rl.MouseButtonRight) {
		delta := rl.GetMouseDelta()
		c.Yaw -= delta.X * c.Sensitivity
		c.Pitch += delta.Y * c.Sensitivity
	}

	// Keyboard arrow keys for orbit control
	const keyTurnSpeed = 1.5
	if rl.IsKeyDown(rl.KeyLeft) {
		c.Yaw += keyTurnSpeed * dt
	}
	if rl.IsKeyDown(rl.KeyRight) {
		c.Yaw -= keyTurnSpeed * dt
	}
	if rl.IsKeyDown(rl.KeyUp) {
		c.Pitch += keyTurnSpeed * dt
	}
	if rl.IsKeyDown(rl.KeyDown) {
		c.Pitch -= keyTurnSpeed * dt
	}

	// Clamp pitch to prevent gimbal flip (-85° to +85°)
	const maxPitch = 85.0 * (math.Pi / 180.0)
	if c.Pitch > maxPitch {
		c.Pitch = maxPitch
	} else if c.Pitch < -maxPitch {
		c.Pitch = -maxPitch
	}

	// Mouse wheel zoom
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 {
		c.Distance -= wheel * c.ZoomSpeed
		if c.Distance < c.MinDistance {
			c.Distance = c.MinDistance
		}
		if c.Distance > c.MaxDistance {
			c.Distance = c.MaxDistance
		}
	}

	c.updateCameraPosition()
}

// updateCameraPosition calculates cartesian coordinates from spherical orbit angles.
func (c *OrbitCamera) updateCameraPosition() {
	cosP := float32(math.Cos(float64(c.Pitch)))
	sinP := float32(math.Sin(float64(c.Pitch)))
	sinY := float32(math.Sin(float64(c.Yaw)))
	cosY := float32(math.Cos(float64(c.Yaw)))

	c.Camera.Position = rl.NewVector3(
		c.Target.X+c.Distance*cosP*sinY,
		c.Target.Y+c.Distance*sinP,
		c.Target.Z+c.Distance*cosP*cosY,
	)
	c.Camera.Target = c.Target
}
