# Second Floor Wall-Corner and Wall-Edge Floor/Ceiling Collision

## Status
Complete

## Summary
On the second floor of multi-story buildings (and any elevated layer at elevation >= 2.2m), players are falling through the floor/ceiling when walking near perimeter walls. Currently, bottom floor slab and top ceiling slab collision boxes are only generated for wall tiles if `t.IsCorner() && t.HasFlooring`. Straight wall-edge floor tiles (`t.WallCount() == 1` or non-perpendicular wall edges) do not generate bottom or top collision slabs. Furthermore, wall-corner tiles whose metadata or layer inheritance does not trigger `IsCorner()` also lack floor collision. Because wall segments only occupy a 0.2m boundary along the tile edge, the interior 0.8m walkable floor area on these tiles lacks a solid bounding box, causing the player to fall through to the story below.

## Architectural Requirements

### 1. Universal Floor & Ceiling Collision for Wall Tiles with Flooring
- Any tile that contains wall segments (`len(t.WallSegments) > 0` or `t.Walls != 0`) and has flooring (`t.HasFlooring == true`) must generate:
  - **Bottom Collision Mesh (`BottomCollisionBox`)**: A solid floor slab spanning the entire tile footprint `[worldX - halfW, worldX + halfW]` x `[worldZ - halfD, worldZ + halfD]` with thickness `FloorHeight` (from `floorTopY - FloorHeight` to `floorTopY`).
  - **Top Collision Mesh (`TopCollisionBox`)**: A solid ceiling / roof cap slab spanning the tile footprint from `wallTopY - FloorHeight` to `wallTopY`.
- This must apply to both:
  - **Wall-Corner Tiles** (NW, NE, SW, SE corner walls or explicitly tagged corner tiles).
  - **Wall-Edge Tiles** (single-edge straight walls along North, South, East, or West edges, T-junctions, and end caps).

### 2. Multi-Story & 2nd Floor Layer Elevation
- Ensure that for any elevated layer (such as the second floor with `Elevation = 2.2m`), `BottomCollisionBox` and `TopCollisionBox` are properly elevated by `layerElevation`:
  - `tile.BottomCollisionBox.Min.Y += layerElevation`
  - `tile.BottomCollisionBox.Max.Y += layerElevation`
  - `tile.TopCollisionBox.Min.Y += layerElevation`
  - `tile.TopCollisionBox.Max.Y += layerElevation`
- When elevated wall tiles inherit flooring from underlying tiles or layer templates (`tile.HasFlooring = true`), ensure collision bounding boxes are regenerated or rebuilt with the correct elevated coordinates.

### 3. Collision Query & Physics Integration
- In `GetSolidBoxes()` (and any spatial partitioning / bounding box query methods), ensure `BottomCollisionBox` and `TopCollisionBox` are returned for all wall tiles that have `HasBottomCollision` and `HasTopCollision`.
- Ensure raycasts and swept AABB queries from above (player walking on 2nd floor) hit the bottom collision box (`floorTopY`), and queries from below (player jumping on 1st floor) hit the ceiling collision box (`wallTopY`).

---

## Verification & Acceptance Criteria
- [x] Straight wall-edge tiles with flooring have `HasBottomCollision = true` and `HasTopCollision = true` with full tile footprint dimensions.
- [x] Corner wall tiles with flooring have `HasBottomCollision = true` and `HasTopCollision = true`.
- [x] Second floor (`Elevation >= 2.2m`) wall-edge and wall-corner tiles have their floor collision boxes positioned at `Y = 2.2m` (preventing player fall-through).
- [x] `GetSolidBoxes()` includes the floor and ceiling collision bounding boxes for all wall-edge and wall-corner tiles.
- [x] Automated headless unit tests verify bottom and top collision boxes for 1st floor and 2nd floor wall-edge and wall-corner tiles.
- [x] Player walking on the 2nd floor near walls and corners remains supported by the floor and does not fall through the ceiling/floor.

---
### Clarification Log
- *Requirements are complete and clear; no clarifying questions needed.*

---
### Completion Summary
- **Ticket:** 0002-second-floor-wall-tiles-collision
- **Branch:** 0002-second-floor-wall-tiles-collision
- **Summary of Changes:**
  - Updated `Tile3D.rebuildWallSegments` in `internal/tiled/map3d.go` so that all wall tiles with `HasFlooring == true` (both wall-corner tiles and straight wall-edge / partition wall tiles, plus elevated `Walls == 0` tiles with flooring) initialize default `FloorSize`/`FloorPosition` if unset and generate full-tile `BottomCollisionBox` and `TopCollisionBox` slabs (`HasBottomCollision = true`, `HasTopCollision = true`).
  - Updated `Map3D.resolveTileWalls` in `internal/tiled/map3d.go` to guard `FloorPosition.Y` elevation on multi-story layers before rebuilding wall segments and collision boxes.
  - Updated `FPSController.MoveWithCollisions` (`hasHeadroomAt`, `canLandAtStep`) and `resolveWallCollisionsAt` in `internal/camera/fps.go` so overhead ceiling slabs (`TopCollisionBox` at `b.Min.Y > feetY+stepH`) do not falsely block horizontal movement or ground step-ups (e.g., walking up stairs beneath a 1st-floor ceiling) while still rejecting unsteppable ledges and low-headroom ceilings.
  - Added `TestSecondFloorWallEdgesAndCornersCollision` in `internal/tiled/tmx_test.go` (verifying all 2nd-floor perimeter corners, N/S/E/W wall edges, and interior partition wall tiles) and updated `TestWallsFlooringAndThickness` and `TestCornerTilesHaveTopAndBottomCollision`.
  - Added `TestSecondFloorPerimeterWalkNoFallThrough` in `internal/scene/stair_climb_test.go` simulating full perimeter traversal along 2nd-floor wall edges and corners.
- **Files Modified / Added:**
  - [internal/tiled/map3d.go](file:///C:/Users/samue/Projects/zombie-project/internal/tiled/map3d.go)
  - [internal/camera/fps.go](file:///C:/Users/samue/Projects/zombie-project/internal/camera/fps.go)
  - [internal/tiled/tmx_test.go](file:///C:/Users/samue/Projects/zombie-project/internal/tiled/tmx_test.go)
  - [internal/scene/stair_climb_test.go](file:///C:/Users/samue/Projects/zombie-project/internal/scene/stair_climb_test.go)
  - [tickets/0002-second-floor-wall-tiles-collision.complete.md](file:///C:/Users/samue/Projects/zombie-project/tickets/0002-second-floor-wall-tiles-collision.complete.md)
- **Verification Results:**
  - `go test -v -count=1 ./internal/camera/... ./internal/console/... ./internal/cube/... ./internal/human/... ./internal/paths/... ./internal/physics/... ./internal/prop/... ./internal/scene/... ./internal/scripting/... ./internal/tiled/... ./internal/ui/...`: All tests passed (`camera`, `console`, `cube`, `human`, `paths`, `physics`, `scene`, `scripting`, `tiled`; `prop` and `ui` have no test files).
  - Note: `./internal/engine`, `./cmd/game`, and root package did not compile against local `../project-engine` while `../project-engine` has branch `0005-luajit-zero-alloc-standards` checked out (API change from Raylib `rl.Camera3D` to `engine.Camera3D`).
