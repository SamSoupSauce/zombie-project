package camera

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/human"
)

// ViewMode selects between First Person and Orbiting perspective.
type ViewMode int

const (
	ViewModeFirstPerson ViewMode = iota
	ViewModeOrbit
)

// FPSController implements a complete 3D First-Person controller with WASD movement,
// mouse look, jumping, gravity, head-bobbing, wall collision, and a universal 'human'
// stick-figure model as its parent object.
type FPSController struct {
	Human        *human.Human // Parent human model
	Position     rl.Vector3   // Player feet position (synced with Human.Position)
	Velocity     rl.Vector3
	EyeHeight    float32
	Yaw          float32 // Horizontal looking angle (radians, synced with Human.Yaw)
	Pitch        float32 // Vertical looking angle (radians, synced with Human.Pitch)
	Sensitivity  float32
	WalkSpeed    float32
	SprintSpeed  float32
	JumpSpeed    float32
	Gravity      float32
	GroundY      float32
	IsGrounded   bool
	PlayerRadius float32
	StepHeight   float32 // Maximum height of a ledge/staircase step the player can walk up without jumping

	BobTimer  float32
	BobAmount float32
	HeadBob   float32

	Camera     rl.Camera3D
	IsLocked   bool // Whether cursor is locked for mouselook
	justLocked bool // Discards initial centering delta upon locking
	wasFocused bool // Tracks window focus state to handle focus loss/regain
}

// NewFPSController initializes the first-person controller at the specified spawn position
// with an attached universal stick-figure human model as its parent object.
func NewFPSController(spawnPos rl.Vector3, groundY float32) *FPSController {
	parentHuman := human.NewHuman(spawnPos)

	fps := &FPSController{
		Human:        parentHuman,
		Position:     spawnPos,
		Velocity:     rl.NewVector3(0, 0, 0),
		EyeHeight:    parentHuman.EyeHeight,
		Yaw:          0.0,
		Pitch:        0.0,
		Sensitivity:  0.0022,
		WalkSpeed:    4.8,
		SprintSpeed:  8.2,
		JumpSpeed:    6.2,
		Gravity:      -18.0,
		GroundY:      groundY,
		IsGrounded:   spawnPos.Y <= groundY,
		PlayerRadius: 0.35,
		StepHeight:   0.65, // Generous ledge & staircase threshold to walk up without jumping
		BobAmount:    0.04,
		HeadBob:      0.0,
		IsLocked:     false,
		justLocked:   false,
		wasFocused:   true,
	}

	fps.Camera = rl.Camera3D{
		Position:   parentHuman.GetEyePosition(0.0),
		Target:     rl.NewVector3(spawnPos.X, spawnPos.Y+fps.EyeHeight, spawnPos.Z+1.0),
		Up:         rl.NewVector3(0.0, 1.0, 0.0),
		Fovy:       65.0,
		Projection: rl.CameraPerspective,
	}

	fps.updateCameraTarget()
	return fps
}

// LockCursor captures and hides the mouse cursor pointer for first-person mouselook,
// and physically confines the mouse pointer inside the window.
func (c *FPSController) LockCursor() {
	c.IsLocked = true
	c.justLocked = true
	c.wasFocused = true
	if rl.IsWindowReady() {
		rl.HideCursor()
		hidePlatformCursor()
		trapMouseToWindow()
		w := float32(rl.GetScreenWidth())
		h := float32(rl.GetScreenHeight())
		rl.SetMousePosition(int(w/2), int(h/2))
	}
}

// UnlockCursor restores and shows the mouse cursor pointer for UI or console interaction,
// releasing cursor confinement.
func (c *FPSController) UnlockCursor() {
	c.IsLocked = false
	c.justLocked = false
	if rl.IsWindowReady() {
		rl.ShowCursor()
		showPlatformCursor()
		releaseMouseFromWindow()
	}
}

