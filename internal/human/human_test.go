package human

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/prop"
)

// mockProp implements prop.Prop for testing pickup logic.
type mockProp struct {
	id       string
	name     string
	position rl.Vector3
	rotation rl.Vector3
	size     float32
	mass     float32
	held     bool
}

func (m *mockProp) GetID() string                         { return m.id }
func (m *mockProp) GetName() string                       { return m.name }
func (m *mockProp) GetPosition() rl.Vector3               { return m.position }
func (m *mockProp) SetPosition(pos rl.Vector3)            { m.position = pos }
func (m *mockProp) GetRotation() rl.Vector3               { return m.rotation }
func (m *mockProp) SetRotation(rot rl.Vector3)            { m.rotation = rot }
func (m *mockProp) GetSize() float32                      { return m.size }
func (m *mockProp) GetMass() float32                      { return m.mass }
func (m *mockProp) SetMass(mass float32)                  { m.mass = mass }
func (m *mockProp) IsHeld() bool                          { return m.held }
func (m *mockProp) SetHeld(held bool)                     { m.held = held }
func (m *mockProp) RayIntersects(ray rl.Ray) (bool, float32) { return false, 0 }
func (m *mockProp) Draw()                                 {}
func (m *mockProp) Update(dt float32)                     {}

var _ prop.Prop = (*mockProp)(nil)

func TestNewHumanDefaults(t *testing.T) {
	spawn := rl.NewVector3(10.0, -2.0, 5.0)
	h := NewHuman(spawn)

	if h.Position != spawn {
		t.Errorf("expected position %v, got %v", spawn, h.Position)
	}
	if h.EyeHeight != 1.65 {
		t.Errorf("expected EyeHeight 1.65, got %f", h.EyeHeight)
	}

	eyePos := h.GetEyePosition(0.0)
	expectedEyeY := float32(-2.0 + 1.65)
	if math.Abs(float64(eyePos.Y-expectedEyeY)) > 0.001 {
		t.Errorf("expected eye Y %f, got %f", expectedEyeY, eyePos.Y)
	}

	eyePosBob := h.GetEyePosition(0.05)
	if math.Abs(float64(eyePosBob.Y-(expectedEyeY+0.05))) > 0.001 {
		t.Errorf("expected eye Y with bob %f, got %f", expectedEyeY+0.05, eyePosBob.Y)
	}
}

func TestForwardAndRightVectors(t *testing.T) {
	h := NewHuman(rl.NewVector3(0, 0, 0))
	h.Yaw = 0.0 // Facing +Z

	fwd := h.GetForwardVector()
	if math.Abs(float64(fwd.X)) > 0.001 || math.Abs(float64(fwd.Z-1.0)) > 0.001 {
		t.Errorf("at Yaw=0 expected fwd (0,0,1), got %v", fwd)
	}

	right := h.GetRightVector()
	if math.Abs(float64(right.X+1.0)) > 0.001 || math.Abs(float64(right.Z)) > 0.001 {
		t.Errorf("at Yaw=0 expected right (-1,0,0), got %v", right)
	}

	h.Yaw = float32(math.Pi / 2.0) // Turned 90 degrees right
	fwd90 := h.GetForwardVector()
	if math.Abs(float64(fwd90.X+1.0)) > 0.001 || math.Abs(float64(fwd90.Z)) > 0.001 {
		t.Errorf("at Yaw=PI/2 expected fwd (-1,0,0), got %v", fwd90)
	}

	right90 := h.GetRightVector()
	if math.Abs(float64(right90.X)) > 0.001 || math.Abs(float64(right90.Z+1.0)) > 0.001 {
		t.Errorf("at Yaw=PI/2 expected right (0,0,-1), got %v", right90)
	}
}

