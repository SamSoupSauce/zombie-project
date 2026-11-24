package cube

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/physics"
)

// RainbowMode defines the visual styling of the rainbow cube.
type RainbowMode int

const (
	ModeFaceColors RainbowMode = iota
	ModeVertexGradient
	ModeChromaCycle
	rainbowModeCount
)

func (m RainbowMode) String() string {
	switch m {
	case ModeFaceColors:
		return "Face Rainbow (Classic Rubik's Palette)"
	case ModeVertexGradient:
		return "Smooth Vertex Gradient (RGB Spectrum)"
	case ModeChromaCycle:
		return "Dynamic Chroma Cycle (Shifting Hue)"
	default:
		return "Unknown"
	}
}

// RainbowCube represents a rotating 3D rainbow cube.
type RainbowCube struct {
	ID                string        // Unique identifier
	Name              string        // Human-readable object name
	Size              float32       // Half-size of the cube
	Scale             float32       // Scale factor (default 1.0)
	Position          rl.Vector3    // World position
	Rotation          rl.Vector3    // Current rotation in degrees (Pitch, Yaw, Roll)
	RotationSpeed     rl.Vector3    // Rotation speed (deg/sec)
	AutoRotate        bool
	Mode              RainbowMode
	ShowWireframe     bool
	WireframeColor    rl.Color
	Time              float32
	Mass              float32       // Mass in kilograms (kg)
	IsCarried         bool          // Whether currently held by player
	IsDeleted         bool          // Whether entity has been deleted
	Body              *physics.Body // 3D rigid physics body
	ShowCollisionBox  bool          // Whether to draw collision bounding box
	CollisionBoxColor rl.Color
}

// EffectiveSize returns the current rendered half-size accounting for Scale.
func (c *RainbowCube) EffectiveSize() float32 {
	scale := c.Scale
	if scale <= 0 {
		scale = 1.0
	}
	return c.Size * scale
}

// NewRainbowCube initializes a rainbow cube centered at origin.
func NewRainbowCube(halfSize float32) *RainbowCube {
	return NewRainbowCubeWithConfig("cube_1", "RainbowCube", halfSize, rl.NewVector3(0.0, 0.0, 0.0), ModeFaceColors)
}

// NewRainbowCubeWithConfig creates a customized rainbow cube entity with an attached physics body.
func NewRainbowCubeWithConfig(id, name string, halfSize float32, pos rl.Vector3, mode RainbowMode) *RainbowCube {
	body := physics.NewBody(id, name, pos, rl.NewVector3(halfSize, halfSize, halfSize), 35.0)

	return &RainbowCube{
		ID:                id,
		Name:              name,
		Size:              halfSize,
		Scale:             1.0,
		Position:          pos,
		Rotation:          rl.NewVector3(20.0, 45.0, 0.0),
		RotationSpeed:     rl.NewVector3(30.0, 45.0, 20.0),
		AutoRotate:        true,
		Mode:              mode,
		ShowWireframe:     true,
		WireframeColor:    rl.NewColor(20, 20, 30, 220),
		Mass:              35.0,
		IsCarried:         false,
		IsDeleted:         false,
		Body:              body,
		ShowCollisionBox:  false,
		CollisionBoxColor: rl.NewColor(0, 220, 255, 230),
	}
}

// Implement prop.Prop interface getters and setters
func (c *RainbowCube) GetID() string   { return c.ID }
func (c *RainbowCube) GetName() string { return c.Name }
func (c *RainbowCube) GetPosition() rl.Vector3 {
	if c.Body != nil {
		return c.Body.Position
	}
	return c.Position
}
func (c *RainbowCube) SetPosition(pos rl.Vector3) {
	c.Position = pos
	if c.Body != nil {
		c.Body.Position = pos
	}
}
func (c *RainbowCube) GetRotation() rl.Vector3    { return c.Rotation }
func (c *RainbowCube) SetRotation(rot rl.Vector3) { c.Rotation = rot }
func (c *RainbowCube) GetSize() float32           { return c.EffectiveSize() }
func (c *RainbowCube) GetMass() float32 {
	if c.Body != nil {
		return c.Body.Mass
	}
	return c.Mass
}
func (c *RainbowCube) SetMass(mass float32) {
	c.Mass = mass
	if c.Body != nil {
		c.Body.SetMass(mass)
	}
}
func (c *RainbowCube) IsHeld() bool { return c.IsCarried }
func (c *RainbowCube) SetHeld(held bool) {
	c.IsCarried = held
	if c.Body != nil {
		c.Body.IsHeld = held
		if held {
			c.Body.Velocity = rl.NewVector3(0, 0, 0)
			c.Body.AngularVelocity = rl.NewVector3(0, 0, 0)
			c.Body.IsGrounded = false
		}
	}
}

