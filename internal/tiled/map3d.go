package tiled

import (
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/cube"
)

// TileType defines how a TMX tile is interpreted in 3D space.
type TileType int

const (
	TileEmpty TileType = iota
	TileFloor
	TileWall
	TileBarricade
	TilePillar
	TileGrass
	TileWater
	TileWallInterior
	TileWallExterior
	TileStairs
)

// StairDirection defines which cardinal direction a staircase ascends toward (going up).
type StairDirection int

const (
	StairAscendNorth StairDirection = iota // Goes up toward North (-Z)
	StairAscendSouth                       // Goes up toward South (+Z)
	StairAscendEast                        // Goes up toward East (+X)
	StairAscendWest                        // Goes up toward West (-X)
)

// StairStep represents an individual step tread/riser on a multi-step staircase.
type StairStep struct {
	Index       int
	Position    rl.Vector3
	Size        rl.Vector3
	BoundingBox rl.BoundingBox
}

// Architectural constants for walls and flooring
const (
	// WallThickness defines the architectural thickness of a wall segment (~0.2m).
	WallThickness float32 = 0.2
	// FloorHeight defines the standard vertical thickness of underlying ground flooring (~0.08m).
	FloorHeight float32 = 0.08
	// DefaultWallHeight defines the vertical height of walls (2.2m).
	DefaultWallHeight float32 = 2.2
)

// Wall edge bitmasks representing which edge(s) of a square tile have walls (0 to 4 walls per tile).
const (
	WallEdgeNone  uint8 = 0
	WallEdgeNorth uint8 = 1 << 0 // -Z edge (facing North)
	WallEdgeSouth uint8 = 1 << 1 // +Z edge (facing South)
	WallEdgeEast  uint8 = 1 << 2 // +X edge (facing East)
	WallEdgeWest  uint8 = 1 << 3 // -X edge (facing West)
)

// Face direction masks for wall texturing and exterior siding
const (
	FaceNorth uint8 = 1 << 0 // -Z direction (North / Back)
	FaceSouth uint8 = 1 << 1 // +Z direction (South / Front)
	FaceEast  uint8 = 1 << 2 // +X direction (East / Right)
	FaceWest  uint8 = 1 << 3 // -X direction (West / Left)
)

// WallSegment represents a single ~0.2m thick wall along one edge of a square tile.
type WallSegment struct {
	Edge          uint8      // WallEdgeNorth, WallEdgeSouth, WallEdgeEast, WallEdgeWest
	Type          TileType   // TileWallExterior, TileWallInterior, TileWall, etc.
	Position      rl.Vector3 // 3D center position of the 0.2m wall slab
	Size          rl.Vector3 // Dimensions: 0.2m thickness along normal, ts length along edge, 2.2m height
	Color         rl.Color
	WireColor     rl.Color
	IsSolid       bool
	IsExterior    bool
	IsInterior    bool
	ExteriorFaces uint8 // Bitmask of which vertical face(s) have exterior siding
}

// GetBoundingBox returns the exact 3D collision bounding box for this 0.2m wall segment.
func (ws *WallSegment) GetBoundingBox() rl.BoundingBox {
	halfW := ws.Size.X / 2.0
	halfH := ws.Size.Y / 2.0
	halfD := ws.Size.Z / 2.0
	return rl.NewBoundingBox(
		rl.NewVector3(ws.Position.X-halfW, ws.Position.Y-halfH, ws.Position.Z-halfD),
		rl.NewVector3(ws.Position.X+halfW, ws.Position.Y+halfH, ws.Position.Z+halfD),
	)
}

// RayIntersects checks if a ray intersects this 0.2m wall segment.
func (ws *WallSegment) RayIntersects(ray rl.Ray) (bool, float32) {
	col := rl.GetRayCollisionBox(ray, ws.GetBoundingBox())
	return col.Hit, col.Distance
}

// Tile3D represents a single tile instantiated in 3D world space.
type Tile3D struct {
	GID           uint32
	LayerName     string
	Col           int
	Row           int
	Position      rl.Vector3
	Size          rl.Vector3 // Width, Height, Depth
	Color         rl.Color
	WireColor     rl.Color
	IsSolid       bool
	Type          TileType
	Name          string
	IsExterior    bool  // Exterior wall with slanted siding on outside faces
	IsInterior    bool  // Interior wall with plain drywall on all faces
	ExteriorFaces uint8 // Bitmask of which vertical faces have exterior siding (FaceNorth/South/East/West)

	// Flooring requirement: Every wall tile is guaranteed to be on top of an underlying flooring tile
	HasFlooring    bool
	FloorGID       uint32
	FloorType      TileType
	FloorName      string
	FloorColor     rl.Color
	FloorWireColor rl.Color
	FloorPosition  rl.Vector3
	FloorSize      rl.Vector3

	// Wall edge configuration (0 to 4 walls per square tile, ~0.2m thickness each)
	Walls         uint8         // Bitmask of WallEdgeNorth, WallEdgeSouth, WallEdgeEast, WallEdgeWest
	WallsExplicit bool          // True if walls were explicitly assigned from TMX properties or script
	FlipFlags     uint32        // Tiled flip/rotation flags (bits 31, 30, 29)
	WallSegments  []WallSegment // Up to 4 active wall segments

	// Staircase configuration
	IsStairs    bool
	StairDir    StairDirection
	StepCount   int
	StairHeight float32
	StartHeight float32
	Steps       []StairStep
	StepBoxes   []rl.BoundingBox
}

// WallCount returns the number of walls on this square tile (0 to 4).
func (t *Tile3D) WallCount() int {
	return bits.OnesCount8(t.Walls)
}

// HasWall returns true if the specified wall edge is present.
func (t *Tile3D) HasWall(edge uint8) bool {
	return t.Walls&edge != 0
}

// AddWall adds a 0.2m thick wall segment along the specified edge.
func (t *Tile3D) AddWall(edge uint8, wallType TileType, isExterior bool, ts, baseY float32) {
	t.Walls |= edge
	t.rebuildWallSegments(ts, baseY)
}

// RemoveWall removes the wall along the specified edge.
func (t *Tile3D) RemoveWall(edge uint8, ts, baseY float32) {
	t.Walls &^= edge
	t.rebuildWallSegments(ts, baseY)
	if t.Walls == 0 {
		t.IsSolid = false
	}
}

// rebuildWallSegments updates the slice of 0.2m thick WallSegments based on t.Walls.
func (t *Tile3D) rebuildWallSegments(ts, baseY float32) {
	t.WallSegments = make([]WallSegment, 0, 4)
	if t.Walls == 0 {
		if t.Type == TileWallExterior || t.Type == TileWallInterior || t.Type == TileWall {
			t.IsSolid = false
		}
		return
	}
	t.IsSolid = true
	wallH := float32(2.2)
	floorTopY := baseY + FloorHeight
	centerY := floorTopY + wallH/2.0

	worldX := t.Position.X
	worldZ := t.Position.Z
	if t.HasFlooring {
		worldX = t.FloorPosition.X
		worldZ = t.FloorPosition.Z
	}

	edges := []uint8{WallEdgeNorth, WallEdgeSouth, WallEdgeEast, WallEdgeWest}
	for _, edge := range edges {
		if t.Walls&edge == 0 {
			continue
		}
		var pos, size rl.Vector3
		var extFaces uint8 = 0
		switch edge {
		case WallEdgeNorth:
			pos = rl.NewVector3(worldX, centerY, worldZ-ts/2.0+WallThickness/2.0)
			size = rl.NewVector3(ts, wallH, WallThickness)
			if t.IsExterior || t.Type == TileWallExterior {
				extFaces |= FaceNorth
			}
		case WallEdgeSouth:
			pos = rl.NewVector3(worldX, centerY, worldZ+ts/2.0-WallThickness/2.0)
			size = rl.NewVector3(ts, wallH, WallThickness)
			if t.IsExterior || t.Type == TileWallExterior {
				extFaces |= FaceSouth
			}
		case WallEdgeWest:
			pos = rl.NewVector3(worldX-ts/2.0+WallThickness/2.0, centerY, worldZ)
			size = rl.NewVector3(WallThickness, wallH, ts)
			if t.IsExterior || t.Type == TileWallExterior {
				extFaces |= FaceWest
			}
		case WallEdgeEast:
			pos = rl.NewVector3(worldX+ts/2.0-WallThickness/2.0, centerY, worldZ)
			size = rl.NewVector3(WallThickness, wallH, ts)
			if t.IsExterior || t.Type == TileWallExterior {
				extFaces |= FaceEast
			}
		}

		ws := WallSegment{
			Edge:          edge,
			Type:          t.Type,
			Position:      pos,
			Size:          size,
			Color:         t.Color,
			WireColor:     t.WireColor,
			IsSolid:       true,
			IsExterior:    t.IsExterior || t.Type == TileWallExterior,
			IsInterior:    t.IsInterior || t.Type == TileWallInterior,
			ExteriorFaces: extFaces,
		}
		t.WallSegments = append(t.WallSegments, ws)
	}

	if len(t.WallSegments) > 0 {
		t.IsSolid = true
		t.Position = t.WallSegments[0].Position
		t.Size = t.WallSegments[0].Size
	}
}

// rebuildStairSteps generates the individual step bounding boxes and 3D positions for TileStairs.
func (t *Tile3D) rebuildStairSteps(ts, baseY float32) {
	if t.StepCount <= 0 {
		t.StepCount = 4
	}
	if t.StairHeight <= 0 {
		t.StairHeight = 1.1
	}

	baseFloorY := baseY
	if t.HasFlooring {
		baseFloorY = baseY + FloorHeight
	}
	bottomY := baseFloorY + t.StartHeight

	n := t.StepCount
	stepRise := t.StairHeight / float32(n)
	stepRun := ts / float32(n)

	worldX := t.Position.X
	worldZ := t.Position.Z

	t.Steps = make([]StairStep, n)
	t.StepBoxes = make([]rl.BoundingBox, 0, n+1)

	// If this staircase is elevated on top of a lower flight (StartHeight > 0),
	// add the solid foundation bounding box underneath the steps.
	if t.StartHeight > 0.001 {
		foundationBB := rl.NewBoundingBox(
			rl.NewVector3(worldX-ts/2.0, baseFloorY, worldZ-ts/2.0),
			rl.NewVector3(worldX+ts/2.0, bottomY, worldZ+ts/2.0),
		)
		t.StepBoxes = append(t.StepBoxes, foundationBB)
	}

	for k := 0; k < n; k++ {
		stepH := float32(k+1) * stepRise
		centerY := bottomY + stepH/2.0

		var pos, size rl.Vector3
		var bb rl.BoundingBox

		switch t.StairDir {
		case StairAscendNorth:
			// Low at South (+Z), High at North (-Z)
			zMin := worldZ + ts/2.0 - float32(k+1)*stepRun
			zMax := worldZ + ts/2.0 - float32(k)*stepRun
			centerZ := (zMin + zMax) / 2.0
			pos = rl.NewVector3(worldX, centerY, centerZ)
			size = rl.NewVector3(ts, stepH, stepRun)
			bb = rl.NewBoundingBox(
				rl.NewVector3(worldX-ts/2.0, baseFloorY, zMin),
				rl.NewVector3(worldX+ts/2.0, bottomY+stepH, zMax),
			)

		case StairAscendSouth:
			// Low at North (-Z), High at South (+Z)
			zMin := worldZ - ts/2.0 + float32(k)*stepRun
			zMax := worldZ - ts/2.0 + float32(k+1)*stepRun
			centerZ := (zMin + zMax) / 2.0
			pos = rl.NewVector3(worldX, centerY, centerZ)
			size = rl.NewVector3(ts, stepH, stepRun)
			bb = rl.NewBoundingBox(
				rl.NewVector3(worldX-ts/2.0, baseFloorY, zMin),
				rl.NewVector3(worldX+ts/2.0, bottomY+stepH, zMax),
			)

		case StairAscendEast:
			// Low at West (-X), High at East (+X)
			xMin := worldX - ts/2.0 + float32(k)*stepRun
			xMax := worldX - ts/2.0 + float32(k+1)*stepRun
			centerX := (xMin + xMax) / 2.0
			pos = rl.NewVector3(centerX, centerY, worldZ)
			size = rl.NewVector3(stepRun, stepH, ts)
			bb = rl.NewBoundingBox(
				rl.NewVector3(xMin, baseFloorY, worldZ-ts/2.0),
				rl.NewVector3(xMax, bottomY+stepH, worldZ+ts/2.0),
			)

		case StairAscendWest:
			// Low at East (+X), High at West (-X)
			xMin := worldX + ts/2.0 - float32(k+1)*stepRun
			xMax := worldX + ts/2.0 - float32(k)*stepRun
			centerX := (xMin + xMax) / 2.0
			pos = rl.NewVector3(centerX, centerY, worldZ)
			size = rl.NewVector3(stepRun, stepH, ts)
			bb = rl.NewBoundingBox(
				rl.NewVector3(xMin, baseFloorY, worldZ-ts/2.0),
				rl.NewVector3(xMax, bottomY+stepH, worldZ+ts/2.0),
			)
		}

		t.Steps[k] = StairStep{
			Index:       k,
			Position:    pos,
			Size:        size,
			BoundingBox: bb,
		}
		t.StepBoxes = append(t.StepBoxes, bb)
	}

	totalH := t.StartHeight + t.StairHeight
	t.IsSolid = true
	t.Position = rl.NewVector3(worldX, baseFloorY+totalH/2.0, worldZ)
	t.Size = rl.NewVector3(ts, totalH, ts)
}

