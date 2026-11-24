package tiled

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"zombie-project/internal/camera"
	"zombie-project/internal/cube"
)

func TestParseTMXValid(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="3" height="3" tilewidth="32" tileheight="32">
 <properties>
  <property name="title" value="Test Outpost"/>
  <property name="difficulty" value="Hard"/>
 </properties>
 <layer id="1" name="Ground" width="3" height="3">
  <data encoding="csv">
1,2,3,
4,5,6,
1,1,2
  </data>
 </layer>
 <objectgroup id="2" name="Spawns">
  <object id="10" name="Player" type="player" x="32" y="32" width="32" height="32">
   <properties>
    <property name="role" value="scout"/>
   </properties>
  </object>
  <object id="11" name="BonusCube" type="cube" x="64" y="64" width="32" height="32">
   <properties>
    <property name="size" value="0.9"/>
    <property name="mode" value="chroma"/>
   </properties>
  </object>
 </objectgroup>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("failed to parse TMX: %v", err)
	}

	if m.Width != 3 || m.Height != 3 {
		t.Errorf("expected map size 3x3, got %dx%d", m.Width, m.Height)
	}
	if m.GetProperty("title", "") != "Test Outpost" {
		t.Errorf("expected title 'Test Outpost', got '%s'", m.GetProperty("title", ""))
	}

	ground := m.GetLayerByName("Ground")
	if ground == nil {
		t.Fatalf("layer 'Ground' not found")
	}

	expectedGIDs := [][]uint32{
		{1, 2, 3},
		{4, 5, 6},
		{1, 1, 2},
	}

	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if ground.Tiles[y][x] != expectedGIDs[y][x] {
				t.Errorf("at (%d, %d): expected GID %d, got %d", x, y, expectedGIDs[y][x], ground.Tiles[y][x])
			}
		}
	}

	og := m.GetObjectGroupByName("Spawns")
	if og == nil || len(og.Objects) != 2 {
		t.Fatalf("expected 2 objects in Spawns group")
	}

	if og.Objects[0].Name != "Player" || og.Objects[0].GetProperty("role", "") != "scout" {
		t.Errorf("player object properties not parsed correctly")
	}
}

func TestMap3DConstruction(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="2" height="2" tilewidth="32" tileheight="32">
 <layer id="1" name="Base" width="2" height="2">
  <data encoding="csv">
1,2,
3,4
  </data>
 </layer>
 <objectgroup id="2" name="Entities">
  <object id="1" name="TestCube" type="cube" x="32" y="32" width="32" height="32">
   <properties>
    <property name="size" value="1.2"/>
   </properties>
  </object>
 </objectgroup>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	m3d := NewMap3D(m, 1.0, -2.0)
	if len(m3d.Tiles) != 4 {
		t.Errorf("expected 4 3D tiles, got %d", len(m3d.Tiles))
	}

	if len(m3d.SpawnedCubes) != 1 {
		t.Fatalf("expected 1 spawned cube, got %d", len(m3d.SpawnedCubes))
	}
	if m3d.SpawnedCubes[0].Name != "TestCube" {
		t.Errorf("expected cube name 'TestCube', got '%s'", m3d.SpawnedCubes[0].Name)
	}
	if m3d.SpawnedCubes[0].Size != 1.2 {
		t.Errorf("expected cube size 1.2, got %f", m3d.SpawnedCubes[0].Size)
	}

	// Test Raycasting against 3D tiles
	wallTile := m3d.Tiles[1] // GID 2 at col 1, row 0
	ray := rl.Ray{
		Position:  rl.NewVector3(wallTile.Position.X, wallTile.Position.Y+5.0, wallTile.Position.Z),
		Direction: rl.NewVector3(0, -1, 0),
	}

	hitTile, _ := m3d.RayIntersectsClosest(ray)
	if hitTile == nil || hitTile.GID != 2 {
		t.Errorf("expected ray to hit wall tile (GID 2)")
	}
}

func TestLoadTMXDisk(t *testing.T) {
	m, err := LoadTMX("../../maps/zombie_outpost.tmx")
	if err != nil {
		t.Fatalf("failed to load maps/zombie_outpost.tmx from disk: %v", err)
	}

	if m.Width != 14 || m.Height != 14 {
		t.Errorf("expected 14x14 map, got %dx%d", m.Width, m.Height)
	}
	if !strings.Contains(m.GetProperty("title", ""), "Zombie") {
		t.Errorf("expected title to contain 'Zombie', got '%s'", m.GetProperty("title", ""))
	}
}

func TestLoadKitchenSinkMap(t *testing.T) {
	m, err := LoadTMX("../../maps/kitchen_sink.tmx")
	if err != nil {
		t.Fatalf("failed to load maps/kitchen_sink.tmx from disk: %v", err)
	}

	if m.Width != 24 || m.Height != 1 {
		t.Errorf("expected 24x1 single long row map, got %dx%d", m.Width, m.Height)
	}
	if m.GetProperty("title", "") != "Kitchen Sink Gallery" {
		t.Errorf("expected title 'Kitchen Sink Gallery', got '%s'", m.GetProperty("title", ""))
	}

	m3d := NewMap3D(m, 1.0, -2.2)
	if len(m3d.Tiles) != 24 {
		t.Fatalf("expected 24 3D tiles, got %d", len(m3d.Tiles))
	}

	// Verify all tiles are in a single row (Row 0)
	for i, tile := range m3d.Tiles {
		if tile.Row != 0 {
			t.Errorf("tile index %d has row %d, expected 0", i, tile.Row)
		}
		if tile.Col != i {
			t.Errorf("tile index %d has col %d, expected %d", i, tile.Col, i)
		}
	}

	// Verify specific tile types across the single row
	if m3d.Tiles[0].Type != TileFloor || m3d.Tiles[0].IsSolid {
		t.Errorf("col 0 expected non-solid TileFloor (Asphalt)")
	}
	if m3d.Tiles[1].Type != TileGrass || m3d.Tiles[1].IsSolid {
		t.Errorf("col 1 expected non-solid TileGrass")
	}
	if m3d.Tiles[5].Type != TileWater || m3d.Tiles[5].IsSolid {
		t.Errorf("col 5 expected non-solid TileWater (Toxic Pool)")
	}
	if m3d.Tiles[7].Type != TileBarricade || !m3d.Tiles[7].IsSolid {
		t.Errorf("col 7 expected solid TileBarricade (Concrete Curb)")
	}
	if m3d.Tiles[8].Type != TileBarricade || !m3d.Tiles[8].IsSolid {
		t.Errorf("col 8 expected solid TileBarricade (Crate Barricade)")
	}
	if m3d.Tiles[10].Type != TileWall || !m3d.Tiles[10].IsSolid {
		t.Errorf("col 10 expected solid TileWall (Stone Wall)")
	}
	if m3d.Tiles[12].Type != TilePillar || !m3d.Tiles[12].IsSolid {
		t.Errorf("col 12 expected solid TilePillar (Steel Pillar)")
	}
	if m3d.Tiles[13].Type != TileWallExterior || !m3d.Tiles[13].IsSolid || !m3d.Tiles[13].IsExterior {
		t.Errorf("col 13 expected solid TileWallExterior (Exterior Wall Slanted Siding), got %+v", m3d.Tiles[13])
	}
	if m3d.Tiles[13].ExteriorFaces&FaceSouth == 0 {
		t.Errorf("col 13 expected FaceSouth to be set for exterior siding, got mask %04b", m3d.Tiles[13].ExteriorFaces)
	}
	if m3d.Tiles[14].Type != TileWallInterior || !m3d.Tiles[14].IsSolid || !m3d.Tiles[14].IsInterior {
		t.Errorf("col 14 expected solid TileWallInterior (Interior Wall Plain Drywall), got %+v", m3d.Tiles[14])
	}

	// Verify solid collision bounding boxes
	boxes := m3d.GetSolidBoxes()
	if len(boxes) == 0 {
		t.Errorf("expected solid collision boxes for obstacles in kitchen_sink row")
	}

	// Verify spawned entities and interactive cubes
	if len(m3d.SpawnedCubes) < 5 {
		t.Errorf("expected at least 5 spawned cubes in kitchen_sink, got %d", len(m3d.SpawnedCubes))
	}

	// Verify mass parsing on spawned cubes
	var heavyCube *cube.RainbowCube
	var microCube *cube.RainbowCube
	for _, c := range m3d.SpawnedCubes {
		if c.Name == "HeavySteelCrate" {
			heavyCube = c
		}
		if c.Name == "MicroCube" {
			microCube = c
		}
	}
	if heavyCube == nil || heavyCube.Mass != 120.0 {
		t.Errorf("expected HeavySteelCrate with mass 120, got %v", heavyCube)
	}
	if microCube == nil || microCube.Mass != 5.0 {
		t.Errorf("expected MicroCube with mass 5, got %v", microCube)
	}

	// Verify player and zombie spawns exist
	hasPlayer := false
	hasZombie := false
	for _, sp := range m3d.Spawns {
		if sp.Type == "player" {
			hasPlayer = true
		}
		if sp.Type == "zombie" {
			hasZombie = true
		}
	}
	if !hasPlayer {
		t.Errorf("expected player spawn in kitchen_sink map")
	}
	if !hasZombie {
		t.Errorf("expected zombie spawn in kitchen_sink map")
	}
}