// GetBoundingBox returns the 3D collision bounding box in world space.
func (c *RainbowCube) GetBoundingBox() rl.BoundingBox {
	if c.Body != nil {
		return c.Body.GetBoundingBox()
	}
	s := c.EffectiveSize()
	return rl.NewBoundingBox(
		rl.NewVector3(c.Position.X-s, c.Position.Y-s, c.Position.Z-s),
		rl.NewVector3(c.Position.X+s, c.Position.Y+s, c.Position.Z+s),
	)
}

// Physics body delegation methods
func (c *RainbowCube) GetVelocity() rl.Vector3 {
	if c.Body != nil {
		return c.Body.Velocity
	}
	return rl.NewVector3(0, 0, 0)
}
func (c *RainbowCube) SetVelocity(vel rl.Vector3) {
	if c.Body != nil {
		c.Body.Velocity = vel
	}
}
func (c *RainbowCube) ApplyImpulse(impulse rl.Vector3) {
	if c.Body != nil {
		c.Body.ApplyImpulse(impulse)
	}
}
func (c *RainbowCube) IsGrounded() bool {
	if c.Body != nil {
		return c.Body.IsGrounded
	}
	return true
}
func (c *RainbowCube) GetRestitution() float32 {
	if c.Body != nil {
		return c.Body.Restitution
	}
	return 0.35
}
func (c *RainbowCube) SetRestitution(e float32) {
	if c.Body != nil {
		c.Body.Restitution = e
	}
}
func (c *RainbowCube) GetFriction() float32 {
	if c.Body != nil {
		return c.Body.Friction
	}
	return 0.75
}
func (c *RainbowCube) SetFriction(f float32) {
	if c.Body != nil {
		c.Body.Friction = f
	}
}
func (c *RainbowCube) IsStatic() bool {
	if c.Body != nil {
		return c.Body.Type == physics.BodyStatic
	}
	return false
}
func (c *RainbowCube) SetStatic(isStatic bool) {
	if c.Body != nil {
		c.Body.SetStatic(isStatic)
	}
}

// RayIntersects tests whether a world-space ray intersects this oriented cube.
func (c *RainbowCube) RayIntersects(ray rl.Ray) (bool, float32) {
	s := c.EffectiveSize()
	if s <= 0 {
		return false, 0
	}

	degToRad := float32(math.Pi / 180.0)
	matYaw := rl.MatrixRotateY(c.Rotation.Y * degToRad)
	matPitch := rl.MatrixRotateX(c.Rotation.X * degToRad)
	matRoll := rl.MatrixRotateZ(c.Rotation.Z * degToRad)
	matRot := rl.MatrixMultiply(rl.MatrixMultiply(matYaw, matPitch), matRoll)

	matInv := rl.MatrixInvert(matRot)

	relPos := rl.Vector3Subtract(ray.Position, c.Position)
	localRayPos := rl.Vector3Transform(relPos, matInv)
	localRayDir := rl.Vector3Transform(ray.Direction, matInv)

	localRay := rl.Ray{
		Position:  localRayPos,
		Direction: rl.Vector3Normalize(localRayDir),
	}

	box := rl.NewBoundingBox(
		rl.NewVector3(-s, -s, -s),
		rl.NewVector3(s, s, s),
	)

	col := rl.GetRayCollisionBox(localRay, box)
	return col.Hit, col.Distance
}

