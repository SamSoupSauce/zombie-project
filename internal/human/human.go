package human

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/prop"
)

// MaxPickupMass defines the maximum weight in kilograms that the human can lift.
// Props with mass strictly LESS than 50.0 kg (< 50kg) can be picked up.
const MaxPickupMass = float32(50.0)

// Finger represents a single articulated finger on a hand.
type Finger struct {
	Name    string
	Knuckle rl.Vector3
	Joint   rl.Vector3
	Tip     rl.Vector3
}

// Hand represents a hand with palm and 5 distinct fingers (Thumb, Index, Middle, Ring, Pinky).
type Hand struct {
	Wrist   rl.Vector3
	Palm    rl.Vector3
	Fingers [5]Finger
}

// Human represents a universal 3D stick-figure character model with full procedural
// anatomy: head, face (eyes, smile), neck, spine, shoulders, arms, hands with 5 individual
// fingers, pelvis, legs, and flat feet. It acts as the parent object of the FPSController.
type Human struct {
	ID        string
	Name      string
	Position  rl.Vector3 // World position of feet base
	Velocity  rl.Vector3
	Yaw       float32 // Rotation angle around Y axis (radians)
	Pitch     float32 // Vertical head looking pitch (radians)
	EyeHeight float32 // Height from feet to eye level (~1.65m)

	WalkCycle  float32 // Accumulator driving walk animations
	IsMoving   bool
	IsGrounded bool
	WalkSpeed  float32

	HeldProp prop.Prop // Currently carried prop (< 50kg)

	// Stick figure style aesthetics
	BodyColor     rl.Color // Bone white / silver stick lines
	JointColor    rl.Color // Soft metallic joint spheres
	EyeColor      rl.Color // Dark pupil color
	PupilHighlight rl.Color // White reflection highlight
	SmileColor    rl.Color // Cheerful facial expression color
	HandColor     rl.Color // Hand & finger color

	IdleTimer  float32
	ThrowTimer float32 // Active during throw forward thrust animation
}

// NewHuman creates a universal stick-figure human model at the specified spawn position.
func NewHuman(spawnPos rl.Vector3) *Human {
	return &Human{
		ID:             "human_player",
		Name:           "Human Stick Figure",
		Position:       spawnPos,
		Velocity:       rl.NewVector3(0, 0, 0),
		Yaw:            0.0,
		Pitch:          0.0,
		EyeHeight:      1.65,
		WalkCycle:      0.0,
		IsMoving:       false,
		IsGrounded:     true,
		WalkSpeed:      0.0,
		HeldProp:       nil,
		BodyColor:      rl.NewColor(235, 238, 245, 255), // Sleek clean stick-figure ivory
		JointColor:     rl.NewColor(185, 195, 210, 255), // Joint connectors
		EyeColor:       rl.NewColor(20, 25, 40, 255),    // Deep obsidian eyes
		PupilHighlight: rl.NewColor(255, 255, 255, 255), // Reflection glint
		SmileColor:     rl.NewColor(30, 35, 50, 255),    // Smile line
		HandColor:      rl.NewColor(238, 242, 250, 255), // Hand surface
		IdleTimer:      0.0,
	}
}

// GetEyePosition returns the world-space camera eye position.
func (h *Human) GetEyePosition(headBob float32) rl.Vector3 {
	return rl.NewVector3(h.Position.X, h.Position.Y+h.EyeHeight+headBob, h.Position.Z)
}

// GetForwardVector returns the horizontal forward direction vector.
func (h *Human) GetForwardVector() rl.Vector3 {
	sinY := float32(math.Sin(float64(h.Yaw)))
	cosY := float32(math.Cos(float64(h.Yaw)))
	return rl.NewVector3(-sinY, 0.0, cosY)
}

// GetRightVector returns the horizontal right direction vector.
func (h *Human) GetRightVector() rl.Vector3 {
	sinY := float32(math.Sin(float64(h.Yaw)))
	cosY := float32(math.Cos(float64(h.Yaw)))
	return rl.NewVector3(-cosY, 0.0, -sinY)
}