// Teleport sets the player's position immediately and resets velocity.
func (c *FPSController) Teleport(pos rl.Vector3) {
	c.Position = pos
	c.Velocity = rl.NewVector3(0, 0, 0)
	if c.Human != nil {
		c.Human.Position = pos
		c.Human.Velocity = rl.NewVector3(0, 0, 0)
	}
	c.updateCameraTarget()
}

// Update handles mouse look, WASD movement, jumping, gravity, collision, and updates parent Human.
func (c *FPSController) Update(dt float32, solidBoxes []rl.BoundingBox) {
	// 1. Mouse Look and Cursor Confinement
	if c.IsLocked && rl.IsWindowReady() {
		if !rl.IsWindowFocused() {
			if c.wasFocused {
				releaseMouseFromWindow()
				rl.ShowCursor()
				showPlatformCursor()
				c.wasFocused = false
			}
		} else {
			if !c.wasFocused {
				trapMouseToWindow()
				rl.HideCursor()
				hidePlatformCursor()
				c.justLocked = true
				c.wasFocused = true
			}

			w := float32(rl.GetScreenWidth())
			h := float32(rl.GetScreenHeight())
			centerX := w / 2.0
			centerY := h / 2.0

			if c.justLocked {
				c.justLocked = false
				rl.HideCursor()
				hidePlatformCursor()
				trapMouseToWindow()
				rl.SetMousePosition(int(centerX), int(centerY))
			} else {
				trapMouseToWindow()

				mousePos := rl.GetMousePosition()
				dx := mousePos.X - centerX
				dy := mousePos.Y - centerY

				if dx != 0 || dy != 0 {
					rl.SetMousePosition(int(centerX), int(centerY))
					c.Yaw += dx * c.Sensitivity
					c.Pitch -= dy * c.Sensitivity

					// Clamp pitch to avoid neck snap (-88° to +88°)
					const maxPitch = 88.0 * (math.Pi / 180.0)
					if c.Pitch > maxPitch {
						c.Pitch = maxPitch
					} else if c.Pitch < -maxPitch {
						c.Pitch = -maxPitch
					}
				}
			}
		}
	}

	// 2. Movement Directions (Horizontal plane only)
	sinY := float32(math.Sin(float64(c.Yaw)))
	cosY := float32(math.Cos(float64(c.Yaw)))

	// In Raylib/OpenGL right-handed coordinates looking along +Z,
	// Screen Right is along -X and Screen Left is along +X.
	forward := rl.NewVector3(-sinY, 0.0, cosY)
	right := rl.NewVector3(-cosY, 0.0, -sinY)

	moveDir := rl.NewVector3(0, 0, 0)
	if rl.IsKeyDown(rl.KeyW) {
		moveDir = rl.Vector3Add(moveDir, forward)
	}
	if rl.IsKeyDown(rl.KeyS) {
		moveDir = rl.Vector3Subtract(moveDir, forward)
	}
	if rl.IsKeyDown(rl.KeyD) {
		moveDir = rl.Vector3Add(moveDir, right)
	}
	if rl.IsKeyDown(rl.KeyA) {
		moveDir = rl.Vector3Subtract(moveDir, right)
	}

	// Speed selection (Sprint on LeftShift)
	currentSpeed := c.WalkSpeed
	if rl.IsKeyDown(rl.KeyLeftShift) {
		currentSpeed = c.SprintSpeed
	}

	isMoving := moveDir.X != 0 || moveDir.Z != 0
	if isMoving {
		moveDir = rl.Vector3Normalize(moveDir)
	}

	// 3. Jump & Vertical Gravity
	if c.IsGrounded && rl.IsKeyPressed(rl.KeySpace) {
		c.Velocity.Y = c.JumpSpeed
		c.IsGrounded = false
	}

	// Apply gravity
	c.Velocity.Y += c.Gravity * dt

	// 4. Position Integration & Wall Collision with Step-Up/Ledge Traversal
	deltaH := rl.Vector3Scale(moveDir, currentSpeed*dt)
	c.MoveWithCollisions(deltaH, dt, solidBoxes)

	// 5. Head Bobbing
	headBob := float32(0.0)
	if isMoving && c.IsGrounded {
		bobFreq := float32(9.0)
		if currentSpeed == c.SprintSpeed {
			bobFreq = 13.0
		}
		c.BobTimer += dt * bobFreq
		headBob = float32(math.Sin(float64(c.BobTimer))) * c.BobAmount
	} else {
		c.BobTimer = 0.0
	}
	c.HeadBob = headBob

	// 6. Synchronize state with parent Human stick-figure model
	if c.Human != nil {
		c.Human.Position = c.Position
		c.Human.Yaw = c.Yaw
		c.Human.Pitch = c.Pitch
		c.Human.IsMoving = isMoving
		c.Human.IsGrounded = c.IsGrounded
		c.Human.WalkSpeed = currentSpeed
		c.Human.Update(dt)
	}

	// 7. Update Camera Position and Look Direction
	if c.Human != nil {
		c.Camera.Position = c.Human.GetEyePosition(headBob)
	} else {
		eyeY := c.Position.Y + c.EyeHeight + headBob
		c.Camera.Position = rl.NewVector3(c.Position.X, eyeY, c.Position.Z)
	}
	c.updateCameraTarget()
}