func TestMap3DJSONAndDispose(t *testing.T) {
	m, err := LoadTMX("../../maps/kitchen_sink.tmx")
	if err != nil {
		t.Fatalf("failed loading kitchen_sink.tmx: %v", err)
	}

	m3d := NewMap3D(m, 1.0, -2.2)
	data, err := m3d.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize Map3D to JSON: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, "Kitchen Sink Gallery") {
		t.Errorf("expected JSON to contain title 'Kitchen Sink Gallery'")
	}
	if !strings.Contains(jsonStr, "HeavySteelCrate") {
		t.Errorf("expected JSON to contain spawned cube 'HeavySteelCrate'")
	}
	if !strings.Contains(jsonStr, `"width": 24`) {
		t.Errorf("expected JSON to contain width 24")
	}

	// Test SaveJSON
	tmpSavePath := filepath.Join(os.TempDir(), "test_map_snapshot.json")
	defer os.Remove(tmpSavePath)
	if err := m3d.SaveJSON(tmpSavePath); err != nil {
		t.Fatalf("failed SaveJSON: %v", err)
	}
	readBytes, err := os.ReadFile(tmpSavePath)
	if err != nil || len(readBytes) == 0 {
		t.Fatalf("expected written JSON file at %s", tmpSavePath)
	}

	// Test Dispose()
	m3d.Dispose()
	if m3d.Tiles != nil || m3d.Spawns != nil || m3d.SpawnedCubes != nil || m3d.SourceMap != nil {
		t.Errorf("expected Dispose() to clear all slices and references to nil")
	}
}

func TestExteriorInteriorWallDualFace(t *testing.T) {
	// 3x3 map representing an enclosed room corner with exterior and interior walls:
	// Row 0: Exterior Wall (14), Exterior Wall (14), Exterior Wall (14)
	// Row 1: Exterior Wall (14), Interior Floor (10), Interior Wall (15)
	// Row 2: Exterior Wall (14), Exterior Wall (14), Exterior Wall (14)
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="3" height="3" tilewidth="32" tileheight="32">
 <layer id="1" name="Walls" width="3" height="3">
  <data encoding="csv">
14,14,14,
14,10,15,
14,14,14
  </data>
 </layer>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	m3d := NewMap3D(m, 1.0, -2.0)
	if len(m3d.Tiles) != 9 {
		t.Fatalf("expected 9 tiles, got %d", len(m3d.Tiles))
	}

	// Center tile (1, 1) is Wood Floor (GID 10)
	floorTile := m3d.findTileAt(1, 1)
	if floorTile == nil || floorTile.Type != TileFloor {
		t.Fatalf("expected center tile to be TileFloor")
	}

	// Interior wall tile (2, 1) is GID 15
	intWall := m3d.findTileAt(2, 1)
	if intWall == nil || intWall.Type != TileWallInterior || !intWall.IsInterior || !intWall.IsSolid {
		t.Fatalf("expected interior wall at (2, 1)")
	}

	// North exterior wall tile (1, 0)
	northWall := m3d.findTileAt(1, 0)
	if northWall == nil || northWall.Type != TileWallExterior || !northWall.IsExterior {
		t.Fatalf("expected exterior wall at (1, 0)")
	}
	// North wall (1, 0) has interior neighbor to the South (1, 1)
	// Therefore, FaceNorth must be exterior siding, but FaceSouth (facing interior room) must be plain drywall
	if northWall.ExteriorFaces&FaceNorth == 0 {
		t.Errorf("expected northWall to have FaceNorth set for exterior siding")
	}
	if northWall.ExteriorFaces&FaceSouth != 0 {
		t.Errorf("expected northWall FaceSouth to NOT have exterior siding (should be plain interior drywall)")
	}

	// South exterior wall tile (1, 2)
	southWall := m3d.findTileAt(1, 2)
	if southWall == nil || southWall.Type != TileWallExterior || !southWall.IsExterior {
		t.Fatalf("expected exterior wall at (1, 2)")
	}
	// South wall (1, 2) has interior neighbor to the North (1, 1)
	// Therefore, FaceSouth must be exterior siding, but FaceNorth (facing interior room) must be plain drywall
	if southWall.ExteriorFaces&FaceSouth == 0 {
		t.Errorf("expected southWall to have FaceSouth set for exterior siding")
	}
	if southWall.ExteriorFaces&FaceNorth != 0 {
		t.Errorf("expected southWall FaceNorth to NOT have exterior siding (should be plain interior drywall)")
	}

	// West exterior wall tile (0, 1)
	westWall := m3d.findTileAt(0, 1)
	if westWall == nil || westWall.Type != TileWallExterior || !westWall.IsExterior {
		t.Fatalf("expected exterior wall at (0, 1)")
	}
	// West wall (0, 1) has interior neighbor to the East (1, 1)
	if westWall.ExteriorFaces&FaceWest == 0 {
		t.Errorf("expected westWall to have FaceWest set for exterior siding")
	}
	if westWall.ExteriorFaces&FaceEast != 0 {
		t.Errorf("expected westWall FaceEast to NOT have exterior siding (should be plain interior drywall)")
	}

	// Verify JSON export contains exterior and interior wall properties
	jsonBytes, err := m3d.ToJSON()
	if err != nil {
		t.Fatalf("JSON export failed: %v", err)
	}
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, `"type": "WallExterior"`) {
		t.Errorf("expected JSON to contain WallExterior type")
	}
	if !strings.Contains(jsonStr, `"type": "WallInterior"`) {
		t.Errorf("expected JSON to contain WallInterior type")
	}
	if !strings.Contains(jsonStr, `"is_exterior": true`) {
		t.Errorf("expected JSON to contain is_exterior: true")
	}
	if !strings.Contains(jsonStr, `"is_interior": true`) {
		t.Errorf("expected JSON to contain is_interior: true")
	}
}