// CanPickUp checks if the given prop satisfies the strict < 50kg pickup condition.
func (h *Human) CanPickUp(p prop.Prop) (bool, string) {
	if p == nil {
		return false, "No prop specified"
	}
	mass := p.GetMass()
	if mass < MaxPickupMass {
		return true, ""
	}
	return false, fmt.Sprintf("Cannot lift %s: %.1f kg is too heavy! (Limit is < %.1f kg)", p.GetName(), mass, MaxPickupMass)
}

// PickUp attaches a prop if it is < 50kg. Returns success and a user-facing status message.
func (h *Human) PickUp(p prop.Prop) (bool, string) {
	if h.HeldProp != nil {
		return false, fmt.Sprintf("Hands full: already carrying %s", h.HeldProp.GetName())
	}
	canLift, reason := h.CanPickUp(p)
	if !canLift {
		return false, reason
	}

	h.HeldProp = p
	p.SetHeld(true)
	return true, fmt.Sprintf("Picked up %s (%.1f kg)", p.GetName(), p.GetMass())
}

// DropProp releases the currently held prop. Returns the dropped prop or nil.
func (h *Human) DropProp() prop.Prop {
	if h.HeldProp == nil {
		return nil
	}
	p := h.HeldProp
	p.SetHeld(false)
	h.HeldProp = nil
	return p
}

// TriggerThrow triggers a forward hand thrust animation upon throwing a prop.
func (h *Human) TriggerThrow() {
	h.ThrowTimer = 0.22
}

// Update advances the human walk cycle and idle breathing animation timers.
func (h *Human) Update(dt float32) {
	h.IdleTimer += dt
	if h.ThrowTimer > 0 {
		h.ThrowTimer -= dt
		if h.ThrowTimer < 0 {
			h.ThrowTimer = 0
		}
	}
	if h.IsMoving && h.IsGrounded {
		cycleSpeed := float32(8.5)
		if h.WalkSpeed > 5.5 {
			cycleSpeed = 13.0
		}
		h.WalkCycle += dt * cycleSpeed
	} else {
		// Smoothly return walk cycle towards neutral pose
		h.WalkCycle = float32(math.Mod(float64(h.WalkCycle), 2.0*math.Pi))
		if h.WalkCycle > 0.05 {
			h.WalkCycle -= dt * 6.0
			if h.WalkCycle < 0 {
				h.WalkCycle = 0
			}
		}
	}
}

