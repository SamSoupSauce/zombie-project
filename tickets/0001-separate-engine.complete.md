# Extract Core Engine Architecture to project-engine

## Status
Complete

## Objective
Extract the working custom 3D engine loop, pipeline-driven `Renderable` component, Lua bridge/console, and tile-to-world importer out of the Zombie Project codebase and establish them in a dedicated standalone repository located at `../project-engine` targeting `git@github.com:SamSoupSauce/project-engine.git`.

---

## Clarification Log
- *Requirements are complete and clear; no clarifying questions needed.*

---

## Target Repository & Setup
* **Target Path:** `../project-engine`
* **Remote Origin:** `git@github.com:SamSoupSauce/project-engine.git`
* **Go Module:** Initialize `go.mod` (e.g., `module github.com/SamSoupSauce/project-engine`).
* **License:** MIT License.

---

## Architectural Requirements

### 1. Core Engine Package (`../project-engine/engine/`)
* **Spatial & Entity Foundation:**
  * Define `GameObject` (or `Entity`) containing strictly spatial identity (`Transform` with position, rotation, scale), hierarchy pointers (`Parent *GameObject`, `Children []*GameObject`), and an active status flag.
  * Define a clean, minimal `Component` interface (`Init`, `Update`, `Destroy`).
  * Ensure `engine` has zero dependency on gameplay logic or concrete physics backends (wrap Jolt hooks behind abstract handles/interfaces).

* **Pipeline-Driven Renderable Component:**
  * Implement unified `Renderable` struct:
    * `MeshID` (GPU buffer handle)
    * `MaterialID` (Shader/pipeline state handle)
    * `Flags` (`RenderFlags` bitmask: `FlagVisible`, `FlagCastShadow`, `FlagWireframe`, `FlagOverrideParent`, etc.)
  * Implement hierarchical cascade evaluation during scene-graph traversal so child nodes inherit parent visibility and render modes unless an explicit override flag is toggled.
  * Extract draw commands into a flat, allocation-free slice during the camera pass rather than invoking draw logic directly from entity components.

### 2. Modular Subpackages (`../project-engine/pkg/`)
* **`pkg/luabridge`:**
  * Decouple the interactive Lua console and scripting runtime bridge.
  * Expose an interface for `RegisterCommand`, `Execute`, and input-hook interception.
  * Reference core `engine.GameObject` pointers cleanly without circular dependencies.
* **`pkg/tileimporter`:**
  * Extract the 2D grid/tile map parser and 3D world mesh/collider instantiator.
  * Ensure the package can run headless tests to verify coordinate transformations and contiguous mesh/collision stitching.

### 3. Zombie Project Consumer Integration
* Update Zombie Project's `go.mod` to consume `project-engine` (initially via a local `replace` directive pointing to `../project-engine`).
* Swap internal imports across Zombie Project to reference the new external engine packages.
* Verify Zombie Project compiles and boots identically with zero gameplay regressions.

---

## Acceptance Criteria
- [x] `../project-engine` is initialized as a valid Git repo with remote set to `git@github.com:SamSoupSauce/project-engine.git`.
- [x] `project-engine` compiles cleanly with zero circular imports (`import cycle not allowed`).
- [x] No game-specific logic (e.g., zombie behavior, weapon stats) exists in `project-engine`.
- [x] Headless unit tests pass for `pkg/tileimporter` verifying map-to-world coordinate transforms.
- [x] Lua bridge initializes, registers basic engine debug commands, and processes input strings independently.
- [x] Extraction passes produce flat `DrawCommand` buffers from active `GameObject` hierarchies with functional bitmask overrides.
- [x] Zombie Project successfully builds and runs against the extracted `project-engine` module.

---

## Resolution Log
- Initialized and cloned `git@github.com:SamSoupSauce/project-engine.git` at `C:\Users\samue\Projects\project-engine`.
- Implemented core `engine` package:
  - `Transform` and `GameObject` spatial hierarchy with recursive matrix compounding and thread-safe component management.
  - Pipeline-driven `Renderable` component with `RenderFlags` (`FlagVisible`, `FlagCastShadow`, `FlagWireframe`, `FlagOverrideParent`), cascade inheritance evaluation, and flat draw-command collection.
  - Abstract physics handles decoupling concrete physics engines from core primitives.
  - Core `Engine` game loop with `Scene` interface, window lifecycle, frame timing, and double-pass rendering.
  - Headless unit tests in `engine_test.go` covering hierarchy, lifecycle, and cascade evaluation.