// GetBoundingBox returns the 3D bounding box for this tile.
func (t *Tile3D) GetBoundingBox() rl.BoundingBox {
	if (t.Type == TileStairs || t.IsStairs) && len(t.StepBoxes) > 0 {
		bb := t.StepBoxes[0]
		for _, sb := range t.StepBoxes[1:] {
			if sb.Min.X < bb.Min.X {
				bb.Min.X = sb.Min.X
			}
			if sb.Min.Y < bb.Min.Y {
				bb.Min.Y = sb.Min.Y
			}
			if sb.Min.Z < bb.Min.Z {
				bb.Min.Z = sb.Min.Z
			}
			if sb.Max.X > bb.Max.X {
				bb.Max.X = sb.Max.X
			}
			if sb.Max.Y > bb.Max.Y {
				bb.Max.Y = sb.Max.Y
			}
			if sb.Max.Z > bb.Max.Z {
				bb.Max.Z = sb.Max.Z
			}
		}
		return bb
	}
	if len(t.WallSegments) == 1 {
		return t.WallSegments[0].GetBoundingBox()
	}
	if len(t.WallSegments) > 1 {
		bb := t.WallSegments[0].GetBoundingBox()
		for _, ws := range t.WallSegments[1:] {
			wsBB := ws.GetBoundingBox()
			if wsBB.Min.X < bb.Min.X {
				bb.Min.X = wsBB.Min.X
			}
			if wsBB.Min.Y < bb.Min.Y {
				bb.Min.Y = wsBB.Min.Y
			}
			if wsBB.Min.Z < bb.Min.Z {
				bb.Min.Z = wsBB.Min.Z
			}
			if wsBB.Max.X > bb.Max.X {
				bb.Max.X = wsBB.Max.X
			}
			if wsBB.Max.Y > bb.Max.Y {
				bb.Max.Y = wsBB.Max.Y
			}
			if wsBB.Max.Z > bb.Max.Z {
				bb.Max.Z = wsBB.Max.Z
			}
		}
		return bb
	}
	halfW := t.Size.X / 2.0
	halfH := t.Size.Y / 2.0
	halfD := t.Size.Z / 2.0
	return rl.NewBoundingBox(
		rl.NewVector3(t.Position.X-halfW, t.Position.Y-halfH, t.Position.Z-halfD),
		rl.NewVector3(t.Position.X+halfW, t.Position.Y+halfH, t.Position.Z+halfD),
	)
}

// RayIntersects checks if a ray intersects this 3D tile or any of its wall segments or stair steps.
func (t *Tile3D) RayIntersects(ray rl.Ray) (bool, float32) {
	if (t.Type == TileStairs || t.IsStairs) && len(t.StepBoxes) > 0 {
		hitAny := false
		minDist := float32(math.MaxFloat32)
		for _, sb := range t.StepBoxes {
			col := rl.GetRayCollisionBox(ray, sb)
			if col.Hit && col.Distance < minDist {
				hitAny = true
				minDist = col.Distance
			}
		}
		if t.HasFlooring {
			floorBB := rl.NewBoundingBox(
				rl.NewVector3(t.FloorPosition.X-t.FloorSize.X/2.0, t.FloorPosition.Y-t.FloorSize.Y/2.0, t.FloorPosition.Z-t.FloorSize.Z/2.0),
				rl.NewVector3(t.FloorPosition.X+t.FloorSize.X/2.0, t.FloorPosition.Y+t.FloorSize.Y/2.0, t.FloorPosition.Z+t.FloorSize.Z/2.0),
			)
			col := rl.GetRayCollisionBox(ray, floorBB)
			if col.Hit && col.Distance < minDist {
				hitAny = true
				minDist = col.Distance
			}
		}
		return hitAny, minDist
	}
	if len(t.WallSegments) > 0 {
		hitAny := false
		minDist := float32(math.MaxFloat32)
		for _, ws := range t.WallSegments {
			hit, dist := ws.RayIntersects(ray)
			if hit && dist < minDist {
				hitAny = true
				minDist = dist
			}
		}
		if t.HasFlooring {
			floorBB := rl.NewBoundingBox(
				rl.NewVector3(t.FloorPosition.X-t.FloorSize.X/2.0, t.FloorPosition.Y-t.FloorSize.Y/2.0, t.FloorPosition.Z-t.FloorSize.Z/2.0),
				rl.NewVector3(t.FloorPosition.X+t.FloorSize.X/2.0, t.FloorPosition.Y+t.FloorSize.Y/2.0, t.FloorPosition.Z+t.FloorSize.Z/2.0),
			)
			col := rl.GetRayCollisionBox(ray, floorBB)
			if col.Hit && col.Distance < minDist {
				hitAny = true
				minDist = col.Distance
			}
		}
		return hitAny, minDist
	}

	col := rl.GetRayCollisionBox(ray, t.GetBoundingBox())
	return col.Hit, col.Distance
}

// MapSpawnPoint represents an entity or trigger extracted from <objectgroup>.
type MapSpawnPoint struct {
	ID         int
	Name       string
	Type       string
	Position   rl.Vector3
	Width      float32
	Height     float32
	Properties map[string]string
}

// Map3D manages the 3D level geometry and spawned entities generated from a TMX map.
type Map3D struct {
	SourceMap    *Map
	TileSize     float32 // World units per tile (default 1.0)
	BaseY        float32 // Ground plane Y height (default -2.2)
	Tiles        []Tile3D
	Spawns       []MapSpawnPoint
	SpawnedCubes []*cube.RainbowCube
}

// NewMap3D parses and constructs a 3D level from a loaded TMX map.
func NewMap3D(m *Map, tileSize float32, baseY float32) *Map3D {
	m3d := &Map3D{
		SourceMap:    m,
		TileSize:     tileSize,
		BaseY:        baseY,
		Tiles:        make([]Tile3D, 0),
		Spawns:       make([]MapSpawnPoint, 0),
		SpawnedCubes: make([]*cube.RainbowCube, 0),
	}

	m3d.Build()
	return m3d
}

// Build translates TMX tile layers and objects into 3D geometry and spawners.
func (m3d *Map3D) Build() {
	m := m3d.SourceMap
	if m == nil {
		return
	}

	m3d.Tiles = make([]Tile3D, 0)
	m3d.Spawns = make([]MapSpawnPoint, 0)
	m3d.SpawnedCubes = make([]*cube.RainbowCube, 0)

	mapW := float32(m.Width)
	mapH := float32(m.Height)
	ts := m3d.TileSize

	// Process each tile layer
	for _, layer := range m.Layers {
		if !layer.IsVisible() {
			continue
		}

		for r := 0; r < len(layer.Tiles); r++ {
			for c := 0; c < len(layer.Tiles[r]); c++ {
				gid := layer.Tiles[r][c]
				if gid == 0 {
					continue
				}

				// Center map around (0, BaseY, 0)
				worldX := (float32(c) - mapW/2.0 + 0.5) * ts
				worldZ := (float32(r) - mapH/2.0 + 0.5) * ts

				// Check if an existing tile was already placed at (c, r) (e.g. from an earlier floor layer)
				existingIdx := -1
				for i := range m3d.Tiles {
					if m3d.Tiles[i].Col == c && m3d.Tiles[i].Row == r {
						existingIdx = i
						break
					}
				}

				flags := uint32(0)
				if r < len(layer.FlipFlags) && c < len(layer.FlipFlags[r]) {
					flags = layer.FlipFlags[r][c]
				}

				tile := m3d.createTile3D(gid, layer.Name, c, r, worldX, worldZ)
				tile.FlipFlags = flags
				if flags != 0 && tile.Walls != 0 {
					tile.Walls = RotateWallMask(tile.Walls, flags)
					tile.WallsExplicit = true
					tile.rebuildWallSegments(ts, m3d.BaseY)
				}
				if flags != 0 && (tile.Type == TileStairs || tile.IsStairs) {
					tile.StairDir = RotateStairDirection(tile.StairDir, flags)
					tile.rebuildStairSteps(ts, m3d.BaseY)
				}

				if existingIdx >= 0 {
					existing := &m3d.Tiles[existingIdx]
					if tile.IsSolid || tile.Walls != 0 || tile.Type == TileWallExterior || tile.Type == TileWallInterior || tile.Type == TileWall || tile.Type == TileStairs || tile.IsStairs {
						// Inherit the existing ground floor beneath this wall or staircase
						tile.HasFlooring = true
						tile.FloorGID = existing.GID
						tile.FloorType = existing.Type
						tile.FloorName = existing.Name
						tile.FloorColor = existing.Color
						tile.FloorWireColor = existing.WireColor
						tile.FloorPosition = existing.FloorPosition
						tile.FloorSize = existing.FloorSize
						if tile.Type == TileStairs || tile.IsStairs {
							tile.rebuildStairSteps(ts, m3d.BaseY)
						} else {
							tile.rebuildWallSegments(ts, m3d.BaseY)
						}
						m3d.Tiles[existingIdx] = tile
						continue
					}
				}

				m3d.Tiles = append(m3d.Tiles, tile)
			}
		}
	}

	// Resolve walls (0-4 walls per square tile) and exterior face siding orientations
	m3d.resolveTileWalls()
	m3d.resolveWallFaces()
	m3d.resolveStaircases()

	// Process object groups (entity spawners, player starts, custom cubes)
	for _, og := range m.ObjectGroups {
		for _, obj := range og.Objects {
			// Convert pixel coordinates (X, Y) to 3D world space
			tileX := obj.X / float32(m.TileWidth)
			tileY := obj.Y / float32(m.TileHeight)

			worldX := (tileX - mapW/2.0 + 0.5) * ts
			worldZ := (tileY - mapH/2.0 + 0.5) * ts

			props := make(map[string]string)
			for _, p := range obj.Properties {
				props[p.Name] = p.GetVal()
			}

			spawn := MapSpawnPoint{
				ID:         obj.ID,
				Name:       obj.Name,
				Type:       strings.ToLower(obj.Type),
				Position:   rl.NewVector3(worldX, m3d.BaseY+0.5, worldZ),
				Width:      obj.Width / float32(m.TileWidth) * ts,
				Height:     obj.Height / float32(m.TileHeight) * ts,
				Properties: props,
			}
			m3d.Spawns = append(m3d.Spawns, spawn)

			// If object type is "cube", spawn an interactive 3D RainbowCube
			if spawn.Type == "cube" || strings.HasPrefix(strings.ToLower(obj.Name), "cube") {
				sizeVal := float32(0.65)
				if val, ok := props["size"]; ok {
					if v, err := strconv.ParseFloat(val, 32); err == nil && v > 0 {
						sizeVal = float32(v)
					}
				}

				mode := cube.ModeFaceColors
				if mVal, ok := props["mode"]; ok {
					switch strings.ToLower(mVal) {
					case "gradient":
						mode = cube.ModeVertexGradient
					case "chroma":
						mode = cube.ModeChromaCycle
					}
				}

				cID := fmt.Sprintf("map_cube_%d", obj.ID)
				cName := obj.Name
				if cName == "" {
					cName = fmt.Sprintf("Cube_%d", obj.ID)
				}

				massVal := float32(35.0)
				if val, ok := props["mass"]; ok {
					if m, err := strconv.ParseFloat(val, 32); err == nil && m > 0 {
						massVal = float32(m)
					}
				}

				cPos := rl.NewVector3(worldX, m3d.BaseY+sizeVal+0.05, worldZ)
				sc := cube.NewRainbowCubeWithConfig(cID, cName, sizeVal, cPos, mode)
				sc.SetMass(massVal)
				m3d.SpawnedCubes = append(m3d.SpawnedCubes, sc)
			}
		}
	}
}

// parseWallMask parses a string into a bitmask of wall edges (0 to 4 walls).
func parseWallMask(val string) uint8 {
	v := strings.ToLower(strings.TrimSpace(val))
	switch v {
	case "0", "none", "false":
		return WallEdgeNone
	case "1":
		return WallEdgeSouth
	case "2":
		return WallEdgeNorth | WallEdgeSouth
	case "3":
		return WallEdgeNorth | WallEdgeSouth | WallEdgeWest
	case "4", "all", "true", "box", "enclosed":
		return WallEdgeNorth | WallEdgeSouth | WallEdgeEast | WallEdgeWest
	}

	var mask uint8 = 0
	tokens := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == '|' || r == ' ' || r == ';'
	})
	for _, token := range tokens {
		switch strings.TrimSpace(token) {
		case "north", "n":
			mask |= WallEdgeNorth
		case "south", "s":
			mask |= WallEdgeSouth
		case "east", "e":
			mask |= WallEdgeEast
		case "west", "w":
			mask |= WallEdgeWest
		case "all", "4":
			mask |= WallEdgeNorth | WallEdgeSouth | WallEdgeEast | WallEdgeWest
		case "none", "0":
			return WallEdgeNone
		}
	}
	return mask
}

func wallEdgeToString(edge uint8) string {
	switch edge {
	case WallEdgeNorth:
		return "North"
	case WallEdgeSouth:
		return "South"
	case WallEdgeEast:
		return "East"
	case WallEdgeWest:
		return "West"
	default:
		return "None"
	}
}