// DrawThirdPerson renders the full 3D stick-figure human model in world space:
// head with face (eyes, smile), spine, shoulders, arms, hands with 5 individual fingers,
// hips, legs, and flat feet.
func (h *Human) DrawThirdPerson() {
	rl.PushMatrix()
	rl.Translatef(h.Position.X, h.Position.Y, h.Position.Z)

	// Rotate whole body according to horizontal Yaw angle
	yawDeg := h.Yaw * (180.0 / math.Pi)
	rl.Rotatef(-yawDeg, 0.0, 1.0, 0.0)

	// Colors
	stickCol := h.BodyColor
	jointCol := h.JointColor

	// Limb thicknesses
	const spineRadius = float32(0.045)
	const limbRadius = float32(0.035)
	const jointRadius = float32(0.042)

	// 1. Pelvis / Hips Bar
	pelvisY := float32(0.86)
	leftHip := rl.NewVector3(-0.16, pelvisY, 0.0)
	rightHip := rl.NewVector3(0.16, pelvisY, 0.0)
	hipCenter := rl.NewVector3(0.0, pelvisY, 0.0)
	rl.DrawCylinderEx(leftHip, rightHip, limbRadius, limbRadius, 8, stickCol)
	rl.DrawSphere(leftHip, jointRadius, jointCol)
	rl.DrawSphere(rightHip, jointRadius, jointCol)
	rl.DrawSphere(hipCenter, jointRadius, jointCol)

	// 2. Spine / Torso (from Pelvis to Shoulders)
	shoulderY := float32(1.40)
	shoulderCenter := rl.NewVector3(0.0, shoulderY, 0.0)
	rl.DrawCylinderEx(hipCenter, shoulderCenter, spineRadius, spineRadius, 10, stickCol)

	// 3. Shoulders Bar
	leftShoulder := rl.NewVector3(-0.24, shoulderY, 0.0)
	rightShoulder := rl.NewVector3(0.24, shoulderY, 0.0)
	rl.DrawCylinderEx(leftShoulder, rightShoulder, limbRadius, limbRadius, 8, stickCol)
	rl.DrawSphere(leftShoulder, jointRadius, jointCol)
	rl.DrawSphere(rightShoulder, jointRadius, jointCol)
	rl.DrawSphere(shoulderCenter, jointRadius, jointCol)

	// 4. Neck & Head
	neckTop := rl.NewVector3(0.0, 1.48, 0.0)
	rl.DrawCylinderEx(shoulderCenter, neckTop, limbRadius, limbRadius, 8, stickCol)

	headCenter := rl.NewVector3(0.0, 1.65, 0.0)
	headRadius := float32(0.18)

	// Head rotation (Pitch)
	rl.PushMatrix()
	rl.Translatef(headCenter.X, headCenter.Y, headCenter.Z)
	pitchDeg := h.Pitch * (180.0 / math.Pi)
	rl.Rotatef(pitchDeg, 1.0, 0.0, 0.0)

	// Stick Figure Head Sphere
	rl.DrawSphere(rl.NewVector3(0, 0, 0), headRadius, stickCol)

	// Face: Eyes
	eyeY := float32(0.04)
	eyeZ := float32(0.16)
	eyeSpacing := float32(0.065)
	eyeRadius := float32(0.026)
	leftEye := rl.NewVector3(-eyeSpacing, eyeY, eyeZ)
	rightEye := rl.NewVector3(eyeSpacing, eyeY, eyeZ)
	rl.DrawSphere(leftEye, eyeRadius, h.EyeColor)
	rl.DrawSphere(rightEye, eyeRadius, h.EyeColor)

	// Eye Pupil Highlights (spark of life!)
	glintRadius := float32(0.010)
	leftGlint := rl.NewVector3(-eyeSpacing+0.01, eyeY+0.01, eyeZ+0.018)
	rightGlint := rl.NewVector3(eyeSpacing+0.01, eyeY+0.01, eyeZ+0.018)
	rl.DrawSphere(leftGlint, glintRadius, h.PupilHighlight)
	rl.DrawSphere(rightGlint, glintRadius, h.PupilHighlight)

	// Face: Cheerful Smile Arc
	mouthY := float32(-0.06)
	mouthZ := float32(0.165)
	p0 := rl.NewVector3(-0.060, mouthY+0.025, mouthZ-0.015)
	p1 := rl.NewVector3(-0.030, mouthY, mouthZ)
	p2 := rl.NewVector3(0.000, mouthY-0.01, mouthZ+0.005)
	p3 := rl.NewVector3(0.030, mouthY, mouthZ)
	p4 := rl.NewVector3(0.060, mouthY+0.025, mouthZ-0.015)

	smileThickness := float32(0.008)
	rl.DrawCylinderEx(p0, p1, smileThickness, smileThickness, 6, h.SmileColor)
	rl.DrawCylinderEx(p1, p2, smileThickness, smileThickness, 6, h.SmileColor)
	rl.DrawCylinderEx(p2, p3, smileThickness, smileThickness, 6, h.SmileColor)
	rl.DrawCylinderEx(p3, p4, smileThickness, smileThickness, 6, h.SmileColor)

	rl.PopMatrix() // End head/face

	// 5. Legs & Feet
	legSwing := float32(math.Sin(float64(h.WalkCycle))) * 0.45
	if !h.IsMoving {
		legSwing = 0.0
	}

	// Left Leg (swings forward/back)
	kneeY := float32(0.44)
	leftKneeZ := legSwing * 0.35
	leftKnee := rl.NewVector3(-0.16, kneeY, leftKneeZ)
	leftAnkleZ := -legSwing * 0.25
	leftAnkle := rl.NewVector3(-0.16, 0.05, leftAnkleZ)

	rl.DrawCylinderEx(leftHip, leftKnee, limbRadius, limbRadius, 8, stickCol)
	rl.DrawSphere(leftKnee, jointRadius, jointCol)
	rl.DrawCylinderEx(leftKnee, leftAnkle, limbRadius, limbRadius, 8, stickCol)
	rl.DrawSphere(leftAnkle, jointRadius, jointCol)

	// Left Flat Foot (flat on ground, pointing forward)
	leftFootToe := rl.NewVector3(-0.16, 0.04, leftAnkleZ+0.16)
	leftFootHeel := rl.NewVector3(-0.16, 0.04, leftAnkleZ-0.05)
	rl.DrawCylinderEx(leftFootHeel, leftFootToe, 0.038, 0.030, 8, stickCol)
	rl.DrawSphere(leftFootToe, 0.032, stickCol)

	// Right Leg
	rightKneeZ := -legSwing * 0.35
	rightKnee := rl.NewVector3(0.16, kneeY, rightKneeZ)
	rightAnkleZ := legSwing * 0.25
	rightAnkle := rl.NewVector3(0.16, 0.05, rightAnkleZ)

	rl.DrawCylinderEx(rightHip, rightKnee, limbRadius, limbRadius, 8, stickCol)
	rl.DrawSphere(rightKnee, jointRadius, jointCol)
	rl.DrawCylinderEx(rightKnee, rightAnkle, limbRadius, limbRadius, 8, stickCol)
	rl.DrawSphere(rightAnkle, jointRadius, jointCol)

	// Right Flat Foot (flat on ground, pointing forward)
	rightFootToe := rl.NewVector3(0.16, 0.04, rightAnkleZ+0.16)
	rightFootHeel := rl.NewVector3(0.16, 0.04, rightAnkleZ-0.05)
	rl.DrawCylinderEx(rightFootHeel, rightFootToe, 0.038, 0.030, 8, stickCol)
	rl.DrawSphere(rightFootToe, 0.032, stickCol)

	// 6. Arms and Hands with 5 Individual Fingers
	if h.HeldProp != nil {
		// Both arms forward holding the prop
		heldPropDist := float32(0.55)
		heldPropY := float32(1.10)

		leftElbow := rl.NewVector3(-0.28, 1.20, 0.20)
		leftWrist := rl.NewVector3(-0.20, heldPropY, heldPropDist)
		rl.DrawCylinderEx(leftShoulder, leftElbow, limbRadius, limbRadius, 8, stickCol)
		rl.DrawSphere(leftElbow, jointRadius, jointCol)
		rl.DrawCylinderEx(leftElbow, leftWrist, limbRadius, limbRadius, 8, stickCol)
		rl.DrawSphere(leftWrist, jointRadius, jointCol)
		h.drawHandWithFingers(leftWrist, rl.NewVector3(0.08, 0.0, 0.06), false)

		rightElbow := rl.NewVector3(0.28, 1.20, 0.20)
		rightWrist := rl.NewVector3(0.20, heldPropY, heldPropDist)
		rl.DrawCylinderEx(rightShoulder, rightElbow, limbRadius, limbRadius, 8, stickCol)
		rl.DrawSphere(rightElbow, jointRadius, jointCol)
		rl.DrawCylinderEx(rightElbow, rightWrist, limbRadius, limbRadius, 8, stickCol)
		rl.DrawSphere(rightWrist, jointRadius, jointCol)
		h.drawHandWithFingers(rightWrist, rl.NewVector3(-0.08, 0.0, 0.06), true)
	} else {
		// Arms swinging naturally with walk cycle
		armSwing := float32(math.Sin(float64(h.WalkCycle))) * 0.40
		if !h.IsMoving {
			armSwing = 0.0
		}

		// Left Arm
		leftElbow := rl.NewVector3(-0.26, 1.15, -armSwing*0.25)
		leftWrist := rl.NewVector3(-0.28, 0.90, -armSwing*0.35+0.05)
		rl.DrawCylinderEx(leftShoulder, leftElbow, limbRadius, limbRadius, 8, stickCol)
		rl.DrawSphere(leftElbow, jointRadius, jointCol)
		rl.DrawCylinderEx(leftElbow, leftWrist, limbRadius, limbRadius, 8, stickCol)
		rl.DrawSphere(leftWrist, jointRadius, jointCol)
		h.drawHandWithFingers(leftWrist, rl.NewVector3(0.0, 0.0, 0.08), false)

		// Right Arm
		rightElbow := rl.NewVector3(0.26, 1.15, armSwing*0.25)
		rightWrist := rl.NewVector3(0.28, 0.90, armSwing*0.35+0.05)
		rl.DrawCylinderEx(rightShoulder, rightElbow, limbRadius, limbRadius, 8, stickCol)
		rl.DrawSphere(rightElbow, jointRadius, jointCol)
		rl.DrawCylinderEx(rightElbow, rightWrist, limbRadius, limbRadius, 8, stickCol)
		rl.DrawSphere(rightWrist, jointRadius, jointCol)
		h.drawHandWithFingers(rightWrist, rl.NewVector3(0.0, 0.0, 0.08), true)
	}

	rl.PopMatrix()
}