// Update advances cube rotation, physics synchronization, and internal animation timers.
func (c *RainbowCube) Update(dt float32) {
	c.Time += dt

	effSize := c.EffectiveSize()
	if c.Body != nil {
		c.Body.HalfExtents = rl.NewVector3(effSize, effSize, effSize)
		c.Position = c.Body.Position
	}

	if c.IsCarried {
		return
	}

	// Physics-driven rolling rotation when moving on the ground
	if c.Body != nil && c.Body.IsGrounded {
		speed := c.Body.GetHorizontalSpeed()
		if speed > 0.12 && effSize > 0.05 {
			rollRate := (speed / effSize) * (180.0 / math.Pi)
			c.Rotation.X += (c.Body.Velocity.Z / speed) * rollRate * dt
			c.Rotation.Z -= (c.Body.Velocity.X / speed) * rollRate * dt
		}
	} else if c.Body != nil && !c.Body.IsGrounded && (c.Body.AngularVelocity.X != 0 || c.Body.AngularVelocity.Y != 0 || c.Body.AngularVelocity.Z != 0) {
		// Mid-air flight tumble from angular velocity
		c.Rotation.X += c.Body.AngularVelocity.X * dt
		c.Rotation.Y += c.Body.AngularVelocity.Y * dt
		c.Rotation.Z += c.Body.AngularVelocity.Z * dt
	} else if c.AutoRotate {
		c.Rotation.X += c.RotationSpeed.X * dt
		c.Rotation.Y += c.RotationSpeed.Y * dt
		c.Rotation.Z += c.RotationSpeed.Z * dt
	}

	// Keep angles in [0, 360)
	c.Rotation.X = float32(math.Mod(float64(c.Rotation.X), 360.0))
	c.Rotation.Y = float32(math.Mod(float64(c.Rotation.Y), 360.0))
	c.Rotation.Z = float32(math.Mod(float64(c.Rotation.Z), 360.0))
}

// CycleMode advances to the next rainbow visual mode.
func (c *RainbowCube) CycleMode() {
	c.Mode = (c.Mode + 1) % rainbowModeCount
}

// SetMode directly sets the rainbow visual mode.
func (c *RainbowCube) SetMode(mode RainbowMode) {
	if mode >= 0 && mode < rainbowModeCount {
		c.Mode = mode
	}
}

// ToggleAutoRotate pauses or resumes auto-rotation.
func (c *RainbowCube) ToggleAutoRotate() {
	c.AutoRotate = !c.AutoRotate
}

// ToggleWireframe toggles the edge wireframe overlay.
func (c *RainbowCube) ToggleWireframe() {
	c.ShowWireframe = !c.ShowWireframe
}

// AdjustSpeed multiplies rotation speeds by a delta factor.
func (c *RainbowCube) AdjustSpeed(multiplier float32) {
	c.RotationSpeed.X *= multiplier
	c.RotationSpeed.Y *= multiplier
	c.RotationSpeed.Z *= multiplier

	// Clamp within reasonable limits
	const minSpeed = 5.0
	const maxSpeed = 300.0
	if math.Abs(float64(c.RotationSpeed.Y)) < minSpeed {
		sign := float32(1.0)
		if c.RotationSpeed.Y < 0 {
			sign = -1.0
		}
		c.RotationSpeed.X = 30.0 * sign
		c.RotationSpeed.Y = 45.0 * sign
		c.RotationSpeed.Z = 20.0 * sign
	} else if math.Abs(float64(c.RotationSpeed.Y)) > maxSpeed {
		c.RotationSpeed.X = 200.0
		c.RotationSpeed.Y = 300.0
		c.RotationSpeed.Z = 135.0
	}
}

// Draw renders the rotating rainbow cube in 3D space.
func (c *RainbowCube) Draw() {
	rl.PushMatrix()
	rl.Translatef(c.Position.X, c.Position.Y, c.Position.Z)

	// Apply rotations in standard Yaw (Y), Pitch (X), Roll (Z) order
	rl.Rotatef(c.Rotation.Y, 0.0, 1.0, 0.0)
	rl.Rotatef(c.Rotation.X, 1.0, 0.0, 0.0)
	rl.Rotatef(c.Rotation.Z, 0.0, 0.0, 1.0)

	s := c.EffectiveSize()

	switch c.Mode {
	case ModeFaceColors:
		c.drawFaceColors(s)
	case ModeVertexGradient:
		c.drawVertexGradient(s)
	case ModeChromaCycle:
		c.drawChromaCycle(s)
	}

	// Render wireframe outline
	if c.ShowWireframe {
		wireColor := c.WireframeColor
		if c.Mode == ModeFaceColors {
			wireColor = rl.NewColor(15, 15, 20, 240)
		} else {
			wireColor = rl.NewColor(255, 255, 255, 180)
		}
		// Scale slightly up (1.001) to avoid Z-fighting with faces
		edgeSize := s * 2.002
		rl.DrawCubeWires(rl.NewVector3(0, 0, 0), edgeSize, edgeSize, edgeSize, wireColor)
	}

	rl.PopMatrix()

	if c.ShowCollisionBox {
		col := c.CollisionBoxColor
		if col.A == 0 {
			col = rl.NewColor(0, 255, 128, 220)
		}
		rl.DrawBoundingBox(c.GetBoundingBox(), col)
	}
}