// RotateWallMask rotates and flips a wall edge mask according to Tiled flip/rotation flags.
func RotateWallMask(mask uint8, flags uint32) uint8 {
	if flags == 0 || mask == 0 {
		return mask
	}
	m := mask

	// 1. Diagonal flip (transpose x and y across main diagonal)
	// Swaps North (-Y) <-> West (-X), South (+Y) <-> East (+X)
	if flags&TiledFlipDiagonal != 0 {
		var newM uint8 = 0
		if m&WallEdgeNorth != 0 {
			newM |= WallEdgeWest
		}
		if m&WallEdgeWest != 0 {
			newM |= WallEdgeNorth
		}
		if m&WallEdgeSouth != 0 {
			newM |= WallEdgeEast
		}
		if m&WallEdgeEast != 0 {
			newM |= WallEdgeSouth
		}
		m = newM
	}

	// 2. Horizontal flip (negate x)
	// Swaps East (+X) <-> West (-X)
	if flags&TiledFlipHorizontal != 0 {
		var newM uint8 = 0
		if m&WallEdgeEast != 0 {
			newM |= WallEdgeWest
		}
		if m&WallEdgeWest != 0 {
			newM |= WallEdgeEast
		}
		newM |= m & (WallEdgeNorth | WallEdgeSouth)
		m = newM
	}

	// 3. Vertical flip (negate y)
	// Swaps North (-Y) <-> South (+Y)
	if flags&TiledFlipVertical != 0 {
		var newM uint8 = 0
		if m&WallEdgeNorth != 0 {
			newM |= WallEdgeSouth
		}
		if m&WallEdgeSouth != 0 {
			newM |= WallEdgeNorth
		}
		newM |= m & (WallEdgeEast | WallEdgeWest)
		m = newM
	}

	return m
}

// parseStairDirection parses a string into a StairDirection enum.
func parseStairDirection(val string) StairDirection {
	v := strings.ToLower(strings.TrimSpace(val))
	switch v {
	case "north", "n", "up", "0":
		return StairAscendNorth
	case "south", "s", "down", "1":
		return StairAscendSouth
	case "east", "e", "right", "2":
		return StairAscendEast
	case "west", "w", "left", "3":
		return StairAscendWest
	default:
		if strings.Contains(v, "south") {
			return StairAscendSouth
		}
		if strings.Contains(v, "east") {
			return StairAscendEast
		}
		if strings.Contains(v, "west") {
			return StairAscendWest
		}
		return StairAscendNorth
	}
}

// stairDirToString returns the human-readable direction name for a StairDirection.
func stairDirToString(dir StairDirection) string {
	switch dir {
	case StairAscendNorth:
		return "North"
	case StairAscendSouth:
		return "South"
	case StairAscendEast:
		return "East"
	case StairAscendWest:
		return "West"
	default:
		return "North"
	}
}

// RotateStairDirection rotates and flips a StairDirection according to Tiled flip/rotation flags.
func RotateStairDirection(dir StairDirection, flags uint32) StairDirection {
	if flags == 0 {
		return dir
	}
	d := dir

	// 1. Diagonal flip (transpose x and z across main diagonal)
	// Swaps North (-Z) <-> West (-X), South (+Z) <-> East (+X)
	if flags&TiledFlipDiagonal != 0 {
		switch d {
		case StairAscendNorth:
			d = StairAscendWest
		case StairAscendWest:
			d = StairAscendNorth
		case StairAscendSouth:
			d = StairAscendEast
		case StairAscendEast:
			d = StairAscendSouth
		}
	}

	// 2. Horizontal flip (negate x)
	// Swaps East (+X) <-> West (-X)
	if flags&TiledFlipHorizontal != 0 {
		switch d {
		case StairAscendEast:
			d = StairAscendWest
		case StairAscendWest:
			d = StairAscendEast
		}
	}

	// 3. Vertical flip (negate z)
	// Swaps North (-Z) <-> South (+Z)
	if flags&TiledFlipVertical != 0 {
		switch d {
		case StairAscendNorth:
			d = StairAscendSouth
		case StairAscendSouth:
			d = StairAscendNorth
		}
	}

	return d
}

// createWallSegment builds a single ~0.2m thick wall along an edge of a square tile.
func createWallSegment(edge uint8, wallType TileType, worldX, worldZ, ts, baseY, wallHeight float32, isExterior, isInterior bool, color, wireColor rl.Color) WallSegment {
	floorTopY := baseY + FloorHeight
	centerY := floorTopY + wallHeight/2.0

	var pos, size rl.Vector3
	var extFaces uint8 = 0

	switch edge {
	case WallEdgeNorth:
		// North edge (-Z): thickness 0.2 along Z, length ts along X
		pos = rl.NewVector3(worldX, centerY, worldZ-ts/2.0+WallThickness/2.0)
		size = rl.NewVector3(ts, wallHeight, WallThickness)
		if isExterior || wallType == TileWallExterior {
			extFaces |= FaceNorth
		}
	case WallEdgeSouth:
		// South edge (+Z): thickness 0.2 along Z, length ts along X
		pos = rl.NewVector3(worldX, centerY, worldZ+ts/2.0-WallThickness/2.0)
		size = rl.NewVector3(ts, wallHeight, WallThickness)
		if isExterior || wallType == TileWallExterior {
			extFaces |= FaceSouth
		}
	case WallEdgeWest:
		// West edge (-X): thickness 0.2 along X, length ts along Z
		pos = rl.NewVector3(worldX-ts/2.0+WallThickness/2.0, centerY, worldZ)
		size = rl.NewVector3(WallThickness, wallHeight, ts)
		if isExterior || wallType == TileWallExterior {
			extFaces |= FaceWest
		}
	case WallEdgeEast:
		// East edge (+X): thickness 0.2 along X, length ts along Z
		pos = rl.NewVector3(worldX+ts/2.0-WallThickness/2.0, centerY, worldZ)
		size = rl.NewVector3(WallThickness, wallHeight, ts)
		if isExterior || wallType == TileWallExterior {
			extFaces |= FaceEast
		}
	}

	return WallSegment{
		Edge:          edge,
		Type:          wallType,
		Position:      pos,
		Size:          size,
		Color:         color,
		WireColor:     wireColor,
		IsSolid:       true,
		IsExterior:    isExterior || wallType == TileWallExterior,
		IsInterior:    isInterior || wallType == TileWallInterior,
		ExteriorFaces: extFaces,
	}
}

func (m3d *Map3D) createTile3D(gid uint32, layerName string, col, row int, x, z float32) Tile3D {
	t := m3d.createBaseTile3D(gid, layerName, col, row, x, z)
	m3d.applyTilesetProperties(&t)
	return t
}

