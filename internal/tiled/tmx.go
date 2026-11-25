package tiled

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"zombie-project/internal/paths"
)

// Map represents the top-level <map> element of a Tiled TMX file.
type Map struct {
	XMLName      xml.Name      `xml:"map"`
	Version      string        `xml:"version,attr"`
	TiledVersion string        `xml:"tiledversion,attr"`
	Orientation  string        `xml:"orientation,attr"`
	RenderOrder  string        `xml:"renderorder,attr"`
	Width        int           `xml:"width,attr"`
	Height       int           `xml:"height,attr"`
	TileWidth    int           `xml:"tilewidth,attr"`
	TileHeight   int           `xml:"tileheight,attr"`
	Properties   []Property    `xml:"properties>property"`
	Tilesets     []Tileset     `xml:"tileset"`
	Layers       []Layer       `xml:"layer"`
	ObjectGroups []ObjectGroup `xml:"objectgroup"`

	FilePath string `xml:"-"`
}

// Property represents a custom <property> inside <properties>.
type Property struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
	Text  string `xml:",chardata"`
}

// GetVal returns the attribute value or chardata value.
func (p Property) GetVal() string {
	if p.Value != "" {
		return p.Value
	}
	return strings.TrimSpace(p.Text)
}

// Tileset represents a <tileset> element in TMX.
type Tileset struct {
	FirstGID   uint32        `xml:"firstgid,attr"`
	Name       string        `xml:"name,attr"`
	TileWidth  int           `xml:"tilewidth,attr"`
	TileHeight int           `xml:"tileheight,attr"`
	TileCount  int           `xml:"tilecount,attr"`
	Columns    int           `xml:"columns,attr"`
	Source     string        `xml:"source,attr"` // If external tsx
	Image      Image         `xml:"image"`
	Tiles      []TilesetTile `xml:"tile"`
}