func TestCanPickUpMassLimit(t *testing.T) {
	h := NewHuman(rl.NewVector3(0, 0, 0))

	// Nil prop test
	canNil, _ := h.CanPickUp(nil)
	if canNil {
		t.Errorf("expected nil prop to not be pickable")
	}

	// Light props (< 50kg)
	pLight1 := &mockProp{name: "Feather", mass: 0.5}
	can1, _ := h.CanPickUp(pLight1)
	if !can1 {
		t.Errorf("expected 0.5kg prop to be pickable (< 50kg)")
	}

	pLight2 := &mockProp{name: "CompanionCube", mass: 28.0}
	can2, _ := h.CanPickUp(pLight2)
	if !can2 {
		t.Errorf("expected 28.0kg prop to be pickable (< 50kg)")
	}

	pBoundaryUnder := &mockProp{name: "UnderBoundary", mass: 49.99}
	canUnder, _ := h.CanPickUp(pBoundaryUnder)
	if !canUnder {
		t.Errorf("expected 49.99kg prop to be pickable (< 50kg)")
	}

	// Exact 50kg boundary (strictly < 50kg requirement, so 50.0kg cannot be lifted)
	pExact50 := &mockProp{name: "FiftyKgWeight", mass: 50.0}
	canExact, reasonExact := h.CanPickUp(pExact50)
	if canExact {
		t.Errorf("expected exactly 50.0kg prop to NOT be pickable (requires < 50kg)")
	}
	if reasonExact == "" {
		t.Errorf("expected descriptive rejection reason for 50.0kg")
	}

	// Heavy props (>= 50kg)
	pHeavy := &mockProp{name: "HeavyCrate", mass: 160.0}
	canHeavy, reasonHeavy := h.CanPickUp(pHeavy)
	if canHeavy {
		t.Errorf("expected 160.0kg prop to NOT be pickable")
	}
	if reasonHeavy == "" {
		t.Errorf("expected descriptive rejection reason for heavy prop")
	}
}

func TestPickUpAndDropProp(t *testing.T) {
	h := NewHuman(rl.NewVector3(0, 0, 0))

	crate := &mockProp{id: "crate_1", name: "Crate", mass: 35.0}
	heavyAnvil := &mockProp{id: "anvil_1", name: "Anvil", mass: 120.0}

	// 1. Trying to pick up heavy anvil fails
	ok, _ := h.PickUp(heavyAnvil)
	if ok {
		t.Errorf("picking up 120kg anvil should have failed")
	}
	if h.HeldProp != nil {
		t.Errorf("HeldProp should remain nil after failed pickup")
	}
	if heavyAnvil.IsHeld() {
		t.Errorf("anvil IsHeld() should be false")
	}

	// 2. Picking up crate (< 50kg) succeeds
	ok, msg := h.PickUp(crate)
	if !ok {
		t.Fatalf("picking up 35kg crate should succeed: %s", msg)
	}
	if h.HeldProp != crate {
		t.Errorf("HeldProp should be set to crate")
	}
	if !crate.IsHeld() {
		t.Errorf("crate IsHeld() should now be true")
	}

	// 3. Picking up another prop while hands full fails
	secondCube := &mockProp{id: "cube_2", name: "MiniCube", mass: 10.0}
	ok2, _ := h.PickUp(secondCube)
	if ok2 {
		t.Errorf("picking up second prop should fail when already carrying a prop")
	}

	// 4. Dropping prop
	dropped := h.DropProp()
	if dropped != crate {
		t.Errorf("expected dropped prop to be crate")
	}
	if h.HeldProp != nil {
		t.Errorf("HeldProp should be nil after drop")
	}
	if crate.IsHeld() {
		t.Errorf("crate IsHeld() should be false after drop")
	}

	// 5. Dropping again returns nil
	droppedAgain := h.DropProp()
	if droppedAgain != nil {
		t.Errorf("dropping with empty hands should return nil")
	}
}

func TestHumanUpdateAnimation(t *testing.T) {
	h := NewHuman(rl.NewVector3(0, 0, 0))

	// Stationary update
	h.Update(0.1)
	if h.IdleTimer < 0.1 {
		t.Errorf("expected IdleTimer to advance, got %f", h.IdleTimer)
	}
	if h.WalkCycle != 0 {
		t.Errorf("WalkCycle should remain 0 while stationary")
	}

	// Moving update
	h.IsMoving = true
	h.IsGrounded = true
	h.WalkSpeed = 4.8
	h.Update(0.1)
	if h.WalkCycle <= 0 {
		t.Errorf("expected WalkCycle to advance while moving")
	}
}

func TestHumanTriggerThrow(t *testing.T) {
	h := NewHuman(rl.NewVector3(0, 0, 0))

	h.TriggerThrow()
	if h.ThrowTimer != 0.22 {
		t.Errorf("expected ThrowTimer 0.22 after TriggerThrow, got %f", h.ThrowTimer)
	}

	h.Update(0.1)
	if math.Abs(float64(h.ThrowTimer-0.12)) > 0.001 {
		t.Errorf("expected ThrowTimer ~0.12 after 0.1s dt, got %f", h.ThrowTimer)
	}

	h.Update(0.2)
	if h.ThrowTimer != 0 {
		t.Errorf("expected ThrowTimer to reach 0, got %f", h.ThrowTimer)
	}
}