// drawHandWithFingers draws palm and 5 distinct articulating fingers (Thumb, Index, Middle, Ring, Pinky).
func (h *Human) drawHandWithFingers(wrist rl.Vector3, dir rl.Vector3, isRight bool) {
	handCol := h.HandColor
	fingerCol := h.JointColor
	const fRadius = float32(0.007)
	const palmRadius = float32(0.024)

	// Palm center
	palm := rl.Vector3Add(wrist, rl.Vector3Scale(dir, 0.5))
	rl.DrawSphere(palm, palmRadius, handCol)

	// Finger directions
	forward := rl.Vector3Normalize(dir)
	up := rl.NewVector3(0.0, 1.0, 0.0)
	side := rl.Vector3Normalize(rl.Vector3CrossProduct(forward, up))
	if !isRight {
		side = rl.Vector3Scale(side, -1.0)
	}

	// 5 distinct fingers
	fingerDefs := []struct {
		name    string
		offset  float32 // lateral offset
		length1 float32
		length2 float32
		spread  float32
	}{
		{"Thumb", -0.018, 0.024, 0.020, -0.45},
		{"Index", -0.008, 0.032, 0.025, -0.10},
		{"Middle", 0.000, 0.038, 0.028, 0.00},
		{"Ring", 0.008, 0.034, 0.025, 0.08},
		{"Pinky", 0.016, 0.026, 0.020, 0.22},
	}

	for _, fd := range fingerDefs {
		knuckle := rl.Vector3Add(palm, rl.Vector3Scale(side, fd.offset))
		knuckle = rl.Vector3Add(knuckle, rl.Vector3Scale(forward, 0.015))

		// Joint 1
		fDir := rl.Vector3Add(forward, rl.Vector3Scale(side, fd.spread))
		fDir = rl.Vector3Normalize(fDir)

		joint := rl.Vector3Add(knuckle, rl.Vector3Scale(fDir, fd.length1))
		tip := rl.Vector3Add(joint, rl.Vector3Scale(fDir, fd.length2))

		// Draw knuckle, bone 1, joint, bone 2, tip
		rl.DrawSphere(knuckle, fRadius*1.3, fingerCol)
		rl.DrawCylinderEx(knuckle, joint, fRadius, fRadius, 6, handCol)
		rl.DrawSphere(joint, fRadius*1.1, fingerCol)
		rl.DrawCylinderEx(joint, tip, fRadius, fRadius*0.8, 6, handCol)
		rl.DrawSphere(tip, fRadius*0.9, fingerCol)
	}
}