func TestSeamlessExteriorSiding(t *testing.T) {
	// 2 adjacent exterior walls in a row: GID 14 at (0, 0) and (1, 0)
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="2" height="1" tilewidth="32" tileheight="32">
 <layer id="1" name="Walls" width="2" height="1">
  <data encoding="csv">
14,14
  </data>
 </layer>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	tileSize := float32(1.0)
	m3d := NewMap3D(m, tileSize, -2.0)
	if len(m3d.Tiles) != 2 {
		t.Fatalf("expected 2 tiles, got %d", len(m3d.Tiles))
	}

	t0 := m3d.findTileAt(0, 0)
	t1 := m3d.findTileAt(1, 0)
	if t0 == nil || t1 == nil {
		t.Fatalf("expected tiles at (0, 0) and (1, 0)")
	}

	// Verify both are exterior walls with full tileSize width for seamless alignment
	if t0.Size.X != tileSize || t1.Size.X != tileSize {
		t.Errorf("expected wall tile size X to equal tileSize (%f), got t0=%f, t1=%f", tileSize, t0.Size.X, t1.Size.X)
	}

	// Verify right edge of t0 exactly matches left edge of t1 (zero gap seamless alignment)
	t0RightEdge := t0.Position.X + t0.Size.X/2.0
	t1LeftEdge := t1.Position.X - t1.Size.X/2.0
	diff := t1LeftEdge - t0RightEdge
	if diff < -0.0001 || diff > 0.0001 {
		t.Errorf("expected seamless edge connection between adjacent walls, got diff %f (t0Right=%f, t1Left=%f)", diff, t0RightEdge, t1LeftEdge)
	}

	// Verify bounding boxes touch continuously
	bb0 := t0.GetBoundingBox()
	bb1 := t1.GetBoundingBox()
	if bb0.Max.X != bb1.Min.X {
		t.Errorf("expected bounding boxes to touch seamlessly: bb0.Max.X=%f != bb1.Min.X=%f", bb0.Max.X, bb1.Min.X)
	}

	// Verify both have exterior siding facing South (+Z)
	if t0.ExteriorFaces&FaceSouth == 0 || t1.ExteriorFaces&FaceSouth == 0 {
		t.Errorf("expected both adjacent walls to have FaceSouth exterior siding")
	}
}

func TestWallsFlooringAndThickness(t *testing.T) {
	// Map with Exterior Wall (GID 14) and Interior Wall (GID 15)
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="2" height="1" tilewidth="32" tileheight="32">
 <layer id="1" name="Walls" width="2" height="1">
  <data encoding="csv">
14,15
  </data>
 </layer>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	tileSize := float32(1.0)
	baseY := float32(-2.2)
	m3d := NewMap3D(m, tileSize, baseY)

	if len(m3d.Tiles) != 2 {
		t.Fatalf("expected 2 tiles, got %d", len(m3d.Tiles))
	}

	for i, tile := range m3d.Tiles {
		// 1. Flooring guarantee: Every wall tile must have underlying flooring
		if !tile.HasFlooring {
			t.Errorf("tile %d (GID %d): expected HasFlooring to be true", i, tile.GID)
		}
		if tile.FloorSize.X != tileSize || tile.FloorSize.Z != tileSize {
			t.Errorf("tile %d: expected FloorSize X,Z to be %f, got %v", i, tileSize, tile.FloorSize)
		}
		if tile.FloorSize.Y != FloorHeight {
			t.Errorf("tile %d: expected FloorSize.Y to be FloorHeight (%f), got %f", i, FloorHeight, tile.FloorSize.Y)
		}

		// 2. Thickness guarantee: Every active wall segment must occupy ~0.2m thickness
		if len(tile.WallSegments) == 0 {
			t.Errorf("tile %d: expected wall segments to be present, got 0", i)
		}
		for j, ws := range tile.WallSegments {
			thickness := ws.Size.Z
			if ws.Edge == WallEdgeEast || ws.Edge == WallEdgeWest {
				thickness = ws.Size.X
			}
			if thickness != WallThickness {
				t.Errorf("tile %d ws %d: expected thickness %f, got %f", i, j, WallThickness, thickness)
			}
			if ws.Size.Y != DefaultWallHeight {
				t.Errorf("tile %d ws %d: expected wall height %f, got %f", i, j, DefaultWallHeight, ws.Size.Y)
			}
		}
	}

	// 3. Solid collision boxes should reflect the 0.2m thickness segments
	boxes := m3d.GetSolidBoxes()
	if len(boxes) != 2 {
		t.Fatalf("expected 2 solid boxes, got %d", len(boxes))
	}
	for i, b := range boxes {
		depth := b.Max.Z - b.Min.Z
		width := b.Max.X - b.Min.X
		// One dimension is thickness (0.2m), the other is length (1.0m)
		if (depth < 0.19 || depth > 0.21) && (width < 0.19 || width > 0.21) {
			t.Errorf("box %d expected to have ~0.2m thickness, got depth=%f, width=%f", i, depth, width)
		}
	}
}

func TestTilesWith0to4Walls(t *testing.T) {
	// Custom map showcasing 0, 1, 2, 3, and 4 walls per square tile
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="5" height="1" tilewidth="32" tileheight="32">
 <tileset firstgid="1" name="tiles" tilewidth="32" tileheight="32" tilecount="5" columns="5">
  <tile id="0">
   <properties>
    <property name="name" value="Open Floor (0 Walls)"/>
    <property name="wall_type" value="exterior"/>
    <property name="walls" value="0"/>
    <property name="flooring" value="asphalt"/>
   </properties>
  </tile>
  <tile id="1">
   <properties>
    <property name="name" value="Single Wall (1 Wall: North)"/>
    <property name="wall_type" value="exterior"/>
    <property name="walls" value="north"/>
    <property name="flooring" value="asphalt"/>
   </properties>
  </tile>
  <tile id="2">
   <properties>
    <property name="name" value="Corner Walls (2 Walls: North+East)"/>
    <property name="wall_type" value="exterior"/>
    <property name="walls" value="north,east"/>
    <property name="flooring" value="asphalt"/>
   </properties>
  </tile>
  <tile id="3">
   <properties>
    <property name="name" value="Alcove Walls (3 Walls: North+South+West)"/>
    <property name="wall_type" value="interior"/>
    <property name="walls" value="north,south,west"/>
    <property name="flooring" value="wood"/>
   </properties>
  </tile>
  <tile id="4">
   <properties>
    <property name="name" value="Enclosed Cell (4 Walls: All)"/>
    <property name="wall_type" value="exterior"/>
    <property name="walls" value="all"/>
    <property name="flooring" value="wood"/>
   </properties>
  </tile>
 </tileset>
 <layer id="1" name="WallTestRow" width="5" height="1">
  <data encoding="csv">
1,2,3,4,5
  </data>
 </layer>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	tileSize := float32(1.0)
	baseY := float32(-2.2)
	m3d := NewMap3D(m, tileSize, baseY)

	if len(m3d.Tiles) != 5 {
		t.Fatalf("expected 5 tiles, got %d", len(m3d.Tiles))
	}

	// 0 Walls: Open Floor
	t0 := &m3d.Tiles[0]
	if t0.WallCount() != 0 || len(t0.WallSegments) != 0 {
		t.Errorf("t0: expected 0 walls, got count=%d, segments=%d", t0.WallCount(), len(t0.WallSegments))
	}
	if !t0.HasFlooring {
		t.Errorf("t0: expected HasFlooring to be true")
	}

	// 1 Wall: North
	t1 := &m3d.Tiles[1]
	if t1.WallCount() != 1 || len(t1.WallSegments) != 1 {
		t.Errorf("t1: expected 1 wall, got count=%d, segments=%d", t1.WallCount(), len(t1.WallSegments))
	}
	if !t1.HasWall(WallEdgeNorth) {
		t.Errorf("t1: expected WallEdgeNorth to be present")
	}
	if t1.WallSegments[0].Size.Z != WallThickness {
		t.Errorf("t1: expected wall thickness %f, got %f", WallThickness, t1.WallSegments[0].Size.Z)
	}

	// 2 Walls: North + East (Corner)
	t2 := &m3d.Tiles[2]
	if t2.WallCount() != 2 || len(t2.WallSegments) != 2 {
		t.Errorf("t2: expected 2 walls, got count=%d, segments=%d", t2.WallCount(), len(t2.WallSegments))
	}
	if !t2.HasWall(WallEdgeNorth) || !t2.HasWall(WallEdgeEast) {
		t.Errorf("t2: expected North and East walls to be present")
	}

	// 3 Walls: North + South + West (Alcove)
	t3 := &m3d.Tiles[3]
	if t3.WallCount() != 3 || len(t3.WallSegments) != 3 {
		t.Errorf("t3: expected 3 walls, got count=%d, segments=%d", t3.WallCount(), len(t3.WallSegments))
	}
	if !t3.HasWall(WallEdgeNorth) || !t3.HasWall(WallEdgeSouth) || !t3.HasWall(WallEdgeWest) {
		t.Errorf("t3: expected North, South, West walls")
	}
	if t3.HasWall(WallEdgeEast) {
		t.Errorf("t3: East wall should be open for alcove")
	}

	// 4 Walls: All Edges (Enclosed Cell)
	t4 := &m3d.Tiles[4]
	if t4.WallCount() != 4 || len(t4.WallSegments) != 4 {
		t.Errorf("t4: expected 4 walls, got count=%d, segments=%d", t4.WallCount(), len(t4.WallSegments))
	}
	if !t4.HasWall(WallEdgeNorth) || !t4.HasWall(WallEdgeSouth) || !t4.HasWall(WallEdgeEast) || !t4.HasWall(WallEdgeWest) {
		t.Errorf("t4: expected all 4 walls to be present")
	}

	// Center floor space verification:
	// For t4, distance between North wall center and South wall center is (ts - WallThickness) = 0.8m
	// Clear open floor between North wall inner face and South wall inner face is (ts - 2*WallThickness) = 0.6m
	var northWS, southWS *WallSegment
	for i := range t4.WallSegments {
		if t4.WallSegments[i].Edge == WallEdgeNorth {
			northWS = &t4.WallSegments[i]
		} else if t4.WallSegments[i].Edge == WallEdgeSouth {
			southWS = &t4.WallSegments[i]
		}
	}
	if northWS == nil || southWS == nil {
		t.Fatalf("expected both north and south wall segments on t4")
	}
	clearSpan := (southWS.Position.Z - southWS.Size.Z/2.0) - (northWS.Position.Z + northWS.Size.Z/2.0)
	expectedClear := tileSize - 2.0*WallThickness
	if clearSpan < expectedClear-0.001 || clearSpan > expectedClear+0.001 {
		t.Errorf("expected clear floor span %f, got %f", expectedClear, clearSpan)
	}

	// JSON export check
	jsonBytes, err := m3d.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, `"has_flooring": true`) {
		t.Errorf("expected JSON to contain has_flooring: true")
	}
	if !strings.Contains(jsonStr, `"wall_count": 4`) {
		t.Errorf("expected JSON to contain wall_count: 4")
	}
	if !strings.Contains(jsonStr, `"wall_thickness": 0.2`) {
		t.Errorf("expected JSON to contain wall_thickness: 0.2")
	}
	if !strings.Contains(jsonStr, `"wall_segments": [`) {
		t.Errorf("expected JSON to contain wall_segments array")
	}
}