- Implemented `pkg/luabridge`:
  - Decoupled `LuaBridge` with dual-mutex architecture (`stateMu` for Lua state thread isolation and `dataMu` for command/object registries) preventing deadlock during callback queries.
  - Interactive developer `Console` with command history, line buffering, and log output.
  - Comprehensive unit tests in `bridge_test.go`.
- Implemented `pkg/tileimporter`:
  - TMX XML map parser supporting CSV data and flip/rotation flags.
  - `WorldMap3D` coordinate projection, contiguous wall/stair stitching, and top/bottom collision meshes for corner floor tiles.
  - Comprehensive headless unit tests in `importer_test.go`.
- Pushed standalone engine to `git@github.com:SamSoupSauce/project-engine.git` (`f7e652d`).
- Integrated `project-engine` into `zombie-project` via `go.mod` replace directive and updated `internal/engine/engine.go` to delegate to `github.com/SamSoupSauce/project-engine/engine`.
- Verified all un-cached unit tests across `zombie-project` pass with 100% success rate, and verified `zombie-game.exe` builds cleanly.

---

### Completion Summary
- **Ticket:** 0001-separate-engine
- **Branch:** 0001-separate-engine
- **Summary of Changes:**
  - Extracted engine core primitives, renderable pipeline, lua bridge, and tile importer into standalone repository `project-engine` (`github.com/SamSoupSauce/project-engine`).
  - Added full test suites for `engine`, `pkg/luabridge`, and `pkg/tileimporter`.
  - Pushed `project-engine` to remote `git@github.com:SamSoupSauce/project-engine.git` on `main`.
  - Integrated `project-engine` into `zombie-project` via `go.mod` replace directive and updated `internal/engine`.
  - Verified 100% test pass rate across both projects and confirmed binary compilation.
- **Files Modified / Added:**
  - `project-engine`:
    - [README.md](file:///C:/Users/samue/Projects/project-engine/README.md)
    - [go.mod](file:///C:/Users/samue/Projects/project-engine/go.mod)
    - [engine/component.go](file:///C:/Users/samue/Projects/project-engine/engine/component.go)
    - [engine/transform.go](file:///C:/Users/samue/Projects/project-engine/engine/transform.go)
    - [engine/game_object.go](file:///C:/Users/samue/Projects/project-engine/engine/game_object.go)
    - [engine/renderable.go](file:///C:/Users/samue/Projects/project-engine/engine/renderable.go)
    - [engine/physics_handle.go](file:///C:/Users/samue/Projects/project-engine/engine/physics_handle.go)
    - [engine/engine.go](file:///C:/Users/samue/Projects/project-engine/engine/engine.go)
    - [engine/engine_test.go](file:///C:/Users/samue/Projects/project-engine/engine/engine_test.go)
    - [pkg/luabridge/bridge.go](file:///C:/Users/samue/Projects/project-engine/pkg/luabridge/bridge.go)
    - [pkg/luabridge/console.go](file:///C:/Users/samue/Projects/project-engine/pkg/luabridge/console.go)
    - [pkg/luabridge/bridge_test.go](file:///C:/Users/samue/Projects/project-engine/pkg/luabridge/bridge_test.go)
    - [pkg/tileimporter/types.go](file:///C:/Users/samue/Projects/project-engine/pkg/tileimporter/types.go)
    - [pkg/tileimporter/tmx.go](file:///C:/Users/samue/Projects/project-engine/pkg/tileimporter/tmx.go)
    - [pkg/tileimporter/world_map3d.go](file:///C:/Users/samue/Projects/project-engine/pkg/tileimporter/world_map3d.go)
    - [pkg/tileimporter/importer_test.go](file:///C:/Users/samue/Projects/project-engine/pkg/tileimporter/importer_test.go)
  - `zombie-project`:
    - [go.mod](file:///C:/Users/samue/Projects/zombie-project/go.mod)
    - [internal/engine/engine.go](file:///C:/Users/samue/Projects/zombie-project/internal/engine/engine.go)
    - [tickets/0001-separate-engine.complete.md](file:///C:/Users/samue/Projects/zombie-project/tickets/0001-separate-engine.complete.md)
- **Verification Results:**
  - `project-engine`: `go test -v ./...` -> 100% pass across `engine`, `pkg/luabridge`, `pkg/tileimporter`
  - `zombie-project`: `go test -count=1 ./...` -> 100% pass across all packages
  - `zombie-project`: `go build -o zombie-game.exe ./cmd/game` -> Success (exit code 0)