// DrawFirstPersonHands renders the visible hands with 5 individual articulated fingers
// of the parent human stick-figure model in the first-person perspective.
func (c *FPSController) DrawFirstPersonHands() {
	if c.Human != nil {
		swayX := float32(math.Cos(float64(c.BobTimer*0.5))) * c.HeadBob * 0.4
		bobOffset := rl.NewVector3(swayX, c.HeadBob, 0.0)
		c.Human.DrawFirstPersonHands(c.Camera, bobOffset)
	}
}

// DrawThirdPerson renders the full parent human stick figure in world space.
func (c *FPSController) DrawThirdPerson() {
	if c.Human != nil {
		c.Human.DrawThirdPerson()
	}
}

func (c *FPSController) updateCameraTarget() {
	cosP := float32(math.Cos(float64(c.Pitch)))
	sinP := float32(math.Sin(float64(c.Pitch)))
	sinY := float32(math.Sin(float64(c.Yaw)))
	cosY := float32(math.Cos(float64(c.Yaw)))

	lookDir := rl.NewVector3(-cosP*sinY, sinP, cosP*cosY)
	c.Camera.Target = rl.Vector3Add(c.Camera.Position, lookDir)
}

// GetLookRay returns a 3D ray projecting forward from the player's eyes (for crosshair picking).
func (c *FPSController) GetLookRay() rl.Ray {
	cosP := float32(math.Cos(float64(c.Pitch)))
	sinP := float32(math.Sin(float64(c.Pitch)))
	sinY := float32(math.Sin(float64(c.Yaw)))
	cosY := float32(math.Cos(float64(c.Yaw)))

	lookDir := rl.NewVector3(-cosP*sinY, sinP, cosP*cosY)
	return rl.Ray{
		Position:  c.Camera.Position,
		Direction: rl.Vector3Normalize(lookDir),
	}
}