// drawFaceColors renders each face with a distinct vibrant rainbow hue.
func (c *RainbowCube) drawFaceColors(s float32) {
	// 6 distinct rainbow colors
	colFront := rl.NewColor(255, 59, 48, 255)   // Vivid Red (+Z)
	colBack := rl.NewColor(255, 149, 0, 255)   // Bright Orange (-Z)
	colTop := rl.NewColor(255, 204, 0, 255)    // Electric Yellow (+Y)
	colBottom := rl.NewColor(0, 122, 255, 255) // Royal Blue (-Y)
	colLeft := rl.NewColor(52, 199, 89, 255)   // Emerald Green (-X)
	colRight := rl.NewColor(175, 82, 222, 255) // Vivid Violet (+X)

	rl.Begin(rl.Quads)

	// Front Face (+Z) - Red
	c.setQuadColor(colFront)
	rl.Vertex3f(-s, -s, s)
	rl.Vertex3f(s, -s, s)
	rl.Vertex3f(s, s, s)
	rl.Vertex3f(-s, s, s)

	// Back Face (-Z) - Orange
	c.setQuadColor(colBack)
	rl.Vertex3f(s, -s, -s)
	rl.Vertex3f(-s, -s, -s)
	rl.Vertex3f(-s, s, -s)
	rl.Vertex3f(s, s, -s)

	// Top Face (+Y) - Yellow
	c.setQuadColor(colTop)
	rl.Vertex3f(-s, s, s)
	rl.Vertex3f(s, s, s)
	rl.Vertex3f(s, s, -s)
	rl.Vertex3f(-s, s, -s)

	// Bottom Face (-Y) - Blue
	c.setQuadColor(colBottom)
	rl.Vertex3f(-s, -s, -s)
	rl.Vertex3f(s, -s, -s)
	rl.Vertex3f(s, -s, s)
	rl.Vertex3f(-s, -s, s)

	// Right Face (+X) - Violet
	c.setQuadColor(colRight)
	rl.Vertex3f(s, -s, s)
	rl.Vertex3f(s, -s, -s)
	rl.Vertex3f(s, s, -s)
	rl.Vertex3f(s, s, s)

	// Left Face (-X) - Green
	c.setQuadColor(colLeft)
	rl.Vertex3f(-s, -s, -s)
	rl.Vertex3f(-s, -s, s)
	rl.Vertex3f(-s, s, s)
	rl.Vertex3f(-s, s, -s)

	rl.End()
}

// drawVertexGradient maps unique 3D color cube coordinates to each of the 8 vertices.
func (c *RainbowCube) drawVertexGradient(s float32) {
	// 8 Corner Vertices with distinct RGB spectrum colors
	v0 := vertexInfo{rl.NewVector3(-s, -s, -s), rl.NewColor(30, 10, 80, 255)}   // Dark Indigo
	v1 := vertexInfo{rl.NewVector3(s, -s, -s), rl.NewColor(255, 30, 40, 255)}   // Red
	v2 := vertexInfo{rl.NewVector3(s, s, -s), rl.NewColor(255, 240, 20, 255)}   // Yellow
	v3 := vertexInfo{rl.NewVector3(-s, s, -s), rl.NewColor(40, 230, 80, 255)}   // Green
	v4 := vertexInfo{rl.NewVector3(-s, -s, s), rl.NewColor(20, 180, 255, 255)}  // Cyan
	v5 := vertexInfo{rl.NewVector3(s, -s, s), rl.NewColor(255, 40, 200, 255)}   // Magenta
	v6 := vertexInfo{rl.NewVector3(s, s, s), rl.NewColor(255, 255, 255, 255)}   // White/Prism
	v7 := vertexInfo{rl.NewVector3(-s, s, s), rl.NewColor(160, 60, 255, 255)}   // Purple

	rl.Begin(rl.Quads)

	// Front Face: v4, v5, v6, v7
	c.emitVertex(v4)
	c.emitVertex(v5)
	c.emitVertex(v6)
	c.emitVertex(v7)

	// Back Face: v1, v0, v3, v2
	c.emitVertex(v1)
	c.emitVertex(v0)
	c.emitVertex(v3)
	c.emitVertex(v2)

	// Top Face: v7, v6, v2, v3
	c.emitVertex(v7)
	c.emitVertex(v6)
	c.emitVertex(v2)
	c.emitVertex(v3)

	// Bottom Face: v0, v1, v5, v4
	c.emitVertex(v0)
	c.emitVertex(v1)
	c.emitVertex(v5)
	c.emitVertex(v4)

	// Right Face: v5, v1, v2, v6
	c.emitVertex(v5)
	c.emitVertex(v1)
	c.emitVertex(v2)
	c.emitVertex(v6)

	// Left Face: v0, v4, v7, v3
	c.emitVertex(v0)
	c.emitVertex(v4)
	c.emitVertex(v7)
	c.emitVertex(v3)

	rl.End()
}