func (m3d *Map3D) createBaseTile3D(gid uint32, layerName string, col, row int, x, z float32) Tile3D {
	ts := m3d.TileSize

	switch gid {
	case 1:
		// Pavement / Road Floor
		height := FloorHeight
		pos := rl.NewVector3(x, m3d.BaseY+height/2.0, z)
		size := rl.NewVector3(ts, height, ts)
		color := rl.NewColor(45, 52, 65, 255)
		wireColor := rl.NewColor(30, 36, 46, 255)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       pos,
			Size:           size,
			Color:          color,
			WireColor:      wireColor,
			IsSolid:        false,
			Type:           TileFloor,
			Name:           "Floor (Asphalt)",
			HasFlooring:    true,
			FloorGID:       gid,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     color,
			FloorWireColor: wireColor,
			FloorPosition:  pos,
			FloorSize:      size,
			Walls:          WallEdgeNone,
		}
	case 2:
		// Wall / Perimeter Barrier (Stone Wall, ~0.2m thickness on top of asphalt flooring)
		wallH := float32(2.0)
		floorPos := rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z)
		floorSize := rl.NewVector3(ts, FloorHeight, ts)
		floorCol := rl.NewColor(45, 52, 65, 255)
		floorWire := rl.NewColor(30, 36, 46, 255)
		wallPos := rl.NewVector3(x, m3d.BaseY+FloorHeight+wallH/2.0, z+ts/2.0-WallThickness/2.0)
		wallSize := rl.NewVector3(ts, wallH, WallThickness)

		t := Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       wallPos,
			Size:           wallSize,
			Color:          rl.NewColor(90, 80, 85, 255),
			WireColor:      rl.NewColor(40, 35, 40, 255),
			IsSolid:        true,
			Type:           TileWall,
			Name:           "Stone Wall",
			HasFlooring:    true,
			FloorGID:       1,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     floorCol,
			FloorWireColor: floorWire,
			FloorPosition:  floorPos,
			FloorSize:      floorSize,
			Walls:          WallEdgeSouth,
		}
		t.rebuildWallSegments(ts, m3d.BaseY)
		return t

	case 3:
		// Barricade / Wooden Crate
		height := float32(0.9)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       rl.NewVector3(x, m3d.BaseY+height/2.0, z),
			Size:           rl.NewVector3(ts*0.88, height, ts*0.88),
			Color:          rl.NewColor(135, 90, 50, 255),
			WireColor:      rl.NewColor(60, 40, 20, 255),
			IsSolid:        true,
			Type:           TileBarricade,
			Name:           "Crate Barricade",
			HasFlooring:    true,
			FloorGID:       1,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     rl.NewColor(45, 52, 65, 255),
			FloorWireColor: rl.NewColor(30, 36, 46, 255),
			FloorPosition:  rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z),
			FloorSize:      rl.NewVector3(ts, FloorHeight, ts),
			Walls:          WallEdgeNone,
		}
	case 4:
		// Industrial Tower / Pillar
		height := float32(3.4)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       rl.NewVector3(x, m3d.BaseY+height/2.0, z),
			Size:           rl.NewVector3(ts*0.7, height, ts*0.7),
			Color:          rl.NewColor(50, 110, 150, 255),
			WireColor:      rl.NewColor(20, 50, 75, 255),
			IsSolid:        true,
			Type:           TilePillar,
			Name:           "Steel Pillar",
			HasFlooring:    true,
			FloorGID:       1,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     rl.NewColor(45, 52, 65, 255),
			FloorWireColor: rl.NewColor(30, 36, 46, 255),
			FloorPosition:  rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z),
			FloorSize:      rl.NewVector3(ts, FloorHeight, ts),
			Walls:          WallEdgeNone,
		}
	case 5:
		// Grass / Soil Terrain Floor
		height := FloorHeight
		pos := rl.NewVector3(x, m3d.BaseY+height/2.0, z)
		size := rl.NewVector3(ts, height, ts)
		color := rl.NewColor(42, 105, 55, 255)
		wireColor := rl.NewColor(25, 70, 35, 255)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       pos,
			Size:           size,
			Color:          color,
			WireColor:      wireColor,
			IsSolid:        false,
			Type:           TileGrass,
			Name:           "Grass Lawn",
			HasFlooring:    true,
			FloorGID:       gid,
			FloorType:      TileGrass,
			FloorName:      "Grass Lawn",
			FloorColor:     color,
			FloorWireColor: wireColor,
			FloorPosition:  pos,
			FloorSize:      size,
			Walls:          WallEdgeNone,
		}
	case 6:
		// Acid Pool / Water
		height := float32(0.05)
		pos := rl.NewVector3(x, m3d.BaseY+height/2.0, z)
		size := rl.NewVector3(ts, height, ts)
		color := rl.NewColor(30, 210, 180, 200)
		wireColor := rl.NewColor(10, 120, 100, 255)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       pos,
			Size:           size,
			Color:          color,
			WireColor:      wireColor,
			IsSolid:        false,
			Type:           TileWater,
			Name:           "Toxic Water Pool",
			HasFlooring:    true,
			FloorGID:       gid,
			FloorType:      TileWater,
			FloorName:      "Toxic Water Pool",
			FloorColor:     color,
			FloorWireColor: wireColor,
			FloorPosition:  pos,
			FloorSize:      size,
			Walls:          WallEdgeNone,
		}
	case 7:
		// Reinforced Metal Wall (~0.2m thickness on top of asphalt flooring)
		wallH := float32(2.2)
		floorPos := rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z)
		floorSize := rl.NewVector3(ts, FloorHeight, ts)
		floorCol := rl.NewColor(45, 52, 65, 255)
		floorWire := rl.NewColor(30, 36, 46, 255)
		wallPos := rl.NewVector3(x, m3d.BaseY+FloorHeight+wallH/2.0, z+ts/2.0-WallThickness/2.0)
		wallSize := rl.NewVector3(ts, wallH, WallThickness)

		t := Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       wallPos,
			Size:           wallSize,
			Color:          rl.NewColor(58, 66, 78, 255),
			WireColor:      rl.NewColor(130, 150, 175, 255),
			IsSolid:        true,
			Type:           TileWall,
			Name:           "Reinforced Metal Wall",
			HasFlooring:    true,
			FloorGID:       1,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     floorCol,
			FloorWireColor: floorWire,
			FloorPosition:  floorPos,
			FloorSize:      floorSize,
			Walls:          WallEdgeSouth,
		}
		t.rebuildWallSegments(ts, m3d.BaseY)
		return t

	case 8:
		// Glass Crystal Barrier
		height := float32(1.5)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       rl.NewVector3(x, m3d.BaseY+height/2.0, z),
			Size:           rl.NewVector3(ts*0.94, height, ts*0.94),
			Color:          rl.NewColor(160, 220, 245, 170),
			WireColor:      rl.NewColor(210, 240, 255, 240),
			IsSolid:        true,
			Type:           TileWall,
			Name:           "Glass Crystal Barrier",
			HasFlooring:    true,
			FloorGID:       1,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     rl.NewColor(45, 52, 65, 255),
			FloorWireColor: rl.NewColor(30, 36, 46, 255),
			FloorPosition:  rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z),
			FloorSize:      rl.NewVector3(ts, FloorHeight, ts),
			Walls:          WallEdgeNone,
		}
	case 9:
		// Sand Terrain Floor
		height := FloorHeight
		pos := rl.NewVector3(x, m3d.BaseY+height/2.0, z)
		size := rl.NewVector3(ts, height, ts)
		color := rl.NewColor(195, 170, 115, 255)
		wireColor := rl.NewColor(155, 130, 80, 255)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       pos,
			Size:           size,
			Color:          color,
			WireColor:      wireColor,
			IsSolid:        false,
			Type:           TileFloor,
			Name:           "Sand Terrain",
			HasFlooring:    true,
			FloorGID:       gid,
			FloorType:      TileFloor,
			FloorName:      "Sand Terrain",
			FloorColor:     color,
			FloorWireColor: wireColor,
			FloorPosition:  pos,
			FloorSize:      size,
			Walls:          WallEdgeNone,
		}
	case 10:
		// Wood Plank Floor
		height := FloorHeight
		pos := rl.NewVector3(x, m3d.BaseY+height/2.0, z)
		size := rl.NewVector3(ts, height, ts)
		color := rl.NewColor(125, 85, 50, 255)
		wireColor := rl.NewColor(85, 55, 30, 255)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       pos,
			Size:           size,
			Color:          color,
			WireColor:      wireColor,
			IsSolid:        false,
			Type:           TileFloor,
			Name:           "Wood Plank Floor",
			HasFlooring:    true,
			FloorGID:       gid,
			FloorType:      TileFloor,
			FloorName:      "Wood Plank Floor",
			FloorColor:     color,
			FloorWireColor: wireColor,
			FloorPosition:  pos,
			FloorSize:      size,
			Walls:          WallEdgeNone,
		}
	case 11:
		// Lava / Magma Hazard Pool
		height := float32(0.06)
		pos := rl.NewVector3(x, m3d.BaseY+height/2.0, z)
		size := rl.NewVector3(ts, height, ts)
		color := rl.NewColor(245, 80, 25, 230)
		wireColor := rl.NewColor(255, 160, 40, 255)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       pos,
			Size:           size,
			Color:          color,
			WireColor:      wireColor,
			IsSolid:        false,
			Type:           TileWater,
			Name:           "Lava Magma Pool",
			HasFlooring:    true,
			FloorGID:       gid,
			FloorType:      TileWater,
			FloorName:      "Lava Magma Pool",
			FloorColor:     color,
			FloorWireColor: wireColor,
			FloorPosition:  pos,
			FloorSize:      size,
			Walls:          WallEdgeNone,
		}
	case 12:
		// Concrete Curb / Step Barrier
		height := float32(0.45)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       rl.NewVector3(x, m3d.BaseY+height/2.0, z),
			Size:           rl.NewVector3(ts*0.96, height, ts*0.96),
			Color:          rl.NewColor(145, 150, 155, 255),
			WireColor:      rl.NewColor(95, 100, 105, 255),
			IsSolid:        true,
			Type:           TileBarricade,
			Name:           "Concrete Curb",
			HasFlooring:    true,
			FloorGID:       1,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     rl.NewColor(45, 52, 65, 255),
			FloorWireColor: rl.NewColor(30, 36, 46, 255),
			FloorPosition:  rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z),
			FloorSize:      rl.NewVector3(ts, FloorHeight, ts),
			Walls:          WallEdgeNone,
		}
	case 13:
		// Cyber Grid Floor
		height := FloorHeight
		pos := rl.NewVector3(x, m3d.BaseY+height/2.0, z)
		size := rl.NewVector3(ts, height, ts)
		color := rl.NewColor(20, 28, 42, 255)
		wireColor := rl.NewColor(0, 230, 200, 255)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       pos,
			Size:           size,
			Color:          color,
			WireColor:      wireColor,
			IsSolid:        false,
			Type:           TileFloor,
			Name:           "Cyber Grid Floor",
			HasFlooring:    true,
			FloorGID:       gid,
			FloorType:      TileFloor,
			FloorName:      "Cyber Grid Floor",
			FloorColor:     color,
			FloorWireColor: wireColor,
			FloorPosition:  pos,
			FloorSize:      size,
			Walls:          WallEdgeNone,
		}
	case 14:
		// Exterior Wall (Slanted Siding, ~0.2m thickness on top of asphalt flooring)
		wallH := DefaultWallHeight
		floorPos := rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z)
		floorSize := rl.NewVector3(ts, FloorHeight, ts)
		floorCol := rl.NewColor(45, 52, 65, 255) // Underlying Asphalt floor
		floorWire := rl.NewColor(30, 36, 46, 255)
		wallPos := rl.NewVector3(x, m3d.BaseY+FloorHeight+wallH/2.0, z+ts/2.0-WallThickness/2.0)
		wallSize := rl.NewVector3(ts, wallH, WallThickness)

		t := Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       wallPos,
			Size:           wallSize,
			Color:          rl.NewColor(185, 178, 165, 255),
			WireColor:      rl.NewColor(85, 80, 75, 255),
			IsSolid:        true,
			Type:           TileWallExterior,
			Name:           "Exterior Wall (Slanted Siding)",
			IsExterior:     true,
			IsInterior:     false,
			ExteriorFaces:  FaceSouth,
			HasFlooring:    true,
			FloorGID:       1,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     floorCol,
			FloorWireColor: floorWire,
			FloorPosition:  floorPos,
			FloorSize:      floorSize,
			Walls:          WallEdgeSouth, // 1 wall on South edge by default
		}
		t.rebuildWallSegments(ts, m3d.BaseY)
		return t

	case 15:
		// Interior Wall (Plain Drywall, ~0.2m thickness on top of wood plank flooring)
		wallH := DefaultWallHeight
		floorPos := rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z)
		floorSize := rl.NewVector3(ts, FloorHeight, ts)
		floorCol := rl.NewColor(125, 85, 50, 255) // Underlying Wood Plank floor
		floorWire := rl.NewColor(85, 55, 30, 255)
		wallPos := rl.NewVector3(x, m3d.BaseY+FloorHeight+wallH/2.0, z+ts/2.0-WallThickness/2.0)
		wallSize := rl.NewVector3(ts, wallH, WallThickness)

		t := Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       wallPos,
			Size:           wallSize,
			Color:          rl.NewColor(232, 228, 220, 255),
			WireColor:      rl.NewColor(170, 165, 155, 255),
			IsSolid:        true,
			Type:           TileWallInterior,
			Name:           "Interior Wall (Plain Drywall)",
			IsExterior:     false,
			IsInterior:     true,
			ExteriorFaces:  0,
			HasFlooring:    true,
			FloorGID:       10,
			FloorType:      TileFloor,
			FloorName:      "Wood Plank Floor",
			FloorColor:     floorCol,
			FloorWireColor: floorWire,
			FloorPosition:  floorPos,
			FloorSize:      floorSize,
			Walls:          WallEdgeSouth, // 1 wall on South edge by default
		}
		t.rebuildWallSegments(ts, m3d.BaseY)
		return t

	case 25, 26, 27, 28:
		// Pioneer Wood Staircase
		var dir StairDirection
		var dirName string
		switch gid {
		case 25:
			dir = StairAscendNorth
			dirName = "North"
		case 26:
			dir = StairAscendSouth
			dirName = "South"
		case 27:
			dir = StairAscendEast
			dirName = "East"
		case 28:
			dir = StairAscendWest
			dirName = "West"
		}

		stairH := float32(1.1)
		floorPos := rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z)
		floorSize := rl.NewVector3(ts, FloorHeight, ts)
		floorCol := rl.NewColor(125, 85, 50, 255)
		floorWire := rl.NewColor(85, 55, 30, 255)

		t := Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       rl.NewVector3(x, m3d.BaseY+FloorHeight+stairH/2.0, z),
			Size:           rl.NewVector3(ts, stairH, ts),
			Color:          rl.NewColor(135, 92, 54, 255),
			WireColor:      rl.NewColor(75, 50, 25, 255),
			IsSolid:        true,
			Type:           TileStairs,
			Name:           fmt.Sprintf("Pioneer Wood Staircase (%s)", dirName),
			IsStairs:       true,
			StairDir:       dir,
			StepCount:      4,
			StairHeight:    stairH,
			HasFlooring:    true,
			FloorGID:       10,
			FloorType:      TileFloor,
			FloorName:      "Wood Plank Floor",
			FloorColor:     floorCol,
			FloorWireColor: floorWire,
			FloorPosition:  floorPos,
			FloorSize:      floorSize,
		}
		t.rebuildStairSteps(ts, m3d.BaseY)
		return t

	case 29:
		// Pioneer Loft Platform (2.2m elevated wood platform matching wall height)
		height := float32(2.2)
		floorPos := rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z)
		floorSize := rl.NewVector3(ts, FloorHeight, ts)
		platformPos := rl.NewVector3(x, m3d.BaseY+FloorHeight+height/2.0, z)
		platformSize := rl.NewVector3(ts, height, ts)
		return Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       platformPos,
			Size:           platformSize,
			Color:          rl.NewColor(135, 92, 54, 255),
			WireColor:      rl.NewColor(80, 55, 30, 255),
			IsSolid:        true,
			Type:           TileBarricade,
			Name:           "Pioneer Loft Platform",
			HasFlooring:    true,
			FloorGID:       10,
			FloorType:      TileFloor,
			FloorName:      "Wood Plank Floor",
			FloorColor:     rl.NewColor(125, 85, 50, 255),
			FloorWireColor: rl.NewColor(85, 55, 30, 255),
			FloorPosition:  floorPos,
			FloorSize:      floorSize,
			Walls:          WallEdgeNone,
		}

	case 30:
		// Pioneer Wood Staircase (North - Upper Flight)
		stairH := float32(1.1)
		startH := float32(1.1)
		floorPos := rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z)
		floorSize := rl.NewVector3(ts, FloorHeight, ts)
		floorCol := rl.NewColor(125, 85, 50, 255)
		floorWire := rl.NewColor(85, 55, 30, 255)

		t := Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       rl.NewVector3(x, m3d.BaseY+FloorHeight+(startH+stairH)/2.0, z),
			Size:           rl.NewVector3(ts, startH+stairH, ts),
			Color:          rl.NewColor(135, 92, 54, 255),
			WireColor:      rl.NewColor(75, 50, 25, 255),
			IsSolid:        true,
			Type:           TileStairs,
			Name:           "Pioneer Wood Staircase (North - Upper)",
			IsStairs:       true,
			StairDir:       StairAscendNorth,
			StepCount:      4,
			StairHeight:    stairH,
			StartHeight:    startH,
			HasFlooring:    true,
			FloorGID:       10,
			FloorType:      TileFloor,
			FloorName:      "Wood Plank Floor",
			FloorColor:     floorCol,
			FloorWireColor: floorWire,
			FloorPosition:  floorPos,
			FloorSize:      floorSize,
		}
		t.rebuildStairSteps(ts, m3d.BaseY)
		return t

	default:
		// Generic 3D Block / Custom Tileset Tile
		height := float32(1.0)
		t := Tile3D{
			GID:            gid,
			LayerName:      layerName,
			Col:            col,
			Row:            row,
			Position:       rl.NewVector3(x, m3d.BaseY+height/2.0, z),
			Size:           rl.NewVector3(ts*0.92, height, ts*0.92),
			Color:          rl.NewColor(120, 125, 135, 255),
			WireColor:      rl.NewColor(60, 65, 75, 255),
			IsSolid:        true,
			Type:           TileWall,
			Name:           fmt.Sprintf("Tile GID %d", gid),
			HasFlooring:    true,
			FloorGID:       1,
			FloorType:      TileFloor,
			FloorName:      "Floor (Asphalt)",
			FloorColor:     rl.NewColor(45, 52, 65, 255),
			FloorWireColor: rl.NewColor(30, 36, 46, 255),
			FloorPosition:  rl.NewVector3(x, m3d.BaseY+FloorHeight/2.0, z),
			FloorSize:      rl.NewVector3(ts, FloorHeight, ts),
			Walls:          WallEdgeNone,
		}
		return t
	}
}