// DrawFirstPersonHands renders the player's hands and 5 articulated fingers directly
// in front of the camera view, with natural walking sway, idle motion, and held-prop grasping.
func (h *Human) DrawFirstPersonHands(camera rl.Camera3D, bobOffset rl.Vector3) {
	// Calculate camera local coordinate basis
	lookDir := rl.Vector3Normalize(rl.Vector3Subtract(camera.Target, camera.Position))
	upRef := camera.Up
	rightDir := rl.Vector3Normalize(rl.Vector3CrossProduct(lookDir, upRef))
	camUp := rl.Vector3CrossProduct(rightDir, lookDir)

	// Helper to transform local camera coordinates (lx: right, ly: up, lz: forward) to world
	toWorld := func(lx, ly, lz float32) rl.Vector3 {
		p := camera.Position
		p = rl.Vector3Add(p, rl.Vector3Scale(rightDir, lx))
		p = rl.Vector3Add(p, rl.Vector3Scale(camUp, ly))
		p = rl.Vector3Add(p, rl.Vector3Scale(lookDir, lz))
		return p
	}

	handCol := h.HandColor
	stickCol := h.BodyColor
	jointCol := h.JointColor

	// Idle subtle sway
	idleSwayX := float32(math.Sin(float64(h.IdleTimer*1.8))) * 0.008
	idleSwayY := float32(math.Cos(float64(h.IdleTimer*2.2))) * 0.006

	totalBobX := bobOffset.X + idleSwayX
	totalBobY := bobOffset.Y + idleSwayY

	// Throw push forward thrust
	throwPushZ := float32(0.0)
	if h.ThrowTimer > 0 {
		progress := h.ThrowTimer / 0.22
		throwPushZ = float32(math.Sin(float64(progress*math.Pi))) * 0.16
	}

	if h.HeldProp != nil {
		// When holding a prop: hands reach inward grasping either side of the held prop
		propHalfW := float32(0.22)

		// Left Hand grasping prop
		lWrist := toWorld(-propHalfW-0.12+totalBobX, -0.22+totalBobY, 0.62)
		lPalm := toWorld(-propHalfW-0.03+totalBobX, -0.19+totalBobY, 0.69)
		lForearmBase := toWorld(-0.46, -0.48, 0.35)

		rl.DrawCylinderEx(lForearmBase, lWrist, 0.038, 0.032, 8, stickCol)
		rl.DrawSphere(lWrist, 0.035, jointCol)
		rl.DrawSphere(lPalm, 0.026, handCol)
		h.drawFPHandsFingers(lPalm, rightDir, camUp, lookDir, false, true)

		// Right Hand grasping prop
		rWrist := toWorld(propHalfW+0.12+totalBobX, -0.22+totalBobY, 0.62)
		rPalm := toWorld(propHalfW+0.03+totalBobX, -0.19+totalBobY, 0.69)
		rForearmBase := toWorld(0.46, -0.48, 0.35)

		rl.DrawCylinderEx(rForearmBase, rWrist, 0.038, 0.032, 8, stickCol)
		rl.DrawSphere(rWrist, 0.035, jointCol)
		rl.DrawSphere(rPalm, 0.026, handCol)
		h.drawFPHandsFingers(rPalm, rightDir, camUp, lookDir, true, true)
	} else {
		// Natural First-Person Ready Hands (visible lower left and lower right)
		// Left Arm & Hand
		lForearmBase := toWorld(-0.42, -0.45, 0.32)
		lWrist := toWorld(-0.25+totalBobX, -0.22+totalBobY, 0.52+throwPushZ)
		lPalm := toWorld(-0.23+totalBobX, -0.18+totalBobY, 0.59+throwPushZ)

		rl.DrawCylinderEx(lForearmBase, lWrist, 0.036, 0.030, 8, stickCol)
		rl.DrawSphere(lWrist, 0.032, jointCol)
		rl.DrawSphere(lPalm, 0.025, handCol)
		h.drawFPHandsFingers(lPalm, rightDir, camUp, lookDir, false, false)

		// Right Arm & Hand
		rForearmBase := toWorld(0.42, -0.45, 0.32)
		rWrist := toWorld(0.25+totalBobX, -0.22+totalBobY, 0.52+throwPushZ)
		rPalm := toWorld(0.23+totalBobX, -0.18+totalBobY, 0.59+throwPushZ)

		rl.DrawCylinderEx(rForearmBase, rWrist, 0.036, 0.030, 8, stickCol)
		rl.DrawSphere(rWrist, 0.032, jointCol)
		rl.DrawSphere(rPalm, 0.025, handCol)
		h.drawFPHandsFingers(rPalm, rightDir, camUp, lookDir, true, false)
	}
}