func TestMultiLayerFloorInheritance(t *testing.T) {
	// Multi-layer map: Layer 1 is Wood Plank Floor (GID 10), Layer 2 is Exterior Wall (GID 14)
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="1" height="1" tilewidth="32" tileheight="32">
 <layer id="1" name="FloorLayer" width="1" height="1">
  <data encoding="csv">
10
  </data>
 </layer>
 <layer id="2" name="WallLayer" width="1" height="1">
  <data encoding="csv">
14
  </data>
 </layer>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	m3d := NewMap3D(m, 1.0, -2.0)
	if len(m3d.Tiles) != 1 {
		t.Fatalf("expected 1 merged tile at (0, 0), got %d", len(m3d.Tiles))
	}

	tile := &m3d.Tiles[0]
	if !tile.HasFlooring {
		t.Errorf("expected tile to have HasFlooring=true")
	}
	if tile.FloorGID != 10 {
		t.Errorf("expected FloorGID to inherit 10 (Wood Plank Floor), got %d", tile.FloorGID)
	}
	if tile.FloorName != "Wood Plank Floor" {
		t.Errorf("expected FloorName 'Wood Plank Floor', got '%s'", tile.FloorName)
	}
}

func TestKitchenSinkMap0to4Walls(t *testing.T) {
	path := filepath.Join("..", "..", "maps", "kitchen_sink.tmx")
	m, err := LoadTMX(path)
	if err != nil {
		t.Fatalf("failed to load %s: %v", path, err)
	}

	m3d := NewMap3D(m, 1.0, -2.2)
	if len(m3d.Tiles) != 24 {
		t.Fatalf("expected 24 tiles in kitchen_sink map, got %d", len(m3d.Tiles))
	}

	// Verify all tiles have flooring
	for i, tile := range m3d.Tiles {
		if !tile.HasFlooring {
			t.Errorf("kitchen_sink tile %d (%s) missing flooring", i, tile.Name)
		}
	}

	// Verify showcase of 0, 1, 2, 3, and 4 walls:
	// Tile 0 (Col 0): GID 1 (Floor) -> 0 walls
	if m3d.Tiles[0].WallCount() != 0 {
		t.Errorf("tile 0 expected 0 walls, got %d", m3d.Tiles[0].WallCount())
	}

	// Tile 13 (Col 13): GID 14 (Exterior Wall) -> 1 wall (South)
	t13 := &m3d.Tiles[13]
	if t13.WallCount() != 1 || !t13.HasWall(WallEdgeSouth) {
		t.Errorf("tile 13 expected 1 wall on South, got count=%d", t13.WallCount())
	}
	if t13.FloorName != "Floor (Asphalt)" {
		t.Errorf("tile 13 expected asphalt flooring, got %s", t13.FloorName)
	}

	// Tile 14 (Col 14): GID 15 (Interior Wall) -> 1 wall (South)
	t14 := &m3d.Tiles[14]
	if t14.WallCount() != 1 || !t14.HasWall(WallEdgeSouth) {
		t.Errorf("tile 14 expected 1 wall on South, got count=%d", t14.WallCount())
	}
	if t14.FloorName != "Wood Plank Floor" {
		t.Errorf("tile 14 expected wood plank flooring, got %s", t14.FloorName)
	}

	// Tile 15 (Col 15): GID 16 (Corner Walls) -> 2 walls (North + East)
	t15 := &m3d.Tiles[15]
	if t15.WallCount() != 2 || !t15.HasWall(WallEdgeNorth) || !t15.HasWall(WallEdgeEast) {
		t.Errorf("tile 15 expected 2 walls (North+East), got count=%d", t15.WallCount())
	}

	// Tile 16 (Col 16): GID 17 (Alcove Walls) -> 3 walls (North + South + West)
	t16 := &m3d.Tiles[16]
	if t16.WallCount() != 3 || !t16.HasWall(WallEdgeNorth) || !t16.HasWall(WallEdgeSouth) || !t16.HasWall(WallEdgeWest) {
		t.Errorf("tile 16 expected 3 walls (North+South+West), got count=%d", t16.WallCount())
	}

	// Tile 17 (Col 17): GID 18 (Enclosed Cell) -> 4 walls (All)
	t17 := &m3d.Tiles[17]
	if t17.WallCount() != 4 {
		t.Errorf("tile 17 expected 4 walls, got count=%d", t17.WallCount())
	}

	// Bounding boxes check for wall segments
	solidBoxes := m3d.GetSolidBoxes()
	if len(solidBoxes) == 0 {
		t.Errorf("expected solid boxes from kitchen_sink map")
	}
}