// MoveWithCollisions moves the controller horizontally by deltaH, automatically
// stepping up over ledges, curbs, and staircases (<= StepHeight), resolving wall collisions,
// and applying gravity and ground collision.
func (c *FPSController) MoveWithCollisions(deltaH rl.Vector3, dt float32, solidBoxes []rl.BoundingBox) {
	newPos := c.Position
	newPos.X += deltaH.X
	newPos.Z += deltaH.Z

	// 1. Resolve collisions against walls taller than StepHeight
	newPos = c.resolveWallCollisions(newPos, solidBoxes)

	// 2. Step-Up for ledges, curbs, and staircases (obstacle height <= StepHeight)
	stepH := c.StepHeight
	r := c.PlayerRadius
	startFeet := c.Position.Y
	stepUpY := startFeet

	for _, b := range solidBoxes {
		if newPos.X+r > b.Min.X && newPos.X-r < b.Max.X &&
			newPos.Z+r > b.Min.Z && newPos.Z-r < b.Max.Z {
			// Check if obstacle is a step above our feet and within step threshold
			if b.Max.Y > startFeet && b.Max.Y <= startFeet+stepH {
				if b.Max.Y > stepUpY {
					// Verify headroom above the step
					hasHeadroom := true
					for _, other := range solidBoxes {
						if other.Min.Y >= b.Max.Y && other.Min.Y < b.Max.Y+c.EyeHeight {
							if newPos.X+r > other.Min.X && newPos.X-r < other.Max.X &&
								newPos.Z+r > other.Min.Z && newPos.Z-r < other.Max.Z {
								hasHeadroom = false
								break
							}
						}
					}
					if hasHeadroom {
						stepUpY = b.Max.Y
					}
				}
			}
		}
	}

	if stepUpY > startFeet {
		newPos.Y = stepUpY
		if c.Velocity.Y < 0 {
			c.Velocity.Y = 0
		}
		c.IsGrounded = true
	} else {
		newPos.Y += c.Velocity.Y * dt
	}

	// 3. Ground plane and walkable solid box collision
	groundLevel := c.GroundY
	for _, b := range solidBoxes {
		if newPos.X+r*0.5 > b.Min.X && newPos.X-r*0.5 < b.Max.X &&
			newPos.Z+r*0.5 > b.Min.Z && newPos.Z-r*0.5 < b.Max.Z {
			if b.Max.Y <= startFeet+stepH+0.05 {
				if b.Max.Y > groundLevel {
					groundLevel = b.Max.Y
				}
			}
		}
	}

	if newPos.Y <= groundLevel {
		newPos.Y = groundLevel
		c.Velocity.Y = 0
		c.IsGrounded = true
	} else if c.IsGrounded && c.Velocity.Y <= 0 && (startFeet-groundLevel) <= stepH && (startFeet-groundLevel) >= 0 {
		// Step-down snap when walking down stairs/curbs
		newPos.Y = groundLevel
		c.Velocity.Y = 0
		c.IsGrounded = true
	} else {
		c.IsGrounded = false
	}

	c.Position = newPos
}

// resolveWallCollisions prevents the player from passing through solid bounding boxes taller than StepHeight.
func (c *FPSController) resolveWallCollisions(newPos rl.Vector3, boxes []rl.BoundingBox) rl.Vector3 {
	r := c.PlayerRadius
	stepH := c.StepHeight
	startFeet := c.Position.Y

	// Test X-axis movement
	for _, b := range boxes {
		obstacleHeightAboveFeet := b.Max.Y - startFeet
		// Only obstacles taller than StepHeight block horizontal movement
		if obstacleHeightAboveFeet > stepH && b.Min.Y < startFeet+c.EyeHeight {
			if newPos.X+r > b.Min.X && newPos.X-r < b.Max.X &&
				c.Position.Z+r > b.Min.Z && c.Position.Z-r < b.Max.Z {
				newPos.X = c.Position.X
				break
			}
		}
	}

	// Test Z-axis movement
	for _, b := range boxes {
		obstacleHeightAboveFeet := b.Max.Y - startFeet
		// Only obstacles taller than StepHeight block horizontal movement
		if obstacleHeightAboveFeet > stepH && b.Min.Y < startFeet+c.EyeHeight {
			if newPos.X+r > b.Min.X && newPos.X-r < b.Max.X &&
				newPos.Z+r > b.Min.Z && newPos.Z-r < b.Max.Z {
				newPos.Z = c.Position.Z
				break
			}
		}
	}

	return newPos
}

// SetStepHeight adjusts the maximum ledge/staircase height the player can step up without jumping.
func (c *FPSController) SetStepHeight(height float32) {
	if height >= 0.0 && height <= 3.0 {
		c.StepHeight = height
	}
}

// GetStepHeight returns the current maximum ledge/staircase step-up height.
func (c *FPSController) GetStepHeight() float32 {
	return c.StepHeight
}