// drawFPHandsFingers draws 5 articulated fingers for a first-person hand.
func (h *Human) drawFPHandsFingers(palmPos, rightDir, camUp, lookDir rl.Vector3, isRight bool, isGrabbing bool) {
	handCol := h.HandColor
	jointCol := h.JointColor
	const fRad = float32(0.0075)

	sideSign := float32(-1.0)
	if isRight {
		sideSign = 1.0
	}

	fingerConfigs := []struct {
		name       string
		sideOffset float32
		upOffset   float32
		fwdOffset  float32
		len1       float32
		len2       float32
		curlPitch  float32
		yawSpread  float32
	}{
		// 1. Thumb
		{"Thumb", -0.022 * sideSign, 0.012, 0.010, 0.024, 0.020, -0.20, -0.35 * sideSign},
		// 2. Index Finger
		{"Index", -0.010 * sideSign, 0.006, 0.022, 0.032, 0.024, 0.35, -0.10 * sideSign},
		// 3. Middle Finger (longest)
		{"Middle", 0.000 * sideSign, 0.004, 0.024, 0.038, 0.028, 0.40, 0.00},
		// 4. Ring Finger
		{"Ring", 0.010 * sideSign, 0.002, 0.022, 0.034, 0.025, 0.45, 0.10 * sideSign},
		// 5. Pinky Finger
		{"Pinky", 0.020 * sideSign, -0.002, 0.018, 0.026, 0.020, 0.50, 0.22 * sideSign},
	}

	for _, fc := range fingerConfigs {
		// Knuckle root position on the palm
		knuckle := palmPos
		knuckle = rl.Vector3Add(knuckle, rl.Vector3Scale(rightDir, fc.sideOffset))
		knuckle = rl.Vector3Add(knuckle, rl.Vector3Scale(camUp, fc.upOffset))
		knuckle = rl.Vector3Add(knuckle, rl.Vector3Scale(lookDir, fc.fwdOffset))

		// Calculate direction of finger segments
		fwd := lookDir
		if isGrabbing {
			// Curl fingers inward toward center
			fwd = rl.Vector3Add(fwd, rl.Vector3Scale(rightDir, -0.30*sideSign))
		} else {
			fwd = rl.Vector3Add(fwd, rl.Vector3Scale(camUp, -fc.curlPitch))
			fwd = rl.Vector3Add(fwd, rl.Vector3Scale(rightDir, fc.yawSpread))
		}
		fwd = rl.Vector3Normalize(fwd)

		// First segment
		joint1 := rl.Vector3Add(knuckle, rl.Vector3Scale(fwd, fc.len1))

		// Second segment (curls further)
		fwd2 := fwd
		fwd2 = rl.Vector3Add(fwd2, rl.Vector3Scale(camUp, -0.15))
		if isGrabbing {
			fwd2 = rl.Vector3Add(fwd2, rl.Vector3Scale(rightDir, -0.25*sideSign))
		}
		fwd2 = rl.Vector3Normalize(fwd2)
		tip := rl.Vector3Add(joint1, rl.Vector3Scale(fwd2, fc.len2))

		// Draw knuckle joint, bone 1, middle joint, bone 2, fingertip
		rl.DrawSphere(knuckle, fRad*1.3, jointCol)
		rl.DrawCylinderEx(knuckle, joint1, fRad, fRad, 6, handCol)
		rl.DrawSphere(joint1, fRad*1.1, jointCol)
		rl.DrawCylinderEx(joint1, tip, fRad, fRad*0.8, 6, handCol)
		rl.DrawSphere(tip, fRad*0.9, jointCol)
	}
}