func TestLoadConestogaMap(t *testing.T) {
	path := filepath.Join("..", "..", "maps", "conestoga.tmx")
	m, err := LoadTMX(path)
	if err != nil {
		t.Fatalf("failed to load maps/conestoga.tmx: %v", err)
	}

	if m.Width != 16 || m.Height != 16 {
		t.Errorf("expected conestoga map to be 16x16, got %dx%d", m.Width, m.Height)
	}
	title := m.GetProperty("title", "")
	if !strings.Contains(title, "Conestoga") {
		t.Errorf("expected title to contain 'Conestoga', got '%s'", title)
	}

	tileSize := float32(1.0)
	baseY := float32(-2.2)
	m3d := NewMap3D(m, tileSize, baseY)

	if len(m3d.Tiles) != 256 {
		t.Fatalf("expected 256 tiles for 16x16 map, got %d", len(m3d.Tiles))
	}

	// 1. Verify all tiles have flooring
	for i, tile := range m3d.Tiles {
		if !tile.HasFlooring {
			t.Errorf("tile %d (%s at %d,%d) missing flooring", i, tile.Name, tile.Col, tile.Row)
		}
	}

	// 2. Verify Basic House (Floors & Walls): footprint Rows 3..7, Cols 3..9 (35 tiles)
	// All tiles in house have wood plank flooring
	for r := 3; r <= 7; r++ {
		for c := 3; c <= 9; c++ {
			tile := m3d.findTileAt(c, r)
			if tile == nil {
				t.Fatalf("missing house tile at (%d, %d)", c, r)
			}
			if !tile.HasFlooring {
				t.Errorf("house tile at (%d, %d) missing HasFlooring", c, r)
			}
			if tile.FloorName != "Wood Plank Floor" {
				t.Errorf("house tile at (%d, %d) expected Wood Plank Floor, got '%s'", c, r, tile.FloorName)
			}
		}
	}

	// House Exterior Walls:
	// North perimeter wall along Row 3 (slanted siding facing North, drywall facing South)
	tNW := m3d.findTileAt(3, 3)
	if tNW == nil || tNW.WallCount() != 2 || !tNW.HasWall(WallEdgeNorth) || !tNW.HasWall(WallEdgeWest) {
		t.Errorf("house NW corner at (3, 3) expected walls North+West, got count=%d", tNW.WallCount())
	}
	if !tNW.IsExterior {
		t.Errorf("house NW corner expected IsExterior=true")
	}

	tNE := m3d.findTileAt(9, 3)
	if tNE == nil || tNE.WallCount() != 2 || !tNE.HasWall(WallEdgeNorth) || !tNE.HasWall(WallEdgeEast) {
		t.Errorf("house NE corner at (9, 3) expected walls North+East, got count=%d", tNE.WallCount())
	}

	for c := 4; c <= 8; c++ {
		tWallN := m3d.findTileAt(c, 3)
		if tWallN == nil || tWallN.WallCount() != 1 || !tWallN.HasWall(WallEdgeNorth) {
			t.Errorf("house North wall at (%d, 3) expected 1 wall on North, got count=%d", c, tWallN.WallCount())
		}
	}

	// West exterior wall (Col 3, Rows 4..6)
	for r := 4; r <= 6; r++ {
		tWallW := m3d.findTileAt(3, r)
		if tWallW == nil || tWallW.WallCount() != 1 || !tWallW.HasWall(WallEdgeWest) {
			t.Errorf("house West wall at (3, %d) expected 1 wall on West, got count=%d", r, tWallW.WallCount())
		}
	}

	// East exterior wall (Col 9, Rows 4..6)
	for r := 4; r <= 6; r++ {
		tWallE := m3d.findTileAt(9, r)
		if tWallE == nil || tWallE.WallCount() != 1 || !tWallE.HasWall(WallEdgeEast) {
			t.Errorf("house East wall at (9, %d) expected 1 wall on East, got count=%d", r, tWallE.WallCount())
		}
	}

	// South exterior wall (Row 7) with Front Doorway at (6, 7)
	tSW := m3d.findTileAt(3, 7)
	if tSW == nil || tSW.WallCount() != 2 || !tSW.HasWall(WallEdgeSouth) || !tSW.HasWall(WallEdgeWest) {
		t.Errorf("house SW corner at (3, 7) expected walls South+West, got count=%d", tSW.WallCount())
	}
	tSE := m3d.findTileAt(9, 7)
	if tSE == nil || tSE.WallCount() != 2 || !tSE.HasWall(WallEdgeSouth) || !tSE.HasWall(WallEdgeEast) {
		t.Errorf("house SE corner at (9, 7) expected walls South+East, got count=%d", tSE.WallCount())
	}
	// Front Doorway at (6, 7)
	tDoor := m3d.findTileAt(6, 7)
	if tDoor == nil || tDoor.WallCount() != 0 || tDoor.IsSolid {
		t.Errorf("house front doorway at (6, 7) expected 0 walls and non-solid, got walls=%d, solid=%v", tDoor.WallCount(), tDoor.IsSolid)
	}

	// Interior Partition Walls at (5, 4) and (5, 6)
	tPart1 := m3d.findTileAt(5, 4)
	if tPart1 == nil || tPart1.WallCount() != 1 || !tPart1.HasWall(WallEdgeEast) || !tPart1.IsInterior {
		t.Errorf("interior partition at (5, 4) expected interior wall on East")
	}
	tPart2 := m3d.findTileAt(5, 6)
	if tPart2 == nil || tPart2.WallCount() != 1 || !tPart2.HasWall(WallEdgeEast) || !tPart2.IsInterior {
		t.Errorf("interior partition at (5, 6) expected interior wall on East")
	}

	// Interior Doorway connecting rooms at (5, 5)
	tIntDoor := m3d.findTileAt(5, 5)
	if tIntDoor == nil || tIntDoor.WallCount() != 0 || tIntDoor.IsSolid {
		t.Errorf("interior doorway at (5, 5) expected open wood floor (0 walls)")
	}

	// Interior Walkable Space: verify open walkable wood floor
	walkableCoords := [][2]int{
		{4, 4}, {4, 5}, {4, 6},
		{5, 5},
		{6, 4}, {6, 5}, {6, 6},
		{7, 5}, {7, 6},
		{8, 6},
	}
	for _, coord := range walkableCoords {
		c, r := coord[0], coord[1]
		tRoom := m3d.findTileAt(c, r)
		if tRoom == nil || tRoom.WallCount() != 0 || tRoom.IsSolid {
			t.Errorf("interior room tile at (%d, %d) expected open walkable wood floor, got walls=%d, solid=%v", c, r, tRoom.WallCount(), tRoom.IsSolid)
		}
	}

	// Verify staircase at (8, 5) and loft platforms at (7, 4), (8, 4)
	tStairs := m3d.findTileAt(8, 5)
	if tStairs == nil || tStairs.Type != TileStairs || !tStairs.IsStairs {
		t.Errorf("expected staircase at (8, 5), got %v", tStairs)
	}
	tLoft7 := m3d.findTileAt(7, 4)
	if tLoft7 == nil || !tLoft7.IsSolid {
		t.Errorf("expected solid loft platform at (7, 4)")
	}
	tLoft8 := m3d.findTileAt(8, 4)
	if tLoft8 == nil || !tLoft8.IsSolid {
		t.Errorf("expected solid loft platform at (8, 4)")
	}

	// Wall Thickness check: All wall segments in the house must be ~0.2m thick
	for r := 3; r <= 7; r++ {
		for c := 3; c <= 9; c++ {
			tCheck := m3d.findTileAt(c, r)
			for j, ws := range tCheck.WallSegments {
				thickness := ws.Size.Z
				if ws.Edge == WallEdgeEast || ws.Edge == WallEdgeWest {
					thickness = ws.Size.X
				}
				if thickness < 0.19 || thickness > 0.21 {
					t.Errorf("house wall at (%d, %d) seg %d expected ~0.2m thickness, got %f", c, r, j, thickness)
				}
			}
		}
	}

	// Check that inside the house there are no spawned props (just floors and walls!)
	for _, cube := range m3d.SpawnedCubes {
		pos := cube.GetPosition()
		gridC := int(pos.X + 8.0)
		gridR := int(pos.Z + 8.0)
		if gridC >= 3 && gridC <= 9 && gridR >= 3 && gridR <= 7 {
			t.Errorf("expected no props inside the house, found %s at grid (%d, %d)", cube.Name, gridC, gridR)
		}
	}

	// 3. Other map features: Watchtowers, Campfire, River Bridge
	tTowerNW := m3d.findTileAt(2, 2)
	if tTowerNW == nil || tTowerNW.Type != TilePillar {
		t.Errorf("expected Watchtower Pillar at (2, 2)")
	}
	tCampfire := m3d.findTileAt(6, 9)
	if tCampfire == nil || tCampfire.FloorName != "Lava Magma Pool" && !strings.Contains(strings.ToLower(tCampfire.Name), "campfire") {
		t.Errorf("expected campfire at (6, 9), got name='%s'", tCampfire.Name)
	}
	tBridge := m3d.findTileAt(14, 8)
	if tBridge == nil || tBridge.Type != TileFloor || tBridge.IsSolid {
		t.Errorf("expected walkable wood bridge at (14, 8)")
	}

	// River Water at (15, 0)
	tRiver := m3d.findTileAt(15, 0)
	if tRiver == nil || tRiver.Type != TileWater {
		t.Errorf("expected river water at (15, 0), got %+v", tRiver)
	}

	// 4. Verify Player Spawn
	var playerSpawn *MapSpawnPoint
	for i := range m3d.Spawns {
		if m3d.Spawns[i].Type == "player" {
			playerSpawn = &m3d.Spawns[i]
			break
		}
	}
	if playerSpawn == nil {
		t.Fatalf("expected player spawn in conestoga map")
	}
	if playerSpawn.Name != "PlayerPioneer" {
		t.Errorf("expected player name 'PlayerPioneer', got '%s'", playerSpawn.Name)
	}

	// 5. Verify Zombie Spawners
	zombieCount := 0
	for _, sp := range m3d.Spawns {
		if sp.Type == "zombie" {
			zombieCount++
		}
	}
	if zombieCount < 5 {
		t.Errorf("expected at least 5 zombie spawns in conestoga map, got %d", zombieCount)
	}

	// 6. Verify Pickups
	healthFound := false
	ammoFound := false
	itemFound := false
	for _, sp := range m3d.Spawns {
		if sp.Type == "health" {
			healthFound = true
		} else if sp.Type == "ammo" {
			ammoFound = true
		} else if sp.Type == "item" {
			itemFound = true
		}
	}
	if !healthFound || !ammoFound || !itemFound {
		t.Errorf("expected health, ammo, and item pickups in conestoga map (health=%v, ammo=%v, item=%v)", healthFound, ammoFound, itemFound)
	}

	// 6. Verify Interactive Physics Cubes
	if len(m3d.SpawnedCubes) < 4 {
		t.Errorf("expected at least 4 interactive cubes in conestoga map, got %d", len(m3d.SpawnedCubes))
	}
	var toolbox *cube.RainbowCube
	for _, c := range m3d.SpawnedCubes {
		if strings.Contains(c.Name, "Toolbox") {
			toolbox = c
			break
		}
	}
	if toolbox == nil {
		t.Errorf("expected 'ConestogaToolbox' spawned cube")
	} else if toolbox.GetMass() != 15.0 {
		t.Errorf("expected toolbox mass 15.0, got %f", toolbox.GetMass())
	}

	// 7. Verify Solid collision boxes (walls, watchtowers, barricades)
	solidBoxes := m3d.GetSolidBoxes()
	if len(solidBoxes) < 20 {
		t.Errorf("expected at least 20 solid collision boxes in conestoga map, got %d", len(solidBoxes))
	}

	// 8. Verify JSON export
	jsonBytes, err := m3d.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed on conestoga map: %v", err)
	}
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, "Fort Conestoga - Frontier Outpost") {
		t.Errorf("expected JSON to contain map title")
	}
	if !strings.Contains(jsonStr, `"has_flooring": true`) {
		t.Errorf("expected JSON to contain has_flooring")
	}
}