// Image represents an <image> in a tileset.
type Image struct {
	Source string `xml:"source,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

// TilesetTile defines per-tile properties within a tileset.
type TilesetTile struct {
	ID         uint32     `xml:"id,attr"`
	Type       string     `xml:"type,attr"`
	Properties []Property `xml:"properties>property"`
}

// Tiled GID flip and rotation flags (highest bits of 32-bit GID)
const (
	TiledFlipHorizontal   uint32 = 0x80000000 // bit 31
	TiledFlipVertical     uint32 = 0x40000000 // bit 30
	TiledFlipDiagonal     uint32 = 0x20000000 // bit 29
	TiledFlipHexagonal120 uint32 = 0x10000000 // bit 28
	TiledGIDMask          uint32 = 0x1FFFFFFF
)

// Layer represents a <layer> tile layer.
type Layer struct {
	ID         int        `xml:"id,attr"`
	Name       string     `xml:"name,attr"`
	Width      int        `xml:"width,attr"`
	Height     int        `xml:"height,attr"`
	Opacity    float32    `xml:"opacity,attr"`
	Visible    *int       `xml:"visible,attr"`
	Properties []Property `xml:"properties>property"`
	Data       Data       `xml:"data"`

	// Parsed 2D grid of Tile GIDs: Tiles[y][x]
	Tiles [][]uint32 `xml:"-"`
	// Parsed 2D grid of Tiled flip/rotation flags: FlipFlags[y][x]
	FlipFlags [][]uint32 `xml:"-"`
}

// IsVisible returns true unless explicitly set to 0.
func (l Layer) IsVisible() bool {
	if l.Visible != nil && *l.Visible == 0 {
		return false
	}
	return true
}

// GetProperty retrieves the string value of a named custom property on the Layer.
func (l Layer) GetProperty(name, defaultVal string) string {
	for _, p := range l.Properties {
		if strings.EqualFold(p.Name, name) {
			return p.GetVal()
		}
	}
	return defaultVal
}

// Data represents <data> containing tile indices.
type Data struct {
	Encoding    string     `xml:"encoding,attr"`
	Compression string     `xml:"compression,attr"`
	RawData     string     `xml:",chardata"`
	XMLTiles    []XMLTile  `xml:"tile"`
}

// XMLTile represents a `<tile gid="..."/>` element in unencoded data.
type XMLTile struct {
	GID uint32 `xml:"gid,attr"`
}

// ObjectGroup represents an <objectgroup> layer.
type ObjectGroup struct {
	ID         int        `xml:"id,attr"`
	Name       string     `xml:"name,attr"`
	Color      string     `xml:"color,attr"`
	Opacity    float32    `xml:"opacity,attr"`
	Visible    *int       `xml:"visible,attr"`
	Properties []Property `xml:"properties>property"`
	Objects    []Object   `xml:"object"`
}

// Object represents an <object> inside <objectgroup>.
type Object struct {
	ID         int        `xml:"id,attr"`
	Name       string     `xml:"name,attr"`
	Type       string     `xml:"type,attr"`
	X          float32    `xml:"x,attr"`
	Y          float32    `xml:"y,attr"`
	Width      float32    `xml:"width,attr"`
	Height     float32    `xml:"height,attr"`
	Rotation   float32    `xml:"rotation,attr"`
	GID        uint32     `xml:"gid,attr"`
	Visible    *int       `xml:"visible,attr"`
	Properties []Property `xml:"properties>property"`
}

// GetProperty returns the string value of a named property or defaultValue.
func (obj Object) GetProperty(name, defaultVal string) string {
	for _, p := range obj.Properties {
		if strings.EqualFold(p.Name, name) {
			return p.GetVal()
		}
	}
	return defaultVal
}

// LoadTMX reads and parses a .tmx file from disk.
func LoadTMX(filePath string) (*Map, error) {
	resolvedPath := paths.Resolve(filePath)
	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open TMX file (%s): %w", resolvedPath, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read TMX file (%s): %w", resolvedPath, err)
	}

	m, err := ParseTMX(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TMX (%s): %w", resolvedPath, err)
	}

	m.FilePath = resolvedPath
	return m, nil
}

// ParseTMX parses raw XML bytes into a Map and decodes all layer tiles.
func ParseTMX(xmlBytes []byte) (*Map, error) {
	var m Map
	if err := xml.Unmarshal(xmlBytes, &m); err != nil {
		return nil, err
	}

	// Post-process layers into 2D tile arrays
	for i := range m.Layers {
		layer := &m.Layers[i]
		if err := decodeLayerData(layer, m.Width, m.Height); err != nil {
			return nil, fmt.Errorf("layer %q decode error: %w", layer.Name, err)
		}
	}

	return &m, nil
}

func decodeLayerData(layer *Layer, mapW, mapH int) error {
	w := layer.Width
	if w <= 0 {
		w = mapW
	}
	h := layer.Height
	if h <= 0 {
		h = mapH
	}

	layer.Tiles = make([][]uint32, h)
	layer.FlipFlags = make([][]uint32, h)
	for y := 0; y < h; y++ {
		layer.Tiles[y] = make([]uint32, w)
		layer.FlipFlags[y] = make([]uint32, w)
	}

	encoding := strings.ToLower(strings.TrimSpace(layer.Data.Encoding))
	switch encoding {
	case "csv":
		clean := strings.TrimSpace(layer.Data.RawData)
		if clean == "" {
			return nil
		}
		// Split by comma or newline
		tokens := strings.FieldsFunc(clean, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		})

		idx := 0
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			val, err := strconv.ParseUint(token, 10, 32)
			if err != nil {
				continue
			}
			x := idx % w
			y := idx / w
			if y < h && x < w {
				rawVal := uint32(val)
				gid := rawVal & TiledGIDMask
				flags := rawVal & 0xE0000000
				layer.Tiles[y][x] = gid
				layer.FlipFlags[y][x] = flags
			}
			idx++
		}
	default:
		// Check for <tile gid="..."/> elements
		if len(layer.Data.XMLTiles) > 0 {
			for idx, xt := range layer.Data.XMLTiles {
				x := idx % w
				y := idx / w
				if y < h && x < w {
					rawVal := xt.GID
					layer.Tiles[y][x] = rawVal & TiledGIDMask
					layer.FlipFlags[y][x] = rawVal & 0xE0000000
				}
			}
		}
	}

	return nil
}

// GetProperty retrieves the value of a property on the Map.
func (m *Map) GetProperty(name, defaultVal string) string {
	for _, p := range m.Properties {
		if strings.EqualFold(p.Name, name) {
			return p.GetVal()
		}
	}
	return defaultVal
}

// GetLayerByName searches for a tile layer by name.
func (m *Map) GetLayerByName(name string) *Layer {
	for i := range m.Layers {
		if strings.EqualFold(m.Layers[i].Name, name) {
			return &m.Layers[i]
		}
	}
	return nil
}

// GetObjectGroupByName searches for an object group by name.
func (m *Map) GetObjectGroupByName(name string) *ObjectGroup {
	for i := range m.ObjectGroups {
		if strings.EqualFold(m.ObjectGroups[i].Name, name) {
			return &m.ObjectGroups[i]
		}
	}
	return nil
}