// applyTilesetProperties applies custom TMX tileset properties (walls, wall_type, flooring, etc.) to any tile.
func (m3d *Map3D) applyTilesetProperties(t *Tile3D) {
	if m3d.SourceMap == nil {
		return
	}
	ts := m3d.TileSize
	gid := t.GID

	for _, tsSet := range m3d.SourceMap.Tilesets {
		if gid >= tsSet.FirstGID && gid < tsSet.FirstGID+uint32(tsSet.TileCount) {
			tileID := gid - tsSet.FirstGID
			for _, tt := range tsSet.Tiles {
				if tt.ID == tileID {
					for _, p := range tt.Properties {
						pName := strings.ToLower(p.Name)
						pVal := strings.ToLower(p.GetVal())
						if pName == "name" {
							t.Name = p.GetVal()
						} else if pName == "wall_type" {
							if pVal == "exterior" {
								t.Type = TileWallExterior
								t.IsExterior = true
								t.IsInterior = false
								t.ExteriorFaces = FaceSouth
								t.Name = "Exterior Wall (Slanted Siding)"
								if !t.WallsExplicit {
									t.Walls = WallEdgeSouth
								}
								t.Color = rl.NewColor(185, 178, 165, 255)
								t.WireColor = rl.NewColor(85, 80, 75, 255)
							} else if pVal == "interior" {
								t.Type = TileWallInterior
								t.IsInterior = true
								t.IsExterior = false
								t.ExteriorFaces = 0
								t.Name = "Interior Wall (Plain Drywall)"
								if !t.WallsExplicit {
									t.Walls = WallEdgeSouth
								}
								t.Color = rl.NewColor(232, 228, 220, 255)
								t.WireColor = rl.NewColor(170, 165, 155, 255)
								t.FloorGID = 10
								t.FloorName = "Wood Plank Floor"
								t.FloorColor = rl.NewColor(125, 85, 50, 255)
								t.FloorWireColor = rl.NewColor(85, 55, 30, 255)
							}
						} else if pName == "type" {
							if pVal == "stairs" || pVal == "stair" || pVal == "staircase" {
								t.Type = TileStairs
								t.IsStairs = true
								t.IsSolid = true
							}
						} else if pName == "direction" || pName == "dir" || pName == "stair_dir" {
							t.StairDir = parseStairDirection(pVal)
						} else if pName == "steps" || pName == "step_count" {
							if s, err := strconv.Atoi(pVal); err == nil && s > 0 {
								t.StepCount = s
							}
						} else if pName == "rise" || pName == "stair_height" {
							if h, err := strconv.ParseFloat(pVal, 32); err == nil && h > 0 {
								t.StairHeight = float32(h)
							}
						} else if pName == "start_height" || pName == "base_height" || pName == "elevation" {
							if sh, err := strconv.ParseFloat(pVal, 32); err == nil && sh >= 0 {
								t.StartHeight = float32(sh)
							}
						} else if pName == "flight" {
							if pVal == "upper" || pVal == "second" {
								if t.StartHeight == 0 {
									t.StartHeight = 1.1
								}
							}
						} else if pName == "height" && (t.Type == TileStairs || t.IsStairs) {
							if h, err := strconv.ParseFloat(pVal, 32); err == nil && h > 0 {
								t.StairHeight = float32(h)
							}
						} else if pName == "walls" || pName == "wall_edges" {
							t.Walls = parseWallMask(pVal)
							t.WallsExplicit = true
						} else if pName == "flooring" {
							switch pVal {
							case "wood":
								t.FloorGID = 10
								t.FloorName = "Wood Plank Floor"
								t.FloorColor = rl.NewColor(125, 85, 50, 255)
								t.FloorWireColor = rl.NewColor(85, 55, 30, 255)
							case "asphalt", "pavement":
								t.FloorGID = 1
								t.FloorName = "Floor (Asphalt)"
								t.FloorColor = rl.NewColor(45, 52, 65, 255)
								t.FloorWireColor = rl.NewColor(30, 36, 46, 255)
							case "cyber":
								t.FloorGID = 13
								t.FloorName = "Cyber Grid Floor"
								t.FloorColor = rl.NewColor(20, 28, 42, 255)
								t.FloorWireColor = rl.NewColor(0, 230, 200, 255)
							case "grass":
								t.FloorGID = 5
								t.FloorName = "Grass Lawn"
								t.FloorColor = rl.NewColor(42, 105, 55, 255)
								t.FloorWireColor = rl.NewColor(25, 70, 35, 255)
							case "sand":
								t.FloorGID = 9
								t.FloorName = "Sand Terrain"
								t.FloorColor = rl.NewColor(195, 170, 115, 255)
								t.FloorWireColor = rl.NewColor(155, 130, 80, 255)
							}
						}
					}
					if strings.Contains(strings.ToLower(t.Name), "stair") {
						t.Type = TileStairs
						t.IsStairs = true
						t.IsSolid = true
						lowerName := strings.ToLower(t.Name)
						if strings.Contains(lowerName, "south") {
							t.StairDir = StairAscendSouth
						} else if strings.Contains(lowerName, "east") {
							t.StairDir = StairAscendEast
						} else if strings.Contains(lowerName, "west") {
							t.StairDir = StairAscendWest
						} else if strings.Contains(lowerName, "north") {
							t.StairDir = StairAscendNorth
						}
						if (strings.Contains(lowerName, "upper") || strings.Contains(lowerName, "second")) && t.StartHeight == 0 {
							t.StartHeight = 1.1
						}
					}
					if t.Type == TileStairs || t.IsStairs {
						if t.StepCount <= 0 {
							t.StepCount = 4
						}
						if t.StairHeight <= 0 {
							t.StairHeight = 1.1
						}
						t.rebuildStairSteps(ts, m3d.BaseY)
					}
					if t.WallsExplicit || t.Walls != 0 || t.Type == TileWallExterior || t.Type == TileWallInterior {
						if t.Walls == 0 && !t.WallsExplicit {
							t.Walls = WallEdgeSouth
						}
						t.rebuildWallSegments(ts, m3d.BaseY)
					}
					return
				}
			}
		}
	}
}

// RayIntersectsClosest finds the closest 3D tile hit by a ray.
func (m3d *Map3D) RayIntersectsClosest(ray rl.Ray) (*Tile3D, float32) {
	var closest *Tile3D
	minDist := float32(math.MaxFloat32)

	for i := range m3d.Tiles {
		t := &m3d.Tiles[i]
		hit, dist := t.RayIntersects(ray)
		if hit && dist < minDist {
			minDist = dist
			closest = t
		}
	}

	return closest, minDist
}

// RemoveTileAt removes the tile at the specified column and row coordinate.
func (m3d *Map3D) RemoveTileAt(col, row int) bool {
	for i, t := range m3d.Tiles {
		if t.Col == col && t.Row == row {
			m3d.Tiles = append(m3d.Tiles[:i], m3d.Tiles[i+1:]...)
			return true
		}
	}
	return false
}

// GetSolidBoxes returns the bounding boxes of all solid tiles and 0.2m wall segments in the map.
func (m3d *Map3D) GetSolidBoxes() []rl.BoundingBox {
	var boxes []rl.BoundingBox
	for _, t := range m3d.Tiles {
		if !t.IsSolid {
			continue
		}
		if (t.Type == TileStairs || t.IsStairs) && len(t.StepBoxes) > 0 {
			boxes = append(boxes, t.StepBoxes...)
			continue
		}
		if len(t.WallSegments) > 0 {
			for _, ws := range t.WallSegments {
				if ws.IsSolid {
					boxes = append(boxes, ws.GetBoundingBox())
				}
			}
		} else {
			boxes = append(boxes, t.GetBoundingBox())
		}
	}
	return boxes
}

// isWallTile returns true if the tile is any wall type.
func isWallTile(t *Tile3D) bool {
	if t == nil {
		return false
	}
	return t.Type == TileWall || t.Type == TileWallExterior || t.Type == TileWallInterior
}

// isSameWallRun returns true if two adjacent tiles belong to the same continuous wall run.
func isSameWallRun(a, b *Tile3D) bool {
	if a == nil || b == nil || !b.IsSolid {
		return false
	}
	if !isWallTile(a) || !isWallTile(b) {
		return false
	}
	// House walls (exterior or interior) connect to other house walls
	if a.Type == TileWallExterior || a.Type == TileWallInterior {
		return b.Type == TileWallExterior || b.Type == TileWallInterior
	}
	// Stone / Stockade / Metal walls connect to walls of their own type/GID
	return b.GID == a.GID || b.Type == a.Type
}

// resolveTileWalls ensures all wall tiles have valid wall segments (0-4 walls)
// and handles neighbor adjacency in multi-row building layouts.
func (m3d *Map3D) resolveTileWalls() {
	ts := m3d.TileSize
	isMultiRow := m3d.SourceMap != nil && m3d.SourceMap.Height > 1

	for i := range m3d.Tiles {
		t := &m3d.Tiles[i]
		if t.Type != TileWallExterior && t.Type != TileWallInterior && t.Type != TileWall {
			continue
		}

		t.HasFlooring = true
		if t.FloorSize.X == 0 {
			t.FloorSize = rl.NewVector3(ts, FloorHeight, ts)
			t.FloorPosition = rl.NewVector3(t.Position.X, m3d.BaseY+FloorHeight/2.0, t.Position.Z)
			if t.Type == TileWallInterior || t.IsInterior {
				t.FloorGID = 10
				t.FloorType = TileFloor
				t.FloorName = "Wood Plank Floor"
				t.FloorColor = rl.NewColor(125, 85, 50, 255)
				t.FloorWireColor = rl.NewColor(85, 55, 30, 255)
			} else {
				t.FloorGID = 1
				t.FloorType = TileFloor
				t.FloorName = "Floor (Asphalt)"
				t.FloorColor = rl.NewColor(45, 52, 65, 255)
				t.FloorWireColor = rl.NewColor(30, 36, 46, 255)
			}
		}

		if !t.WallsExplicit {
			if isMultiRow {
				nNorth := m3d.findTileAt(t.Col, t.Row-1)
				nSouth := m3d.findTileAt(t.Col, t.Row+1)
				nEast := m3d.findTileAt(t.Col+1, t.Row)
				nWest := m3d.findTileAt(t.Col-1, t.Row)

				wallN := isSameWallRun(t, nNorth)
				wallS := isSameWallRun(t, nSouth)
				wallE := isSameWallRun(t, nEast)
				wallW := isSameWallRun(t, nWest)

				var mask uint8 = 0

				// 1. Corners (connecting in 2 perpendicular directions)
				if wallE && wallS && !wallN && !wallW {
					// NW Corner
					mask = WallEdgeNorth | WallEdgeWest
				} else if wallW && wallS && !wallN && !wallE {
					// NE Corner
					mask = WallEdgeNorth | WallEdgeEast
				} else if wallE && wallN && !wallS && !wallW {
					// SW Corner
					mask = WallEdgeSouth | WallEdgeWest
				} else if wallW && wallN && !wallS && !wallE {
					// SE Corner
					mask = WallEdgeSouth | WallEdgeEast
				// 2. T-Junctions (connecting in 3 directions)
				} else if wallN && wallS && wallE && !wallW {
					mask = WallEdgeWest
				} else if wallN && wallS && wallW && !wallE {
					mask = WallEdgeEast
				} else if wallE && wallW && wallS && !wallN {
					mask = WallEdgeNorth
				} else if wallE && wallW && wallN && !wallS {
					mask = WallEdgeSouth
				// 3. 4-way intersection (all 4 directions)
				} else if wallN && wallS && wallE && wallW {
					mask = WallEdgeNone
				// 4. East-West wall run (connecting along X, but no N/S wall neighbors)
				} else if (wallE || wallW) && !wallN && !wallS {
					if nNorth == nil {
						mask = WallEdgeNorth
					} else if nSouth == nil {
						mask = WallEdgeSouth
					} else if isInteriorTile(nSouth) {
						mask = WallEdgeNorth
					} else if isInteriorTile(nNorth) {
						mask = WallEdgeSouth
					} else if !nNorth.IsSolid {
						mask = WallEdgeNorth
					} else if !nSouth.IsSolid {
						mask = WallEdgeSouth
					} else {
						mask = WallEdgeSouth
					}
				// 5. North-South wall run (connecting along Z, but no E/W wall neighbors)
				} else if (wallN || wallS) && !wallE && !wallW {
					if nWest == nil {
						mask = WallEdgeWest
					} else if nEast == nil {
						mask = WallEdgeEast
					} else if isInteriorTile(nEast) {
						mask = WallEdgeWest
					} else if isInteriorTile(nWest) {
						mask = WallEdgeEast
					} else if !nWest.IsSolid {
						mask = WallEdgeWest
					} else if !nEast.IsSolid {
						mask = WallEdgeEast
					} else {
						mask = WallEdgeEast
					}
				// 6. Isolated wall tile or other configurations
				} else {
					if nNorth == nil || !nNorth.IsSolid {
						mask |= WallEdgeNorth
					}
					if nSouth == nil || !nSouth.IsSolid {
						mask |= WallEdgeSouth
					}
					if nEast == nil || !nEast.IsSolid {
						mask |= WallEdgeEast
					}
					if nWest == nil || !nWest.IsSolid {
						mask |= WallEdgeWest
					}
					if mask == 0 {
						mask = WallEdgeSouth
					}
				}
				t.Walls = mask
			} else {
				if t.Walls == 0 {
					t.Walls = WallEdgeSouth
				}
			}
		}

		t.rebuildWallSegments(ts, m3d.BaseY)
	}
}

// findTileAt searches for the tile at grid coordinates (col, row).
// If multiple tiles exist at (col, row), it prefers solid walls over floors.
func (m3d *Map3D) findTileAt(col, row int) *Tile3D {
	var fallback *Tile3D
	for i := range m3d.Tiles {
		if m3d.Tiles[i].Col == col && m3d.Tiles[i].Row == row {
			if m3d.Tiles[i].IsSolid {
				return &m3d.Tiles[i]
			}
			fallback = &m3d.Tiles[i]
		}
	}
	return fallback
}

// isInteriorTile returns true if the tile represents interior walls or indoor flooring.
func isInteriorTile(t *Tile3D) bool {
	if t == nil {
		return false
	}
	if t.Type == TileWallInterior || t.IsInterior {
		return true
	}
	name := strings.ToLower(t.Name)
	if strings.Contains(name, "drywall") || strings.Contains(name, "interior") || strings.Contains(name, "wood plank") {
		return true
	}
	return false
}