func TestPerimeterWallsOrientationNotSideways(t *testing.T) {
	m, err := LoadTMX("../../maps/zombie_outpost.tmx")
	if err != nil {
		t.Fatalf("LoadTMX error: %v", err)
	}
	m3d := NewMap3D(m, 1.0, -2.2)

	// Verify Row 0 (North perimeter): all walls must run along X (Width=1.0, Depth=0.2, WallEdgeNorth)
	for c := 1; c <= 5; c++ {
		tile := m3d.findTileAt(c, 0)
		if tile == nil || len(tile.WallSegments) == 0 {
			t.Fatalf("expected wall at (%d, 0)", c)
		}
		if tile.Walls != WallEdgeNorth {
			t.Errorf("tile at (%d, 0): expected WallEdgeNorth (1), got %d", c, tile.Walls)
		}
		if tile.WallSegments[0].Size.X != 1.0 || tile.WallSegments[0].Size.Z != WallThickness {
			t.Errorf("tile at (%d, 0) sideways! size: %v", c, tile.WallSegments[0].Size)
		}
	}
	for c := 8; c <= 12; c++ {
		tile := m3d.findTileAt(c, 0)
		if tile == nil || len(tile.WallSegments) == 0 {
			t.Fatalf("expected wall at (%d, 0)", c)
		}
		if tile.Walls != WallEdgeNorth {
			t.Errorf("tile at (%d, 0): expected WallEdgeNorth (1), got %d", c, tile.Walls)
		}
		if tile.WallSegments[0].Size.X != 1.0 || tile.WallSegments[0].Size.Z != WallThickness {
			t.Errorf("tile at (%d, 0) sideways! size: %v", c, tile.WallSegments[0].Size)
		}
	}

	// Verify Row 13 (South perimeter): all walls must run along X (Width=1.0, Depth=0.2, WallEdgeSouth)
	for c := 1; c <= 5; c++ {
		tile := m3d.findTileAt(c, 13)
		if tile == nil || len(tile.WallSegments) == 0 {
			t.Fatalf("expected wall at (%d, 13)", c)
		}
		if tile.Walls != WallEdgeSouth {
			t.Errorf("tile at (%d, 13): expected WallEdgeSouth (2), got %d", c, tile.Walls)
		}
		if tile.WallSegments[0].Size.X != 1.0 || tile.WallSegments[0].Size.Z != WallThickness {
			t.Errorf("tile at (%d, 13) sideways! size: %v", c, tile.WallSegments[0].Size)
		}
	}
	for c := 8; c <= 12; c++ {
		tile := m3d.findTileAt(c, 13)
		if tile == nil || len(tile.WallSegments) == 0 {
			t.Fatalf("expected wall at (%d, 13)", c)
		}
		if tile.Walls != WallEdgeSouth {
			t.Errorf("tile at (%d, 13): expected WallEdgeSouth (2), got %d", c, tile.Walls)
		}
		if tile.WallSegments[0].Size.X != 1.0 || tile.WallSegments[0].Size.Z != WallThickness {
			t.Errorf("tile at (%d, 13) sideways! size: %v", c, tile.WallSegments[0].Size)
		}
	}

	// Verify Col 0 (West perimeter): all walls must run along Z (Width=0.2, Depth=1.0, WallEdgeWest)
	for r := 1; r <= 5; r++ {
		tile := m3d.findTileAt(0, r)
		if tile == nil || len(tile.WallSegments) == 0 {
			t.Fatalf("expected wall at (0, %d)", r)
		}
		if tile.Walls != WallEdgeWest {
			t.Errorf("tile at (0, %d): expected WallEdgeWest (8), got %d", r, tile.Walls)
		}
		if tile.WallSegments[0].Size.X != WallThickness || tile.WallSegments[0].Size.Z != 1.0 {
			t.Errorf("tile at (0, %d) sideways! size: %v", r, tile.WallSegments[0].Size)
		}
	}
	for r := 8; r <= 12; r++ {
		tile := m3d.findTileAt(0, r)
		if tile == nil || len(tile.WallSegments) == 0 {
			t.Fatalf("expected wall at (0, %d)", r)
		}
		if tile.Walls != WallEdgeWest {
			t.Errorf("tile at (0, %d): expected WallEdgeWest (8), got %d", r, tile.Walls)
		}
		if tile.WallSegments[0].Size.X != WallThickness || tile.WallSegments[0].Size.Z != 1.0 {
			t.Errorf("tile at (0, %d) sideways! size: %v", r, tile.WallSegments[0].Size)
		}
	}

	// Verify Col 13 (East perimeter): all walls must run along Z (Width=0.2, Depth=1.0, WallEdgeEast)
	for r := 1; r <= 5; r++ {
		tile := m3d.findTileAt(13, r)
		if tile == nil || len(tile.WallSegments) == 0 {
			t.Fatalf("expected wall at (13, %d)", r)
		}
		if tile.Walls != WallEdgeEast {
			t.Errorf("tile at (13, %d): expected WallEdgeEast (4), got %d", r, tile.Walls)
		}
		if tile.WallSegments[0].Size.X != WallThickness || tile.WallSegments[0].Size.Z != 1.0 {
			t.Errorf("tile at (13, %d) sideways! size: %v", r, tile.WallSegments[0].Size)
		}
	}
	for r := 8; r <= 12; r++ {
		tile := m3d.findTileAt(13, r)
		if tile == nil || len(tile.WallSegments) == 0 {
			t.Fatalf("expected wall at (13, %d)", r)
		}
		if tile.Walls != WallEdgeEast {
			t.Errorf("tile at (13, %d): expected WallEdgeEast (4), got %d", r, tile.Walls)
		}
		if tile.WallSegments[0].Size.X != WallThickness || tile.WallSegments[0].Size.Z != 1.0 {
			t.Errorf("tile at (13, %d) sideways! size: %v", r, tile.WallSegments[0].Size)
		}
	}
}