// drawChromaCycle animates rainbow hues dynamically across time.
func (c *RainbowCube) drawChromaCycle(s float32) {
	// Base hue shifts 60 degrees per second
	baseHue := float32(math.Mod(float64(c.Time*60.0), 360.0))

	// Generate 6 cycling hues offset by 60°
	colors := make([]rl.Color, 6)
	for i := 0; i < 6; i++ {
		hue := float32(math.Mod(float64(baseHue+float32(i*60)), 360.0))
		colors[i] = rl.ColorFromHSV(hue, 0.95, 1.0)
	}

	rl.Begin(rl.Quads)

	// Front Face
	c.setQuadColor(colors[0])
	rl.Vertex3f(-s, -s, s)
	rl.Vertex3f(s, -s, s)
	rl.Vertex3f(s, s, s)
	rl.Vertex3f(-s, s, s)

	// Back Face
	c.setQuadColor(colors[1])
	rl.Vertex3f(s, -s, -s)
	rl.Vertex3f(-s, -s, -s)
	rl.Vertex3f(-s, s, -s)
	rl.Vertex3f(s, s, -s)

	// Top Face
	c.setQuadColor(colors[2])
	rl.Vertex3f(-s, s, s)
	rl.Vertex3f(s, s, s)
	rl.Vertex3f(s, s, -s)
	rl.Vertex3f(-s, s, -s)

	// Bottom Face
	c.setQuadColor(colors[3])
	rl.Vertex3f(-s, -s, -s)
	rl.Vertex3f(s, -s, -s)
	rl.Vertex3f(s, -s, s)
	rl.Vertex3f(-s, -s, s)

	// Right Face
	c.setQuadColor(colors[4])
	rl.Vertex3f(s, -s, s)
	rl.Vertex3f(s, -s, -s)
	rl.Vertex3f(s, s, -s)
	rl.Vertex3f(s, s, s)

	// Left Face
	c.setQuadColor(colors[5])
	rl.Vertex3f(-s, -s, -s)
	rl.Vertex3f(-s, -s, s)
	rl.Vertex3f(-s, s, s)
	rl.Vertex3f(-s, s, -s)

	rl.End()
}

type vertexInfo struct {
	Pos rl.Vector3
	Col rl.Color
}

func (c *RainbowCube) emitVertex(v vertexInfo) {
	rl.Color4ub(v.Col.R, v.Col.G, v.Col.B, v.Col.A)
	rl.Vertex3f(v.Pos.X, v.Pos.Y, v.Pos.Z)
}

func (c *RainbowCube) setQuadColor(col rl.Color) {
	rl.Color4ub(col.R, col.G, col.B, col.A)
}

// FormattedStatus returns an easy-to-read summary string for the HUD.
func (c *RainbowCube) FormattedStatus() string {
	state := "RUNNING"
	if !c.AutoRotate {
		state = "PAUSED"
	}
	return fmt.Sprintf("State: %s | Yaw: %.1f° Pitch: %.1f° Roll: %.1f°",
		state, c.Rotation.Y, c.Rotation.X, c.Rotation.Z)
}