// resolveWallFaces determines which faces of exterior walls face outside (slanted siding)
// versus inside (plain painted drywall).
func (m3d *Map3D) resolveWallFaces() {
	isSingleRow := m3d.SourceMap != nil && m3d.SourceMap.Height == 1

	for i := range m3d.Tiles {
		t := &m3d.Tiles[i]
		if !t.IsExterior {
			continue
		}

		nNorth := m3d.findTileAt(t.Col, t.Row-1)
		nSouth := m3d.findTileAt(t.Col, t.Row+1)
		nEast := m3d.findTileAt(t.Col+1, t.Row)
		nWest := m3d.findTileAt(t.Col-1, t.Row)

		var extFaces uint8 = 0

		if isSingleRow {
			// Single-row showcase gallery (e.g. kitchen_sink):
			// South (+Z / Front facing observer) has slanted siding.
			// North (-Z / Back) has plain interior drywall (dual-face demonstration).
			extFaces |= FaceSouth
			if nWest == nil || !nWest.IsSolid {
				extFaces |= FaceWest
			}
			if nEast == nil || (!nEast.IsSolid && !isInteriorTile(nEast)) {
				extFaces |= FaceEast
			}
		} else {
			hasInterior := isInteriorTile(nNorth) || isInteriorTile(nSouth) || isInteriorTile(nEast) || isInteriorTile(nWest)
			if hasInterior {
				// Building with interior spaces:
				// Any face that does not face an interior tile is exterior
				if !isInteriorTile(nNorth) {
					extFaces |= FaceNorth
				}
				if !isInteriorTile(nSouth) {
					extFaces |= FaceSouth
				}
				if !isInteriorTile(nEast) {
					extFaces |= FaceEast
				}
				if !isInteriorTile(nWest) {
					extFaces |= FaceWest
				}
			} else {
				// Standalone wall segment in open terrain:
				// South (+Z) gets siding, North (-Z) gets plain drywall
				extFaces |= FaceSouth
				if nWest == nil || !nWest.IsSolid {
					extFaces |= FaceWest
				}
				if nEast == nil || !nEast.IsSolid {
					extFaces |= FaceEast
				}
			}
		}

		t.ExteriorFaces = extFaces
		for j := range t.WallSegments {
			ws := &t.WallSegments[j]
			if ws.IsExterior {
				switch ws.Edge {
				case WallEdgeNorth:
					if extFaces&FaceNorth != 0 {
						ws.ExteriorFaces |= FaceNorth
					}
				case WallEdgeSouth:
					if extFaces&FaceSouth != 0 {
						ws.ExteriorFaces |= FaceSouth
					}
				case WallEdgeEast:
					if extFaces&FaceEast != 0 {
						ws.ExteriorFaces |= FaceEast
					}
				case WallEdgeWest:
					if extFaces&FaceWest != 0 {
						ws.ExteriorFaces |= FaceWest
					}
				}
			}
		}
	}
}

// resolveStaircases connects adjacent staircase flights and platforms.
// If two staircase tiles are placed sequentially along their ascending direction,
// the upper staircase automatically sets its StartHeight = lowerStair.StartHeight + lowerStair.StairHeight
// (e.g. stacking a second flight to reach 2.2m from 1.1m) and rebuilds its steps with a solid base.
// Additionally, if the staircase ascends into a landing platform or barricade, its height is aligned.
func (m3d *Map3D) resolveStaircases() {
	ts := m3d.TileSize

	// Propagate heights across contiguous flights (up to 3 passes for multi-flight sequences)
	for pass := 0; pass < 3; pass++ {
		for i := range m3d.Tiles {
			t := &m3d.Tiles[i]
			if t.Type != TileStairs && !t.IsStairs {
				continue
			}

			// Determine target adjacent coordinate in the ascending direction
			nextCol, nextRow := t.Col, t.Row
			switch t.StairDir {
			case StairAscendNorth:
				nextRow--
			case StairAscendSouth:
				nextRow++
			case StairAscendEast:
				nextCol++
			case StairAscendWest:
				nextCol--
			}

			nAhead := m3d.findTileAt(nextCol, nextRow)
			if nAhead == nil {
				continue
			}

			// 1. If adjacent tile ahead is also a staircase ascending in the same direction,
			// stack it on top of this staircase!
			if (nAhead.Type == TileStairs || nAhead.IsStairs) && nAhead.StairDir == t.StairDir {
				topH := t.StartHeight + t.StairHeight
				if nAhead.StartHeight < topH {
					nAhead.StartHeight = topH
					nAhead.rebuildStairSteps(ts, m3d.BaseY)
				}
			}

			// 2. If adjacent tile ahead is a landing/loft platform, ensure its height matches the top of the stair
			if nAhead.Type == TileBarricade || strings.Contains(strings.ToLower(nAhead.Name), "loft") || strings.Contains(strings.ToLower(nAhead.Name), "platform") {
				topH := t.StartHeight + t.StairHeight
				if nAhead.Size.Y < topH {
					nAhead.Size.Y = topH
					baseFloorY := m3d.BaseY
					if nAhead.HasFlooring {
						baseFloorY += FloorHeight
					}
					nAhead.Position.Y = baseFloorY + topH/2.0
					m3d.alignConnectedPlatforms(nextCol, nextRow, topH)
				}
			}
		}
	}
}

// alignConnectedPlatforms propagates height to adjacent platform/barricade tiles (e.g. multi-tile lofts).
func (m3d *Map3D) alignConnectedPlatforms(startCol, startRow int, height float32) {
	deltas := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for _, d := range deltas {
		nc, nr := startCol+d[0], startRow+d[1]
		nb := m3d.findTileAt(nc, nr)
		if nb != nil && (nb.Type == TileBarricade || strings.Contains(strings.ToLower(nb.Name), "loft") || strings.Contains(strings.ToLower(nb.Name), "platform")) {
			if nb.Size.Y < height {
				nb.Size.Y = height
				baseFloorY := m3d.BaseY
				if nb.HasFlooring {
					baseFloorY += FloorHeight
				}
				nb.Position.Y = baseFloorY + height/2.0
			}
		}
	}
}

// drawSlabTopCap renders an architectural coping or top wall cap trim.
func (m3d *Map3D) drawSlabTopCap(pos, size rl.Vector3, capCol rl.Color) {
	topY := pos.Y + size.Y/2.0 + 0.01
	capPos := rl.NewVector3(pos.X, topY, pos.Z)
	capSize := rl.NewVector3(size.X, 0.025, size.Z)
	rl.DrawCubeV(capPos, capSize, capCol)
}

// drawWallTopCap is a backwards-compatible helper for full tiles.
func (m3d *Map3D) drawWallTopCap(t *Tile3D, capCol rl.Color) {
	m3d.drawSlabTopCap(t.Position, t.Size, capCol)
}

// drawSidingFaceZ renders 3D lap/bevel clapboard siding planks on a Z-facing wall face.
// dir is +1.0 for South (+Z) and -1.0 for North (-Z).
func (m3d *Map3D) drawSidingFaceZ(pos, size rl.Vector3, dir float32) {
	const numPlanks = 10
	W := size.X
	H := size.Y
	D := size.Z
	hw := W / 2.0
	hh := H / 2.0
	hd := D / 2.0
	yStart := pos.Y - hh
	hPlank := H / float32(numPlanks)
	faceZ := pos.Z + hd*dir

	// Full seamless span across the width of the wall (no side border trims)
	sx0 := pos.X - hw
	sx1 := pos.X + hw

	dIn := float32(0.003)
	dOut := float32(0.026)

	var zTop, zBot float32
	if dir > 0 {
		zTop = faceZ + dIn
		zBot = faceZ + dOut
	} else {
		zTop = faceZ - dIn
		zBot = faceZ - dOut
	}

	rl.Begin(rl.Quads)
	for k := 0; k < numPlanks; k++ {
		yBot := yStart + float32(k)*hPlank
		yTop := yBot + hPlank

		// Alternating subtle plank hue variation
		var plankCol rl.Color
		if k%2 == 0 {
			plankCol = rl.NewColor(190, 183, 171, 255)
		} else {
			plankCol = rl.NewColor(180, 174, 162, 255)
		}

		// Slanted lap plank face
		rl.Color4ub(plankCol.R, plankCol.G, plankCol.B, plankCol.A)
		if dir > 0 {
			rl.Vertex3f(sx0, yBot, zBot)
			rl.Vertex3f(sx1, yBot, zBot)
			rl.Vertex3f(sx1, yTop, zTop)
			rl.Vertex3f(sx0, yTop, zTop)
		} else {
			rl.Vertex3f(sx1, yBot, zBot)
			rl.Vertex3f(sx0, yBot, zBot)
			rl.Vertex3f(sx0, yTop, zTop)
			rl.Vertex3f(sx1, yTop, zTop)
		}

		// Undercut drip edge shadow shelf at bottom
		shadowCol := rl.NewColor(62, 57, 52, 255)
		rl.Color4ub(shadowCol.R, shadowCol.G, shadowCol.B, shadowCol.A)
		if dir > 0 {
			rl.Vertex3f(sx0, yBot, zBot)
			rl.Vertex3f(sx0, yBot, zTop)
			rl.Vertex3f(sx1, yBot, zTop)
			rl.Vertex3f(sx1, yBot, zBot)
		} else {
			rl.Vertex3f(sx1, yBot, zBot)
			rl.Vertex3f(sx1, yBot, zTop)
			rl.Vertex3f(sx0, yBot, zTop)
			rl.Vertex3f(sx0, yBot, zBot)
		}
	}
	rl.End()

	// Crisp horizontal lap shadow lines along the bottom edge of each plank (seamless span)
	shadowLineCol := rl.NewColor(48, 44, 40, 255)
	for k := 0; k < numPlanks; k++ {
		yBot := yStart + float32(k)*hPlank
		rl.DrawLine3D(
			rl.NewVector3(sx0, yBot, zBot),
			rl.NewVector3(sx1, yBot, zBot),
			shadowLineCol,
		)
	}
}

// drawSidingFaceX renders 3D lap/bevel clapboard siding planks on an X-facing wall face.
// dir is +1.0 for East (+X) and -1.0 for West (-X).
func (m3d *Map3D) drawSidingFaceX(pos, size rl.Vector3, dir float32) {
	const numPlanks = 10
	W := size.X
	H := size.Y
	D := size.Z
	hw := W / 2.0
	hh := H / 2.0
	hd := D / 2.0
	yStart := pos.Y - hh
	hPlank := H / float32(numPlanks)
	faceX := pos.X + hw*dir

	// Full seamless span across the depth of the wall (no side border trims)
	sz0 := pos.Z - hd
	sz1 := pos.Z + hd

	dIn := float32(0.003)
	dOut := float32(0.026)

	var xTop, xBot float32
	if dir > 0 {
		xTop = faceX + dIn
		xBot = faceX + dOut
	} else {
		xTop = faceX - dIn
		xBot = faceX - dOut
	}

	rl.Begin(rl.Quads)
	for k := 0; k < numPlanks; k++ {
		yBot := yStart + float32(k)*hPlank
		yTop := yBot + hPlank

		var plankCol rl.Color
		if k%2 == 0 {
			plankCol = rl.NewColor(190, 183, 171, 255)
		} else {
			plankCol = rl.NewColor(180, 174, 162, 255)
		}

		rl.Color4ub(plankCol.R, plankCol.G, plankCol.B, plankCol.A)
		if dir > 0 {
			rl.Vertex3f(xBot, yBot, sz1)
			rl.Vertex3f(xBot, yBot, sz0)
			rl.Vertex3f(xTop, yTop, sz0)
			rl.Vertex3f(xTop, yTop, sz1)
		} else {
			rl.Vertex3f(xBot, yBot, sz0)
			rl.Vertex3f(xBot, yBot, sz1)
			rl.Vertex3f(xTop, yTop, sz1)
			rl.Vertex3f(xTop, yTop, sz0)
		}

		shadowCol := rl.NewColor(62, 57, 52, 255)
		rl.Color4ub(shadowCol.R, shadowCol.G, shadowCol.B, shadowCol.A)
		if dir > 0 {
			rl.Vertex3f(xBot, yBot, sz0)
			rl.Vertex3f(xTop, yBot, sz0)
			rl.Vertex3f(xTop, yBot, sz1)
			rl.Vertex3f(xBot, yBot, sz1)
		} else {
			rl.Vertex3f(xBot, yBot, sz1)
			rl.Vertex3f(xTop, yBot, sz1)
			rl.Vertex3f(xTop, yBot, sz0)
			rl.Vertex3f(xBot, yBot, sz0)
		}
	}
	rl.End()

	shadowLineCol := rl.NewColor(48, 44, 40, 255)
	for k := 0; k < numPlanks; k++ {
		yBot := yStart + float32(k)*hPlank
		rl.DrawLine3D(
			rl.NewVector3(xBot, yBot, sz0),
			rl.NewVector3(xBot, yBot, sz1),
			shadowLineCol,
		)
	}
}