func TestTiledRotationAndFlipFlags(t *testing.T) {
	// Standard GID 2 with Tiled flip/rotation flags:
	// 0 deg: 2
	// 90 deg CW (H + D): 2 | 0x80000000 | 0x20000000 = 2684354562
	// 180 deg (H + V): 2 | 0x80000000 | 0x40000000 = 3221225474
	// 270 deg CW (V + D): 2 | 0x40000000 | 0x20000000 = 1610612738
	xmlData := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="4" height="1" tilewidth="32" tileheight="32">
 <layer id="1" name="RotatedWalls" width="4" height="1">
  <data encoding="csv">
%d,%d,%d,%d
  </data>
 </layer>
</map>`,
		2,
		2|TiledFlipHorizontal|TiledFlipDiagonal,
		2|TiledFlipHorizontal|TiledFlipVertical,
		2|TiledFlipVertical|TiledFlipDiagonal,
	)

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseTMX failed: %v", err)
	}

	layer := m.GetLayerByName("RotatedWalls")
	if layer == nil {
		t.Fatalf("RotatedWalls layer missing")
	}

	// Verify parsed GIDs and FlipFlags
	if layer.Tiles[0][0] != 2 || layer.FlipFlags[0][0] != 0 {
		t.Errorf("expected 0 deg tile GID=2, flags=0")
	}
	if layer.Tiles[0][1] != 2 || layer.FlipFlags[0][1] != (TiledFlipHorizontal|TiledFlipDiagonal) {
		t.Errorf("expected 90 deg CW tile GID=2, flags=0xA0000000")
	}
	if layer.Tiles[0][2] != 2 || layer.FlipFlags[0][2] != (TiledFlipHorizontal|TiledFlipVertical) {
		t.Errorf("expected 180 deg tile GID=2, flags=0xC0000000")
	}
	if layer.Tiles[0][3] != 2 || layer.FlipFlags[0][3] != (TiledFlipVertical|TiledFlipDiagonal) {
		t.Errorf("expected 270 deg CW tile GID=2, flags=0x60000000")
	}

	m3d := NewMap3D(m, 1.0, -2.2)
	if len(m3d.Tiles) != 4 {
		t.Fatalf("expected 4 tiles in Map3D, got %d", len(m3d.Tiles))
	}

	// Tile 0 (0 deg): default South wall
	if m3d.Tiles[0].Walls != WallEdgeSouth {
		t.Errorf("tile 0 (0 deg): expected WallEdgeSouth (2), got %d", m3d.Tiles[0].Walls)
	}
	if m3d.Tiles[0].Size.X != 1.0 || m3d.Tiles[0].Size.Z != WallThickness {
		t.Errorf("tile 0 size: expected (1.0, 2.2, 0.2), got %v", m3d.Tiles[0].Size)
	}

	// Tile 1 (90 deg CW): rotated South -> West wall
	if m3d.Tiles[1].Walls != WallEdgeWest {
		t.Errorf("tile 1 (90 deg CW): expected WallEdgeWest (8), got %d", m3d.Tiles[1].Walls)
	}
	if m3d.Tiles[1].Size.X != WallThickness || m3d.Tiles[1].Size.Z != 1.0 {
		t.Errorf("tile 1 size: expected (0.2, 2.2, 1.0), got %v", m3d.Tiles[1].Size)
	}

	// Tile 2 (180 deg): rotated South -> North wall
	if m3d.Tiles[2].Walls != WallEdgeNorth {
		t.Errorf("tile 2 (180 deg): expected WallEdgeNorth (1), got %d", m3d.Tiles[2].Walls)
	}
	if m3d.Tiles[2].Size.X != 1.0 || m3d.Tiles[2].Size.Z != WallThickness {
		t.Errorf("tile 2 size: expected (1.0, 2.2, 0.2), got %v", m3d.Tiles[2].Size)
	}

	// Tile 3 (270 deg CW): rotated South -> East wall
	if m3d.Tiles[3].Walls != WallEdgeEast {
		t.Errorf("tile 3 (270 deg CW): expected WallEdgeEast (4), got %d", m3d.Tiles[3].Walls)
	}
	if m3d.Tiles[3].Size.X != WallThickness || m3d.Tiles[3].Size.Z != 1.0 {
		t.Errorf("tile 3 size: expected (0.2, 2.2, 1.0), got %v", m3d.Tiles[3].Size)
	}
}

func TestStaircaseTileCreation(t *testing.T) {
	// Build minimal map with staircases in each cardinal direction
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="4" height="1" tilewidth="32" tileheight="32">
 <layer id="1" name="StairsLayer" width="4" height="1">
  <data encoding="csv">
25,26,27,28
  </data>
 </layer>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseTMX failed: %v", err)
	}

	m3d := NewMap3D(m, 1.0, -2.2)
	if len(m3d.Tiles) != 4 {
		t.Fatalf("expected 4 tiles, got %d", len(m3d.Tiles))
	}

	dirs := []StairDirection{StairAscendNorth, StairAscendSouth, StairAscendEast, StairAscendWest}
	for i, dir := range dirs {
		tile := m3d.Tiles[i]
		if tile.Type != TileStairs {
			t.Errorf("tile %d: expected TileStairs (9), got %d", i, tile.Type)
		}
		if !tile.IsStairs {
			t.Errorf("tile %d: expected IsStairs true", i)
		}
		if !tile.IsSolid {
			t.Errorf("tile %d: expected IsSolid true", i)
		}
		if tile.StairDir != dir {
			t.Errorf("tile %d: expected StairDir %v, got %v", i, dir, tile.StairDir)
		}
		if tile.StepCount != 4 {
			t.Errorf("tile %d: expected 4 steps, got %d", i, tile.StepCount)
		}
		if len(tile.Steps) != 4 {
			t.Errorf("tile %d: expected 4 StairStep elements, got %d", i, len(tile.Steps))
		}
		if len(tile.StepBoxes) != 4 {
			t.Errorf("tile %d: expected 4 StepBoxes, got %d", i, len(tile.StepBoxes))
		}

		// Verify step rises are within StepHeight threshold (<= 0.65m)
		risePerStep := tile.StairHeight / float32(tile.StepCount)
		if risePerStep > 0.65 {
			t.Errorf("tile %d: rise per step %.2f exceeds StepHeight (0.65)", i, risePerStep)
		}
	}

	// Verify GetSolidBoxes returns all 4 steps per staircase (4 tiles * 4 steps = 16 solid boxes)
	solidBoxes := m3d.GetSolidBoxes()
	if len(solidBoxes) != 16 {
		t.Errorf("expected 16 solid boxes from 4 staircases, got %d", len(solidBoxes))
	}

	// Raycast against North ascending stair (tile 0)
	// Tile 0 position: X = (0 - 2.0 + 0.5) * 1.0 = -1.5, Z = 0
	ray := rl.Ray{
		Position:  rl.NewVector3(-1.5, 2.0, 0.0),
		Direction: rl.NewVector3(0, -1, 0),
	}
	hit, dist := m3d.Tiles[0].RayIntersects(ray)
	if !hit {
		t.Errorf("expected ray to hit staircase tile 0")
	}
	if dist <= 0 {
		t.Errorf("expected positive ray distance, got %f", dist)
	}
}

func TestStaircaseSteppingLocomotion(t *testing.T) {
	// Construct a 1x1 map with a North-ascending staircase at origin (0, 0)
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" width="1" height="1" tilewidth="32" tileheight="32">
 <layer id="1" name="StairsLayer" width="1" height="1">
  <data encoding="csv">
25
  </data>
 </layer>
</map>`

	m, err := ParseTMX([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseTMX failed: %v", err)
	}

	baseY := float32(-2.2)
	m3d := NewMap3D(m, 1.0, baseY)
	solidBoxes := m3d.GetSolidBoxes()

	// Staircase is centered at (0, baseY + 1.08/2, 0).
	// Z bounds are [-0.5, +0.5].
	// North is -Z (high end), South is +Z (low end / entrance).
	// Floor level = baseY + FloorHeight = -2.2 + 0.08 = -2.12.
	// Step 0: Z in [+0.25, +0.50], top Y = -2.12 + 0.25 = -1.87
	// Step 1: Z in [ 0.00, +0.25], top Y = -2.12 + 0.50 = -1.62
	// Step 2: Z in [-0.25,  0.00], top Y = -2.12 + 0.75 = -1.37
	// Step 3: Z in [-0.50, -0.25], top Y = -2.12 + 1.00 = -1.12

	// Position player just South of the stairs on the ground
	startPos := rl.NewVector3(0.0, -2.12, 0.85)
	fps := camera.NewFPSController(startPos, baseY)
	fps.Position.Y = -2.12

	// Walk forward into the staircase (in -Z direction toward North)
	// Simulate 12 small steps walking North
	stepDeltaZ := float32(-0.1)
	for i := 0; i < 12; i++ {
		fps.MoveWithCollisions(rl.NewVector3(0, 0, stepDeltaZ), 0.016, solidBoxes)
	}

	// Player should have stepped up onto step 3 (highest step, Y around -1.12)
	expectedHighY := float32(-1.12)
	if fps.Position.Y < expectedHighY-0.05 {
		t.Errorf("expected player to walk up staircase to Y >= %.2f, but reached Y = %.2f (pos: %v)", expectedHighY-0.05, fps.Position.Y, fps.Position)
	}
	if !fps.IsGrounded {
		t.Errorf("expected player to be grounded at top of stairs")
	}

	// Now walk backwards down the staircase (in +Z direction toward South)
	for i := 0; i < 12; i++ {
		fps.MoveWithCollisions(rl.NewVector3(0, 0, -stepDeltaZ), 0.016, solidBoxes)
	}

	// Player should have stepped down cleanly back to ground level
	expectedLowY := float32(-2.12)
	if fps.Position.Y > expectedLowY+0.05 {
		t.Errorf("expected player to walk down staircase to Y <= %.2f, but remained at Y = %.2f", expectedLowY+0.05, fps.Position.Y)
	}
}

func TestStaircaseDihedralRotation(t *testing.T) {
	// Base direction: Ascend North
	baseDir := StairAscendNorth

	// 1. 0 deg: no rotation
	if RotateStairDirection(baseDir, 0) != StairAscendNorth {
		t.Errorf("0 deg: expected North")
	}

	// 2. 90 deg CW: Diagonal | Horizontal
	rot90 := RotateStairDirection(baseDir, TiledFlipDiagonal|TiledFlipHorizontal)
	if rot90 != StairAscendEast {
		t.Errorf("90 deg CW: expected East, got %v", rot90)
	}

	// 3. 180 deg: Horizontal | Vertical
	rot180 := RotateStairDirection(baseDir, TiledFlipHorizontal|TiledFlipVertical)
	if rot180 != StairAscendSouth {
		t.Errorf("180 deg: expected South, got %v", rot180)
	}

	// 4. 270 deg CW: Diagonal | Vertical
	rot270 := RotateStairDirection(baseDir, TiledFlipDiagonal|TiledFlipVertical)
	if rot270 != StairAscendWest {
		t.Errorf("270 deg CW: expected West, got %v", rot270)
	}
}

func TestConestogaHouseStartupMapWithStairs(t *testing.T) {
	m, err := LoadTMX("../../maps/conestoga.tmx")
	if err != nil {
		t.Fatalf("failed to load maps/conestoga.tmx: %v", err)
	}

	m3d := NewMap3D(m, 1.0, -2.2)
	if len(m3d.Tiles) == 0 {
		t.Fatalf("expected tiles loaded from conestoga.tmx")
	}

	// Find the staircase at Col 8, Row 5 (GID 25)
	stairTile := m3d.findTileAt(8, 5)
	if stairTile == nil {
		t.Fatalf("expected staircase tile at (8, 5)")
	}
	if stairTile.Type != TileStairs {
		t.Errorf("expected TileStairs at (8, 5), got %d", stairTile.Type)
	}
	if !stairTile.IsStairs {
		t.Errorf("expected IsStairs == true at (8, 5)")
	}
	if stairTile.StairDir != StairAscendNorth {
		t.Errorf("expected StairAscendNorth at (8, 5), got %v", stairTile.StairDir)
	}
	if len(stairTile.Steps) != 4 {
		t.Errorf("expected 4 steps at (8, 5), got %d", len(stairTile.Steps))
	}
	if !stairTile.HasFlooring {
		t.Errorf("expected staircase to have underlying wood plank flooring")
	}

	// Find the loft platforms at Col 7 & 8, Row 4 (GID 29)
	loftTile8 := m3d.findTileAt(8, 4)
	if loftTile8 == nil {
		t.Fatalf("expected loft tile at (8, 4)")
	}
	if !loftTile8.IsSolid {
		t.Errorf("expected solid loft platform at (8, 4)")
	}

	loftTile7 := m3d.findTileAt(7, 4)
	if loftTile7 == nil {
		t.Fatalf("expected loft tile at (7, 4)")
	}
	if !loftTile7.IsSolid {
		t.Errorf("expected solid loft platform at (7, 4)")
	}

	// Verify player can walk from Row 6 into the stairs at Row 5, and step onto the loft at Row 4
	solidBoxes := m3d.GetSolidBoxes()
	// World coordinate for Col 8 is (8 - 8 + 0.5) * 1.0 = 0.5
	// World coordinate for Row 6 is (6 - 8 + 0.5) * 1.0 = -1.5
	// World coordinate for Row 5 is (5 - 8 + 0.5) * 1.0 = -2.5 (staircase)
	// World coordinate for Row 4 is (4 - 8 + 0.5) * 1.0 = -3.5 (loft)
	fps := camera.NewFPSController(rl.NewVector3(0.5, -2.12, -1.8), -2.2)
	fps.Position.Y = -2.12

	// Walk North toward staircase and loft (-Z direction)
	for i := 0; i < 20; i++ {
		fps.MoveWithCollisions(rl.NewVector3(0, 0, -0.1), 0.016, solidBoxes)
	}

	// Player should have reached the loft platform (elevation ~ -1.12)
	if fps.Position.Y < -1.2 {
		t.Errorf("expected player to reach loft platform at Y ~ -1.12, got Y = %f", fps.Position.Y)
	}
	if fps.Position.Z > -2.6 {
		t.Errorf("expected player to advance North into loft, got Z = %f", fps.Position.Z)
	}
}
