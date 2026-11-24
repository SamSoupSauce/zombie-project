# 🧟 Zombie 3D Game Engine (Go + Lua + Tiled TMX + First Person)

A modular, lightweight 3D game engine written in **Golang** using [Raylib-Go](https://github.com/gen2brain/raylib-go) featuring a full **First-Person Controller (FPS)**, **Lua 5.1 Scripting** via [GopherLua](https://github.com/yuin/gopher-lua), an **Interactive In-Game Developer Console (REPL)**, and a **Tiled Map Editor (.tmx) 3D Level Importer**.

Runs seamlessly on Windows with **zero C compiler / CGO requirements** via `purego`.

---

## 🏃 First-Person Perspective & Locomotion

The game runs in **First-Person View (FPV)** by default:
- **Universal 'Human' Base Model as Parent:** The `FPSController` is parented to a procedural 3D `Human` stick-figure model. Player position, yaw, pitch, velocity, grounded state, and camera eye level are directly synchronized with the parent human object.
- **Visible First-Person Hands & 5 Articulated Fingers:** In first person view, the player's hands and 5 distinct articulating fingers (Thumb, Index, Middle, Ring, Pinky) are rendered with smooth walking sway, idle breathing, and dynamic grasping poses.
- **Full WASD Movement:** Smooth ground-plane movement (Walk: 4.8 m/s).
- **Sprint Mechanic:** Hold **`Left Shift`** to sprint (8.2 m/s).
- **Jump & Gravity:** Press **`Space`** to leap into the air with realistic downward gravity and ground collision.
- **Fluid Mouse Look & Window Capture:** Mouse cursor is captured, hidden (`rl.DisableCursor()`, `rl.HideCursor()`), and physically confined to the window's client bounds using native Windows cursor trapping (`ClipCursor`). The mouse pointer cannot drift or escape to other monitors during gameplay, while automatically releasing when opening the console (` ` `) or switching focus.
- **Head Bobbing:** Sinusoidal camera bobbing during walking and sprinting for realistic movement feedback.
- **3D Wall Collision:** Prevents walking through TMX map stone walls (GID 2), crates (GID 3), and pillars (GID 4) with smooth sliding collision.
- **First-Person Reticle / Crosshair:** Precision center-screen dot with dynamic hover detection and object target prompts.
- **Perspective Toggle:** Press **`[V]`** anytime to switch between First-Person View and Orbit / Third-Person View. In Third-Person view, you can see your complete 3D stick-figure human model (head, face with eyes and smile, spine, shoulders, arms, hands with individual fingers, legs, and flat feet).

---

## 📦 Prop Pickup System (< 50 kg Limit), Throwing & Gravity

Any 3D prop entity implementing `prop.Prop` has physical mass in kilograms (kg) and full rigid body gravity simulation:
- **Pickup Key `[E]`:** Press **`[E]`** while looking at an object within reach ($\le 3.5\text{m}$) to pick it up.
- **Strict $< 50\text{ kg}$ Lifting Rule:**
  - If **$\text{mass} < 50.0\text{ kg}$**: Player picks up the prop! The prop is carried in the player's hands, with hands automatically adjusting to grip both sides of the prop.
  - If **$\text{mass} \ge 50.0\text{ kg}$**: The prop is too heavy to lift! A warning banner alerts the player (`CANNOT LIFT: 160.0 kg is too heavy! Limit is < 50.0 kg`).
- **Throw Prop `[Left Click / LMB]` or `[G]`:** When carrying a prop, pressing **`[LMB]`** or **`[G]`** hurls the prop forward with high velocity ($19.0\text{ m/s}$ along look ray $+ 5.2\text{ m/s}$ upward arc $+$ player velocity contribution) with realistic mid-air tumbling and full gravitational arc trajectory. Hands visibly thrust forward with an impulse animation!
- **Drop / Gentle Toss `[Right Click / RMB]` or `[E]`:** When carrying a prop, pressing **`[RMB]`** or **`[E]`** drops the prop gently to the ground in front of the player.
- **Dynamic Gravity & Tumbling:** Thrown cubes curve downward with gravity ($-18.0\text{ m/s}^2$), tumble with angular momentum, bounce with restitution off floors and walls, roll with surface friction, and transfer momentum when colliding with other cubes!
- **Included Demo Props:**
  - `MiniCube` (12.0 kg) — Liftable (< 50kg)
  - `CompanionCube` (24.0 kg) — Liftable (< 50kg)
  - `RainbowCube` (35.0 kg) — Liftable (< 50kg)
  - `HeavyCrate` (160.0 kg) — Too heavy to lift ($\ge 50$kg, acts as solid obstacle)

---

## ⚡ 3D Rigid Body Physics & Collision Boxes (`internal/physics`)

All cubes and props are powered by a lightweight 3D rigid body physics engine with true Axis-Aligned Bounding Box (AABB) collision detection and sub-stepped numerical integration:

- **Rigid Body Properties:**
  - `Position`, `Velocity` (linear velocity in 3D space), `AngularVelocity` (rolling / tumbling rate).
  - `Mass` (kg) and Inverse Mass (`1/mass` for dynamic, `0` for static).
  - `Restitution` (bounciness factor $[0.0, 1.0]$, default `0.35`).
  - `Friction` (Coulomb surface friction coefficient $[0.0, 1.0]$, default `0.75`).
  - `BodyDynamic`, `BodyKinematic` (script-controlled motion like hover), and `BodyStatic`.
- **AABB 3D Collision Boxes & Resolution:**
  - Real-time world-space bounding box calculations (`c.GetBoundingBox()`).
  - Cube-to-Cube collisions: Calculates penetration depth along minimum overlap axis, resolves positional interpenetration, and applies elastic momentum impulse exchange.
  - Floor & Ground plane collision at $Y = -2.2$: Bounces with restitution, slides with ground friction, and settles cleanly into resting state.
  - Solid Obstacle collision: Cubes bounce and slide off solid TMX map stone walls (GID 2) and barricades (GID 3).
- **Player & Cube Physical Interaction:**
  - Walking into light dynamic cubes ($< 50\text{kg}$) pushes them forward with physical impulse.
  - Heavy or static cubes ($\ge 50\text{kg}$) act as solid obstacles that block player movement.
- **Visual Wireframe Debugging:**
  - Press **`[B]`** or run `engine.toggle_physics_debug()` to toggle visual cyan/green collision box wireframes around all physics entities in 3D space.

---

## 🧍 Universal Stick-Figure Human Base Model (`internal/human`)

The procedural 3D stick figure includes full anatomical features:
- **Head & Face:** 3D spherical skull with two eyes (dark obsidian pupils with sparkling specular highlights) and a cheerful curved smile arc.
- **Spine & Torso:** Vertical spine capsule connecting horizontal shoulder and pelvic bars with spherical joint nodes.
- **Articulated Arms & Hands:** Upper arms and forearms with elbow joints leading to palms and **5 individual articulated fingers** (Thumb, Index, Middle, Ring, Pinky).
- **Legs & Flat Feet:** Thighs and shins with knee joints and flat feet positioned firmly on the ground.
- **Dual-View Rendering:**
  - **First-Person View (`DrawFirstPersonHands`):** Displays forearms, wrists, palms, and all 5 fingers swaying naturally in the viewport, wrapping around props when carried.
  - **Third-Person View (`DrawThirdPerson`):** Displays full body walking/running animations, head pitch tracking, and prop carrying.

---

## 🗺️ Tiled Map Editor (.tmx) 3D Level Importer

The engine includes a native XML parser and 3D level generator for maps created in the [Tiled Map Editor](https://www.mapeditor.org/) (`.tmx` format).

- **Automatic 3D Level Construction:** 
  - Converts 2D tile layers into true 3D architecture centered in the world.
  - **Tile GID 1 (Asphalt/Pavement):** Ground floor slab.
  - **Tile GID 2 (Stone Wall):** Solid elevated 3D stone/brick wall (2.0 units high) with wireframe edges.
  - **Tile GID 3 (Barricade):** Solid wooden/metal crate obstacle (0.9 units high).
  - **Tile GID 4 (Steel Watchtower):** Tall industrial column (3.4 units high).
  - **Tile GID 5 (Grass/Soil):** Natural terrain slab.
  - **Tile GID 6 (Toxic Pool):** Semi-transparent liquid hazard.
- **Included Sample Maps:**
  - [maps/zombie_outpost.tmx](maps/zombie_outpost.tmx): 14x14 fortified perimeter outpost with courtyards, watchtowers, barricade chokepoints, player spawn, and zombie spawners.
  - [maps/city_ruins.tmx](maps/city_ruins.tmx): 12x12 urban street block with ruined buildings, roads, and rubble.

---

## 💻 In-Game Developer Console (**`` ` ``**)

Press **`` ` ``** (the backtick/tilde key directly under ESC) to drop down the console.
- **Mouse Unlocked:** Opening the console unlocks and shows the mouse cursor so you can freely type or click on 3D objects.
- **3D Object Picking:** Left-clicking any 3D object sets the global variable **`target`** to that object!
- **Closing Console:** Press **`` ` ``** or **`ESC`** to close the console and instantly re-lock the mouse cursor for first-person gameplay.

```lua
-- First-Person Player Lua APIs:
player.get_position()       --> returns x, y, z
player.teleport(0, 5, 0)    --> drops player from height 5
player.set_speed(12.0)      --> increases player movement speed
player.jump()               --> triggers a jump in-game
player.get_held_prop()      --> returns table with id, name, mass (or nil)
player.drop_prop()          --> drops currently held prop

-- Prop mass & lifting checks:
target.get_mass()               --> returns mass in kg
target.set_mass(42.0)           --> changes mass dynamically!
target.can_pickup()             --> returns true if mass < 50.0 kg

-- 3D Physics & Collision Box APIs:
vx, vy, vz = target.get_velocity()
target.set_velocity(0, 10, 0)   --> launches object upwards!
target.apply_impulse(50, 0, 0)  --> applies impulse in Newton-seconds
target.get_bounciness()         --> restitution factor [0.0 - 1.0]
target.set_bounciness(0.85)     --> makes cube super bouncy!
target.get_friction()           --> friction coefficient
target.set_friction(0.2)        --> low friction ice-slide!
target.is_grounded()            --> returns true if resting on floor
target.is_static()              --> returns true if immovable static obstacle
target.set_static(true)         --> anchors object permanently
target.toggle_collision_box()   --> toggles visual AABB bounding box
box = target.get_collision_box()--> returns {min = {x,y,z}, max = {x,y,z}}
engine.toggle_physics_debug()   --> toggles all collision boxes in scene

-- Modify targeted objects:
target.set_size(4.0)
target.set_mode("chroma")
target.set_position(0, 2, 0)
print(target.inspect())

-- Switch maps live:
map.load("maps/city_ruins.tmx")
```

---

## 🚀 Getting Started

### Run the Game
```powershell
go run .
# Or run the built executable:
.\zombie-game.exe
```

### Build Release Binary
```powershell
go build -ldflags "-s -w -H=windowsgui" -o zombie-game.exe .
```

---

## 🎮 Controls Summary

| Input | Action |
|---|---|
| **Mouse** | **Look around (First-Person)** |
| **`W` / `A` / `S` / `D`** | **Walk / Move Player** |
| **`Left Shift`** | **Sprint (8.2 m/s)** |
| **`SPACE`** | **Jump / Leap** |
| **`[E]` / `[RMB]`** | **Pick Up prop (< 50kg) / Gentle drop held prop** |
| **`[LMB]` / `[G]`** | **Throw held prop with high velocity & gravity** *(or Toggle Grid if empty)* |
| **`[B]`** | **Toggle 3D Collision Boxes Wireframe Debug View** |
| **Left Click (LMB)** | **Throw held prop (when carrying) / Inspect target along crosshair** |
| **Right Click (RMB)**| **Gentle drop / toss held prop** |
| **`` ` ``** (Grave / Tilde) | **Toggle Developer Console** *(Unlocks mouse)* |
| **`[V]`** | **Toggle First Person / Orbit View** |
| **`[M]`** | **Toggle 3D TMX Map Level rendering** |
| **`[C]`** | **Reset Player to spawn position** |
| **`[F5]`** | **Hot-Reload Lua script** |
| **`[R]`** / **`[1, 2, 3]`** | Switch Rainbow render mode of target |
| **`[H]`** | Toggle Controls & Telemetry HUD |
| **`[F]`** | Toggle fullscreen |
| **`[ESC]`** | Open Console / Exit |

---

## 📁 Project Architecture

```
zombie-project/
├── cmd/game/main.go            # Entry point
├── internal/
│   ├── camera/                 # FPS controller, mouselook, collision, orbit camera & tests
│   ├── console/                # Interactive Lua REPL console & tests
│   ├── cube/                   # Rainbow cube geometry, OBB raycasting, physics delegate & render modes
│   ├── engine/                 # Engine lifecycle, MSAA, VSync
│   ├── human/                  # Universal stick-figure human model & tests
│   ├── physics/                # 3D Rigid body dynamics, AABB collision resolution & tests
│   ├── prop/                   # Universal prop interface
│   ├── scene/                  # Scene manager & 3D first-person level scene
│   ├── scripting/              # Lua 5.1 VM, hot-reload, player, physics, map & engine bindings
│   ├── tiled/                  # TMX XML parser, 3D level generator & tests
│   └── ui/                     # 2D HUD telemetry, crosshair & live Lua console
├── maps/
│   ├── zombie_outpost.tmx      # Fortified perimeter survival outpost
│   └── city_ruins.tmx          # Urban city block with streets & rubble
├── scripts/
│   └── cube_behavior.lua       # Active Lua script controlling the cube
├── main.go                     # Root runner
├── go.mod
├── README.md
└── zombie-game.exe             # Release binary
```