// drawPlainFaceZ renders a smooth plain painted drywall surface with baseboard molding on a Z face.
func (m3d *Map3D) drawPlainFaceZ(pos, size rl.Vector3, dir float32) {
	W := size.X
	H := size.Y
	D := size.Z
	hw := W / 2.0
	hh := H / 2.0
	hd := D / 2.0
	yStart := pos.Y - hh
	faceZ := pos.Z + hd*dir

	drywallCol := rl.NewColor(234, 230, 222, 255)
	drywallOffset := float32(0.003) * dir
	zFlat := faceZ + drywallOffset
	x0 := pos.X - hw
	x1 := pos.X + hw

	rl.Begin(rl.Quads)
	rl.Color4ub(drywallCol.R, drywallCol.G, drywallCol.B, drywallCol.A)
	if dir > 0 {
		rl.Vertex3f(x0, yStart, zFlat)
		rl.Vertex3f(x1, yStart, zFlat)
		rl.Vertex3f(x1, yStart+H, zFlat)
		rl.Vertex3f(x0, yStart+H, zFlat)
	} else {
		rl.Vertex3f(x1, yStart, zFlat)
		rl.Vertex3f(x0, yStart, zFlat)
		rl.Vertex3f(x0, yStart+H, zFlat)
		rl.Vertex3f(x1, yStart+H, zFlat)
	}
	rl.End()

	// Baseboard molding along the bottom (seamless across adjacent tiles)
	baseH := float32(0.14)
	baseD := float32(0.012)
	baseCol := rl.NewColor(248, 246, 240, 255)

	basePos := rl.NewVector3(pos.X, yStart+baseH/2.0, faceZ+(baseD/2.0)*dir)
	baseSize := rl.NewVector3(W, baseH, baseD)
	rl.DrawCubeV(basePos, baseSize, baseCol)

	topTrimZ := faceZ + baseD*dir
	rl.DrawLine3D(
		rl.NewVector3(x0, yStart+baseH, topTrimZ),
		rl.NewVector3(x1, yStart+baseH, topTrimZ),
		rl.NewColor(165, 160, 150, 255),
	)
}

// drawPlainFaceX renders a smooth plain painted drywall surface with baseboard molding on an X face.
func (m3d *Map3D) drawPlainFaceX(pos, size rl.Vector3, dir float32) {
	W := size.X
	H := size.Y
	D := size.Z
	hw := W / 2.0
	hh := H / 2.0
	hd := D / 2.0
	yStart := pos.Y - hh
	faceX := pos.X + hw*dir

	drywallCol := rl.NewColor(234, 230, 222, 255)
	drywallOffset := float32(0.003) * dir
	xFlat := faceX + drywallOffset
	z0 := pos.Z - hd
	z1 := pos.Z + hd

	rl.Begin(rl.Quads)
	rl.Color4ub(drywallCol.R, drywallCol.G, drywallCol.B, drywallCol.A)
	if dir > 0 {
		rl.Vertex3f(xFlat, yStart, z1)
		rl.Vertex3f(xFlat, yStart, z0)
		rl.Vertex3f(xFlat, yStart+H, z0)
		rl.Vertex3f(xFlat, yStart+H, z1)
	} else {
		rl.Vertex3f(xFlat, yStart, z0)
		rl.Vertex3f(xFlat, yStart, z1)
		rl.Vertex3f(xFlat, yStart+H, z1)
		rl.Vertex3f(xFlat, yStart+H, z0)
	}
	rl.End()

	baseH := float32(0.14)
	baseD := float32(0.012)
	baseCol := rl.NewColor(248, 246, 240, 255)

	basePos := rl.NewVector3(faceX+(baseD/2.0)*dir, yStart+baseH/2.0, pos.Z)
	baseSize := rl.NewVector3(baseD, baseH, D)
	rl.DrawCubeV(basePos, baseSize, baseCol)

	topTrimX := faceX + baseD*dir
	rl.DrawLine3D(
		rl.NewVector3(topTrimX, yStart+baseH, z0),
		rl.NewVector3(topTrimX, yStart+baseH, z1),
		rl.NewColor(165, 160, 150, 255),
	)
}

// drawWallSegment renders a single ~0.2m thick wall segment with appropriate siding/drywall.
func (m3d *Map3D) drawWallSegment(ws *WallSegment) {
	// Solid core slab
	rl.DrawCubeV(ws.Position, ws.Size, ws.Color)

	if ws.IsExterior || ws.Type == TileWallExterior {
		switch ws.Edge {
		case WallEdgeSouth:
			// South face outside (+Z) gets slanted lap siding
			m3d.drawSidingFaceZ(ws.Position, ws.Size, 1.0)
			// North face inside (-Z) gets plain drywall
			m3d.drawPlainFaceZ(ws.Position, ws.Size, -1.0)
		case WallEdgeNorth:
			// North face outside (-Z) gets slanted lap siding
			m3d.drawSidingFaceZ(ws.Position, ws.Size, -1.0)
			// South face inside (+Z) gets plain drywall
			m3d.drawPlainFaceZ(ws.Position, ws.Size, 1.0)
		case WallEdgeEast:
			// East face outside (+X) gets slanted lap siding
			m3d.drawSidingFaceX(ws.Position, ws.Size, 1.0)
			// West face inside (-X) gets plain drywall
			m3d.drawPlainFaceX(ws.Position, ws.Size, -1.0)
		case WallEdgeWest:
			// West face outside (-X) gets slanted lap siding
			m3d.drawSidingFaceX(ws.Position, ws.Size, -1.0)
			// East face inside (+X) gets plain drywall
			m3d.drawPlainFaceX(ws.Position, ws.Size, 1.0)
		}
		// Architectural top coping trim
		m3d.drawSlabTopCap(ws.Position, ws.Size, rl.NewColor(135, 130, 120, 255))
	} else if ws.IsInterior || ws.Type == TileWallInterior {
		// Interior wall: plain drywall on both sides
		if ws.Edge == WallEdgeSouth || ws.Edge == WallEdgeNorth {
			m3d.drawPlainFaceZ(ws.Position, ws.Size, 1.0)
			m3d.drawPlainFaceZ(ws.Position, ws.Size, -1.0)
		} else {
			m3d.drawPlainFaceX(ws.Position, ws.Size, 1.0)
			m3d.drawPlainFaceX(ws.Position, ws.Size, -1.0)
		}
		// Architectural top coping trim
		m3d.drawSlabTopCap(ws.Position, ws.Size, rl.NewColor(242, 238, 232, 255))
	} else {
		// Masonry / Stone / Metal / Stockade Wall (Generic 0.2m solid structural wall)
		// Clean stone/structural texture with architectural coping cap - NO DRYWALL OR BASEBOARDS!
		rl.DrawCubeWiresV(ws.Position, ws.Size, ws.WireColor)
		capCol := rl.NewColor(
			uint8(float32(ws.Color.R)*0.8),
			uint8(float32(ws.Color.G)*0.8),
			uint8(float32(ws.Color.B)*0.8),
			255,
		)
		m3d.drawSlabTopCap(ws.Position, ws.Size, capCol)
	}
}

// drawExteriorWall renders an exterior wall with outside slanted siding and inside plain drywall.
func (m3d *Map3D) drawExteriorWall(t *Tile3D) {
	// Solid core block (smooth, seamless)
	rl.DrawCubeV(t.Position, t.Size, t.Color)

	// South Face (+Z)
	if t.ExteriorFaces&FaceSouth != 0 {
		m3d.drawSidingFaceZ(t.Position, t.Size, 1.0)
	} else {
		m3d.drawPlainFaceZ(t.Position, t.Size, 1.0)
	}

	// North Face (-Z)
	if t.ExteriorFaces&FaceNorth != 0 {
		m3d.drawSidingFaceZ(t.Position, t.Size, -1.0)
	} else {
		m3d.drawPlainFaceZ(t.Position, t.Size, -1.0)
	}

	// East Face (+X)
	if t.ExteriorFaces&FaceEast != 0 {
		m3d.drawSidingFaceX(t.Position, t.Size, 1.0)
	} else {
		m3d.drawPlainFaceX(t.Position, t.Size, 1.0)
	}

	// West Face (-X)
	if t.ExteriorFaces&FaceWest != 0 {
		m3d.drawSidingFaceX(t.Position, t.Size, -1.0)
	} else {
		m3d.drawPlainFaceX(t.Position, t.Size, -1.0)
	}

	// Top cap coping
	m3d.drawSlabTopCap(t.Position, t.Size, rl.NewColor(135, 130, 120, 255))
}

// drawInteriorWall renders an interior wall with plain painted drywall and baseboards on all faces.
func (m3d *Map3D) drawInteriorWall(t *Tile3D) {
	// Solid core block (smooth, seamless)
	rl.DrawCubeV(t.Position, t.Size, t.Color)

	m3d.drawPlainFaceZ(t.Position, t.Size, 1.0)
	m3d.drawPlainFaceZ(t.Position, t.Size, -1.0)
	m3d.drawPlainFaceX(t.Position, t.Size, 1.0)
	m3d.drawPlainFaceX(t.Position, t.Size, -1.0)

	m3d.drawSlabTopCap(t.Position, t.Size, rl.NewColor(242, 238, 232, 255))
}

// drawStaircase renders a 3D architectural staircase with tread boards, risers, and edge detailing.
func (m3d *Map3D) drawStaircase(t *Tile3D) {
	if len(t.Steps) == 0 {
		return
	}

	baseCol := t.Color
	wireCol := t.WireColor
	if baseCol.A == 0 {
		baseCol = rl.NewColor(135, 92, 54, 255)
	}
	if wireCol.A == 0 {
		wireCol = rl.NewColor(75, 50, 25, 255)
	}

	// Tread highlight color (slightly polished/lighter wood)
	treadCol := rl.NewColor(
		uint8(min(255, int(baseCol.R)+24)),
		uint8(min(255, int(baseCol.G)+18)),
		uint8(min(255, int(baseCol.B)+12)),
		255,
	)
	treadWireCol := rl.NewColor(
		uint8(max(0, int(wireCol.R)-10)),
		uint8(max(0, int(wireCol.G)-10)),
		uint8(max(0, int(wireCol.B)-10)),
		255,
	)

	const treadThickness float32 = 0.028

	ts := m3d.TileSize
	baseFloorY := m3d.BaseY
	if t.HasFlooring {
		baseFloorY += FloorHeight
	}

	// 0. If elevated (StartHeight > 0), draw the solid foundation riser block beneath it
	if t.StartHeight > 0.001 {
		foundationPos := rl.NewVector3(t.Position.X, baseFloorY+t.StartHeight/2.0, t.Position.Z)
		foundationSize := rl.NewVector3(ts, t.StartHeight, ts)
		rl.DrawCubeV(foundationPos, foundationSize, baseCol)
		rl.DrawCubeWiresV(foundationPos, foundationSize, wireCol)
	}

	for _, step := range t.Steps {
		// 1. Draw solid riser/box
		rl.DrawCubeV(step.Position, step.Size, baseCol)
		rl.DrawCubeWiresV(step.Position, step.Size, wireCol)

		// 2. Draw finished architectural tread plate on top
		topY := step.Position.Y + step.Size.Y/2.0 + treadThickness/2.0
		var treadSize rl.Vector3
		treadPos := rl.NewVector3(step.Position.X, topY, step.Position.Z)

		switch t.StairDir {
		case StairAscendNorth:
			// Slight overhang toward South (+Z entrance)
			treadSize = rl.NewVector3(step.Size.X, treadThickness, step.Size.Z+0.02)
			treadPos.Z += 0.01
		case StairAscendSouth:
			// Slight overhang toward North (-Z entrance)
			treadSize = rl.NewVector3(step.Size.X, treadThickness, step.Size.Z+0.02)
			treadPos.Z -= 0.01
		case StairAscendEast:
			// Slight overhang toward West (-X entrance)
			treadSize = rl.NewVector3(step.Size.X+0.02, treadThickness, step.Size.Z)
			treadPos.X -= 0.01
		case StairAscendWest:
			// Slight overhang toward East (+X entrance)
			treadSize = rl.NewVector3(step.Size.X+0.02, treadThickness, step.Size.Z)
			treadPos.X += 0.01
		}

		rl.DrawCubeV(treadPos, treadSize, treadCol)
		rl.DrawCubeWiresV(treadPos, treadSize, treadWireCol)
	}
}

// Draw renders all 3D level tiles and visual elements.
func (m3d *Map3D) Draw() {
	for i := range m3d.Tiles {
		t := &m3d.Tiles[i]

		// 1. Draw underlying flooring if present
		if t.HasFlooring {
			rl.DrawCubeV(t.FloorPosition, t.FloorSize, t.FloorColor)
			rl.DrawCubeWiresV(t.FloorPosition, t.FloorSize, t.FloorWireColor)
		} else if len(t.WallSegments) == 0 && t.Type != TileStairs && !t.IsStairs {
			rl.DrawCubeV(t.Position, t.Size, t.Color)
			rl.DrawCubeWiresV(t.Position, t.Size, t.WireColor)
		}

		// 1b. Draw staircase if this tile is a staircase
		if t.Type == TileStairs || t.IsStairs {
			m3d.drawStaircase(t)
			continue
		}

		// 2. Draw active ~0.2m thick wall segments (0 to 4 walls per square tile)
		if len(t.WallSegments) > 0 {
			for j := range t.WallSegments {
				m3d.drawWallSegment(&t.WallSegments[j])
			}
			continue
		}

		// 3. Draw non-wall solid obstacles (crates, pillars, concrete curbs, glass barrier)
		if t.IsSolid && t.Type != TileFloor && t.Type != TileGrass && t.Type != TileWater && t.Type != TileStairs && !t.IsStairs {
			rl.DrawCubeV(t.Position, t.Size, t.Color)
			rl.DrawCubeWiresV(t.Position, t.Size, t.WireColor)
		}
	}

	// Render spawned entity markers
	for _, sp := range m3d.Spawns {
		if sp.Type == "player" {
			// Player spawn marker (Cyan beacon ring)
			rl.DrawCircle3D(rl.NewVector3(sp.Position.X, m3d.BaseY+0.03, sp.Position.Z), 0.5, rl.NewVector3(1, 0, 0), 90.0, rl.NewColor(80, 220, 255, 200))
			rl.DrawCubeWires(rl.NewVector3(sp.Position.X, m3d.BaseY+0.4, sp.Position.Z), 0.3, 0.8, 0.3, rl.NewColor(80, 220, 255, 255))
		} else if sp.Type == "zombie" || sp.Type == "enemy" {
			// Zombie spawn marker (Red beacon ring)
			rl.DrawCircle3D(rl.NewVector3(sp.Position.X, m3d.BaseY+0.03, sp.Position.Z), 0.5, rl.NewVector3(1, 0, 0), 90.0, rl.NewColor(255, 60, 60, 180))
			rl.DrawCubeWires(rl.NewVector3(sp.Position.X, m3d.BaseY+0.4, sp.Position.Z), 0.35, 0.8, 0.35, rl.NewColor(255, 60, 60, 255))
		} else if sp.Type == "item" || sp.Type == "pickup" || sp.Type == "loot" {
			// Item / Pickup marker (Golden beacon ring + diamond wireframe)
			rl.DrawCircle3D(rl.NewVector3(sp.Position.X, m3d.BaseY+0.03, sp.Position.Z), 0.45, rl.NewVector3(1, 0, 0), 90.0, rl.NewColor(255, 215, 0, 180))
			rl.DrawCubeWires(rl.NewVector3(sp.Position.X, m3d.BaseY+0.35, sp.Position.Z), 0.3, 0.3, 0.3, rl.NewColor(255, 215, 0, 255))
		} else if sp.Type == "health" || sp.Type == "medkit" {
			// Health / Medkit marker (Green beacon ring + cross wireframe)
			rl.DrawCircle3D(rl.NewVector3(sp.Position.X, m3d.BaseY+0.03, sp.Position.Z), 0.45, rl.NewVector3(1, 0, 0), 90.0, rl.NewColor(60, 255, 120, 180))
			rl.DrawCubeWires(rl.NewVector3(sp.Position.X, m3d.BaseY+0.35, sp.Position.Z), 0.25, 0.4, 0.25, rl.NewColor(60, 255, 120, 255))
		} else if sp.Type == "ammo" || sp.Type == "weapon" {
			// Ammo / Weapon marker (Orange beacon ring + ammo box wireframe)
			rl.DrawCircle3D(rl.NewVector3(sp.Position.X, m3d.BaseY+0.03, sp.Position.Z), 0.45, rl.NewVector3(1, 0, 0), 90.0, rl.NewColor(255, 150, 30, 180))
			rl.DrawCubeWires(rl.NewVector3(sp.Position.X, m3d.BaseY+0.35, sp.Position.Z), 0.4, 0.22, 0.22, rl.NewColor(255, 150, 30, 255))
		}
	}
}

// Map3DJSON defines the full JSON export schema for Map3D serialization.
type Map3DJSON struct {
	SavedAt       string              `json:"saved_at"`
	FilePath      string              `json:"file_path,omitempty"`
	Title         string              `json:"title"`
	Author        string              `json:"author,omitempty"`
	Width         int                 `json:"width"`
	Height        int                 `json:"height"`
	TileWidth     int                 `json:"tile_width"`
	TileHeight    int                 `json:"tile_height"`
	TileSizeWorld float32             `json:"tile_size_world"`
	BaseY         float32             `json:"base_y"`
	Properties    map[string]string   `json:"properties,omitempty"`
	Tiles         []Tile3DJSON        `json:"tiles"`
	Spawns        []MapSpawnPointJSON `json:"spawns"`
	SpawnedCubes  []SpawnedCubeJSON   `json:"spawned_cubes"`
}

type Tile3DJSON struct {
	GID           uint32            `json:"gid"`
	LayerName     string            `json:"layer_name"`
	Col           int               `json:"col"`
	Row           int               `json:"row"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	IsSolid       bool              `json:"is_solid"`
	IsExterior    bool              `json:"is_exterior,omitempty"`
	IsInterior    bool              `json:"is_interior,omitempty"`
	ExteriorFaces uint8             `json:"exterior_faces,omitempty"`
	HasFlooring   bool              `json:"has_flooring"`
	FloorGID      uint32            `json:"floor_gid,omitempty"`
	FloorType     string            `json:"floor_type,omitempty"`
	FloorName     string            `json:"floor_name,omitempty"`
	FloorPosition [3]float32        `json:"floor_position,omitempty"`
	FloorSize     [3]float32        `json:"floor_size,omitempty"`
	FloorColor    [4]uint8          `json:"floor_color,omitempty"`
	Walls         uint8             `json:"walls,omitempty"`
	WallCount     int               `json:"wall_count"`
	WallThickness float32           `json:"wall_thickness,omitempty"`
	WallSegments  []WallSegmentJSON `json:"wall_segments,omitempty"`
	Position      [3]float32        `json:"position"`
	Size          [3]float32        `json:"size"`
	Color         [4]uint8          `json:"color"`
	WireColor     [4]uint8          `json:"wire_color"`
	IsStairs      bool              `json:"is_stairs,omitempty"`
	StairDir      string            `json:"stair_direction,omitempty"`
	StepCount     int               `json:"step_count,omitempty"`
	StairHeight   float32           `json:"stair_height,omitempty"`
	StartHeight   float32           `json:"start_height,omitempty"`
}

type WallSegmentJSON struct {
	Edge          string     `json:"edge"`
	Position      [3]float32 `json:"position"`
	Size          [3]float32 `json:"size"`
	Color         [4]uint8   `json:"color"`
	WireColor     [4]uint8   `json:"wire_color"`
	IsExterior    bool       `json:"is_exterior,omitempty"`
	IsInterior    bool       `json:"is_interior,omitempty"`
	ExteriorFaces uint8      `json:"exterior_faces,omitempty"`
	IsSolid       bool       `json:"is_solid"`
}

type MapSpawnPointJSON struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Position   [3]float32        `json:"position"`
	Width      float32           `json:"width"`
	Height     float32           `json:"height"`
	Properties map[string]string `json:"properties,omitempty"`
}

type SpawnedCubeJSON struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Size      float32    `json:"size"`
	Mass      float32    `json:"mass"`
	Mode      string     `json:"mode"`
	Position  [3]float32 `json:"position"`
	Rotation  [3]float32 `json:"rotation"`
	Velocity  [3]float32 `json:"velocity"`
	IsCarried bool       `json:"is_carried"`
	IsDeleted bool       `json:"is_deleted"`
}

func tileTypeToString(tt TileType) string {
	switch tt {
	case TileFloor:
		return "Floor"
	case TileWall:
		return "Wall"
	case TileBarricade:
		return "Barricade"
	case TilePillar:
		return "Pillar"
	case TileGrass:
		return "Grass"
	case TileWater:
		return "Water"
	case TileWallInterior:
		return "WallInterior"
	case TileWallExterior:
		return "WallExterior"
	case TileStairs:
		return "Stairs"
	default:
		return "Empty"
	}
}

// ToJSON serializes the active Map3D instance to formatted JSON bytes.
func (m3d *Map3D) ToJSON() ([]byte, error) {
	if m3d == nil {
		return nil, fmt.Errorf("cannot serialize nil Map3D")
	}

	title := "Untitled Map"
	filePath := ""
	author := ""
	w, h := 0, 0
	tw, th := 32, 32
	props := make(map[string]string)

	if m3d.SourceMap != nil {
		filePath = m3d.SourceMap.FilePath
		title = m3d.SourceMap.GetProperty("title", "Untitled Map")
		author = m3d.SourceMap.GetProperty("author", "")
		w = m3d.SourceMap.Width
		h = m3d.SourceMap.Height
		tw = m3d.SourceMap.TileWidth
		th = m3d.SourceMap.TileHeight
		for _, p := range m3d.SourceMap.Properties {
			props[p.Name] = p.GetVal()
		}
	}

	tilesJSON := make([]Tile3DJSON, len(m3d.Tiles))
	for i, t := range m3d.Tiles {
		var wsJSON []WallSegmentJSON
		if len(t.WallSegments) > 0 {
			wsJSON = make([]WallSegmentJSON, len(t.WallSegments))
			for j, ws := range t.WallSegments {
				wsJSON[j] = WallSegmentJSON{
					Edge:          wallEdgeToString(ws.Edge),
					Position:      [3]float32{ws.Position.X, ws.Position.Y, ws.Position.Z},
					Size:          [3]float32{ws.Size.X, ws.Size.Y, ws.Size.Z},
					Color:         [4]uint8{ws.Color.R, ws.Color.G, ws.Color.B, ws.Color.A},
					WireColor:     [4]uint8{ws.WireColor.R, ws.WireColor.G, ws.WireColor.B, ws.WireColor.A},
					IsExterior:    ws.IsExterior,
					IsInterior:    ws.IsInterior,
					ExteriorFaces: ws.ExteriorFaces,
					IsSolid:       ws.IsSolid,
				}
			}
		}

		tilesJSON[i] = Tile3DJSON{
			GID:           t.GID,
			LayerName:     t.LayerName,
			Col:           t.Col,
			Row:           t.Row,
			Name:          t.Name,
			Type:          tileTypeToString(t.Type),
			IsSolid:       t.IsSolid,
			IsExterior:    t.IsExterior,
			IsInterior:    t.IsInterior,
			ExteriorFaces: t.ExteriorFaces,
			HasFlooring:   t.HasFlooring,
			FloorGID:      t.FloorGID,
			FloorType:     tileTypeToString(t.FloorType),
			FloorName:     t.FloorName,
			FloorPosition: [3]float32{t.FloorPosition.X, t.FloorPosition.Y, t.FloorPosition.Z},
			FloorSize:     [3]float32{t.FloorSize.X, t.FloorSize.Y, t.FloorSize.Z},
			FloorColor:    [4]uint8{t.FloorColor.R, t.FloorColor.G, t.FloorColor.B, t.FloorColor.A},
			Walls:         t.Walls,
			WallCount:     t.WallCount(),
			WallThickness: WallThickness,
			WallSegments:  wsJSON,
			Position:      [3]float32{t.Position.X, t.Position.Y, t.Position.Z},
			Size:          [3]float32{t.Size.X, t.Size.Y, t.Size.Z},
			Color:         [4]uint8{t.Color.R, t.Color.G, t.Color.B, t.Color.A},
			WireColor:     [4]uint8{t.WireColor.R, t.WireColor.G, t.WireColor.B, t.WireColor.A},
			IsStairs:      t.IsStairs || t.Type == TileStairs,
			StairDir:      stairDirToString(t.StairDir),
			StepCount:     t.StepCount,
			StairHeight:   t.StairHeight,
			StartHeight:   t.StartHeight,
		}
	}

	spawnsJSON := make([]MapSpawnPointJSON, len(m3d.Spawns))
	for i, sp := range m3d.Spawns {
		spawnsJSON[i] = MapSpawnPointJSON{
			ID:         sp.ID,
			Name:       sp.Name,
			Type:       sp.Type,
			Position:   [3]float32{sp.Position.X, sp.Position.Y, sp.Position.Z},
			Width:      sp.Width,
			Height:     sp.Height,
			Properties: sp.Properties,
		}
	}

	cubesJSON := make([]SpawnedCubeJSON, len(m3d.SpawnedCubes))
	for i, c := range m3d.SpawnedCubes {
		pos := c.GetPosition()
		rot := c.GetRotation()
		vel := c.GetVelocity()
		cubesJSON[i] = SpawnedCubeJSON{
			ID:        c.ID,
			Name:      c.Name,
			Size:      c.EffectiveSize(),
			Mass:      c.GetMass(),
			Mode:      c.Mode.String(),
			Position:  [3]float32{pos.X, pos.Y, pos.Z},
			Rotation:  [3]float32{rot.X, rot.Y, rot.Z},
			Velocity:  [3]float32{vel.X, vel.Y, vel.Z},
			IsCarried: c.IsHeld(),
			IsDeleted: c.IsDeleted,
		}
	}

	export := Map3DJSON{
		SavedAt:       time.Now().Format(time.RFC3339),
		FilePath:      filePath,
		Title:         title,
		Author:        author,
		Width:         w,
		Height:        h,
		TileWidth:     tw,
		TileHeight:    th,
		TileSizeWorld: m3d.TileSize,
		BaseY:         m3d.BaseY,
		Properties:    props,
		Tiles:         tilesJSON,
		Spawns:        spawnsJSON,
		SpawnedCubes:  cubesJSON,
	}

	return json.MarshalIndent(export, "", "  ")
}

// SaveJSON writes the serialized Map3D JSON data to destPath.
func (m3d *Map3D) SaveJSON(destPath string) error {
	data, err := m3d.ToJSON()
	if err != nil {
		return err
	}
	dir := filepath.Dir(destPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %q: %w", dir, err)
		}
	}
	return os.WriteFile(destPath, data, 0644)
}

// Dispose promptly frees and clears all map geometry, tiles, spawners, and references.
func (m3d *Map3D) Dispose() {
	if m3d == nil {
		return
	}
	m3d.Tiles = nil
	m3d.Spawns = nil
	m3d.SpawnedCubes = nil
	m3d.SourceMap = nil
}
