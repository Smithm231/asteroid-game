package model

import "slices"

// Tile is one hex of a rock's surface.
type Tile struct {
	// Terrain is the surface type. TerrainVoid tiles are not part of the
	// rock.
	Terrain Terrain
	// Deposit is the raw good under the tile (Ore, Ice, Silica or Lumen), or
	// GoodNone. A depleted tile keeps its Deposit kind with Amount 0, and
	// the zero Tile reads as "Ore, amount 0": always test HasDeposit rather
	// than comparing Deposit with GoodNone.
	Deposit Good
	// Amount is the number of deposit units left.
	Amount float64 `json:",omitempty"`
	// Known is the set of factions that have revealed this tile and can
	// therefore see its deposit (or see that it has none).
	Known FactionMask `json:",omitempty"`
	// Rubble marks the wreckage of a destroyed building. It is cosmetic,
	// except that building over it clears it and grants RubbleAlloyBonus.
	Rubble bool `json:",omitempty"`
	// Building is the ID of the building standing on the tile, or 0. It is
	// maintained by Rock.AddBuilding and Rock.RemoveBuilding.
	Building BuildingID `json:",omitempty"`
}

// HasDeposit reports whether the tile holds a deposit with material left.
func (t *Tile) HasDeposit() bool {
	return t != nil && t.Deposit.Valid() && t.Amount > 0
}

// KnownTo reports whether faction f has revealed the tile.
func (t *Tile) KnownTo(f FactionID) bool { return t != nil && t.Known.Has(f) }

// Reveal marks the tile as revealed to faction f. It reports whether the
// tile was hidden from f before.
func (t *Tile) Reveal(f FactionID) bool {
	if t == nil || !f.Valid() || t.Known.Has(f) {
		return false
	}
	t.Known.Set(f)
	return true
}

// Buildable reports whether a building may stand here as far as the tile
// itself is concerned: Regolith with no building on it. (Adjacency, ownership
// and cost are the construction system's business.)
func (t *Tile) Buildable() bool {
	return t != nil && t.Terrain == TerrainRegolith && t.Building == 0
}

// Salvage is the cargo of a derelict hulk drifting in orbit of a rock.
type Salvage struct {
	// Goods are the salvageable goods (some Alloy and Parts).
	Goods Stock
	// Scrip is the salvageable currency.
	Scrip float64
	// SpawnTick is the tick at which the hulk appeared.
	SpawnTick int
	// ExpireTick is the tick at which the hulk drifts away unclaimed.
	ExpireTick int
	// Claimant is the faction currently accumulating claim time (the rock's
	// owner, or the first faction with a fleet in orbit), or NoFaction.
	Claimant FactionID
	// ClaimSeconds is how long, in game-seconds, Claimant has held its
	// claim; at SalvageClaimSeconds the salvage is collected.
	ClaimSeconds float64
}

// Rock is one body of the belt: an orbit, a hex map, and possibly a colony.
//
// Tiles are stored densely for the square that bounds the hex disc of the
// rock's Radius: tile index = (R+Radius)*(2*Radius+1) + (Q+Radius). Entries
// of the square that lie outside the disc (hex distance > Radius) are unused
// and stay Void; TileAt returns nil for them.
type Rock struct {
	// ID is the rock's identifier and its index in World.Rocks.
	ID RockID
	// Name is the display name.
	Name string
	// Class is the geological class.
	Class RockClass
	// OrbitR is the orbit radius in u.
	OrbitR float64
	// Phase0 is the orbital angle in radians at tick 0.
	Phase0 float64
	// Solar is the sunlight factor, SolarFactor(OrbitR), cached at
	// generation.
	Solar float64
	// Radius is the hex radius of the tile disc (RockRadiusMin..Max).
	Radius int
	// Tiles is the dense tile slice described above; its length is
	// TileCount(Radius).
	Tiles []Tile
	// Owner is the faction whose Keystone stands here, or NoFaction.
	// Owner != NoFaction exactly when Colony != nil.
	Owner FactionID
	// Colony is the colony state while the rock is owned, else nil.
	Colony *Colony
	// Buildings are all buildings on the rock (any state, derelicts
	// included), sorted by ascending ID. Use AddBuilding / RemoveBuilding.
	Buildings []*Building
	// SurveyedBy is the set of factions that have fully surveyed the rock
	// (every tile revealed).
	SurveyedBy FactionMask
	// Salvage is the derelict hulk in orbit, or nil.
	Salvage *Salvage
	// ShapeSeed seeds the procedural look of the rock; the simulation never
	// reads it.
	ShapeSeed uint64
}

// TileCount returns the length of the dense tile slice of a rock of the
// given hex radius: (2*radius+1)^2, or 0 for a negative radius.
func TileCount(radius int) int {
	if radius < 0 {
		return 0
	}
	w := 2*radius + 1
	return w * w
}

// NewRock returns a rock with the given ID and hex radius whose tiles are all
// Void without deposits, with no owner and no buildings. Orbit, name, class
// and terrain are left for the caller to fill in.
func NewRock(id RockID, radius int) *Rock {
	if radius < 0 {
		radius = 0
	}
	r := &Rock{ID: id, Radius: radius, Tiles: make([]Tile, TileCount(radius))}
	for i := range r.Tiles {
		r.Tiles[i].Deposit = GoodNone
	}
	return r
}

// InBounds reports whether h lies within the rock's tile disc (hex distance
// from the centre <= Radius). It says nothing about the terrain.
func (r *Rock) InBounds(h Hex) bool {
	return r != nil && h.Len() <= r.Radius
}

// Index returns the index of hex h in Tiles, or -1 when h is outside the
// tile disc.
func (r *Rock) Index(h Hex) int {
	if !r.InBounds(h) {
		return -1
	}
	return (h.R+r.Radius)*(2*r.Radius+1) + (h.Q + r.Radius)
}

// HexAt is the inverse of Index: it returns the hex stored at tile index i
// and true, or false when i is out of range or belongs to an unused corner
// of the bounding square. Ranging over Tiles and calling HexAt visits the
// rock in the same order as Hexes without allocating.
func (r *Rock) HexAt(i int) (Hex, bool) {
	if r == nil || i < 0 || i >= len(r.Tiles) {
		return Hex{}, false
	}
	w := 2*r.Radius + 1
	h := Hex{Q: i%w - r.Radius, R: i/w - r.Radius}
	if h.Len() > r.Radius {
		return Hex{}, false
	}
	return h, true
}

// TileAt returns the tile at hex h, or nil when h is outside the tile disc.
// The pointer aliases the Tiles slice, so writes through it modify the rock.
func (r *Rock) TileAt(h Hex) *Tile {
	i := r.Index(h)
	if i < 0 || i >= len(r.Tiles) {
		return nil
	}
	return &r.Tiles[i]
}

// InBlob reports whether h is part of the actual rock: inside the tile disc
// and not Void.
func (r *Rock) InBlob(h Hex) bool {
	t := r.TileAt(h)
	return t != nil && t.Terrain != TerrainVoid
}

// Hexes returns every hex of the tile disc (Void ones included) in
// increasing tile-index order: by R, then by Q.
func (r *Rock) Hexes() []Hex {
	if r == nil {
		return nil
	}
	return Hex{}.WithinRadius(r.Radius)
}

// BlobHexes returns every non-Void hex of the rock in increasing tile-index
// order.
func (r *Rock) BlobHexes() []Hex {
	if r == nil {
		return nil
	}
	var out []Hex
	for i := range r.Tiles {
		if r.Tiles[i].Terrain == TerrainVoid {
			continue
		}
		if h, ok := r.HexAt(i); ok {
			out = append(out, h)
		}
	}
	return out
}

// BlobSize returns the number of non-Void tiles of the rock.
func (r *Rock) BlobSize() int {
	if r == nil {
		return 0
	}
	n := 0
	for i := range r.Tiles {
		if r.Tiles[i].Terrain != TerrainVoid {
			if _, ok := r.HexAt(i); ok {
				n++
			}
		}
	}
	return n
}

// Omega returns the rock's angular speed in radians per game-second.
func (r *Rock) Omega() float64 {
	if r == nil {
		return 0
	}
	return OrbitOmega(r.OrbitR)
}

// AngleAt returns the rock's orbital angle in radians at the given tick:
// Phase0 + omega * tick / TicksPerSecond.
func (r *Rock) AngleAt(tick int) float64 {
	if r == nil {
		return 0
	}
	return OrbitAngle(r.OrbitR, r.Phase0, tick)
}

// PosAt returns the rock's belt position at the given tick.
func (r *Rock) PosAt(tick int) Vec2 {
	if r == nil {
		return Vec2{}
	}
	return OrbitPos(r.OrbitR, r.Phase0, tick)
}

// HasColony reports whether the rock is owned (has a Keystone colony).
func (r *Rock) HasColony() bool {
	return r != nil && r.Owner != NoFaction && r.Colony != nil
}

// OwnedBy reports whether faction f owns the rock. It is false for
// NoFaction.
func (r *Rock) OwnedBy(f FactionID) bool {
	return r != nil && f != NoFaction && r.Owner == f
}

// buildingIndex returns the position of building id in the sorted Buildings
// slice and whether it is present.
func (r *Rock) buildingIndex(id BuildingID) (int, bool) {
	return slices.BinarySearchFunc(r.Buildings, id, func(b *Building, id BuildingID) int {
		switch {
		case b.ID < id:
			return -1
		case b.ID > id:
			return 1
		default:
			return 0
		}
	})
}

// BuildingByID returns the building with the given ID on this rock, or nil.
func (r *Rock) BuildingByID(id BuildingID) *Building {
	if r == nil || id <= 0 {
		return nil
	}
	if i, ok := r.buildingIndex(id); ok {
		return r.Buildings[i]
	}
	return nil
}

// BuildingAt returns the building standing on hex h, or nil.
func (r *Rock) BuildingAt(h Hex) *Building {
	t := r.TileAt(h)
	if t == nil || t.Building == 0 {
		return nil
	}
	return r.BuildingByID(t.Building)
}

// AddBuilding puts b on the rock: it inserts b into Buildings keeping the
// slice sorted by ID and records b.ID on the tile at b.Hex. It fails without
// changing anything when b is nil or has no ID (ErrBadBuilding), when b.Hex
// is not a solid tile of the rock (ErrBadHex), when the tile already holds a
// building (ErrTileOccupied) or when the ID is already present
// (ErrBadBuilding). It does not check terrain buildability, ownership or
// cost, and it does not touch Rubble.
func (r *Rock) AddBuilding(b *Building) error {
	if r == nil || b == nil || b.ID <= 0 {
		return ErrBadBuilding
	}
	t := r.TileAt(b.Hex)
	if t == nil || t.Terrain == TerrainVoid {
		return ErrBadHex
	}
	if t.Building != 0 {
		return ErrTileOccupied
	}
	i, found := r.buildingIndex(b.ID)
	if found {
		return ErrBadBuilding
	}
	r.Buildings = slices.Insert(r.Buildings, i, b)
	t.Building = b.ID
	return nil
}

// RemoveBuilding takes the building with the given ID off the rock: it
// removes it from Buildings and clears the tile back-reference. It returns
// the removed building, or nil when there is no such building. It does not
// set Rubble.
func (r *Rock) RemoveBuilding(id BuildingID) *Building {
	if r == nil || id <= 0 {
		return nil
	}
	i, found := r.buildingIndex(id)
	if !found {
		return nil
	}
	b := r.Buildings[i]
	r.Buildings = slices.Delete(r.Buildings, i, i+1)
	if t := r.TileAt(b.Hex); t != nil && t.Building == id {
		t.Building = 0
	}
	return b
}

// CountKind returns the number of buildings of the given kind on the rock.
// With onlyActive it counts only operational ones (completed and enabled,
// see Building.Operational); otherwise it counts every state, including
// constructions and derelicts.
func (r *Rock) CountKind(kind BuildingKind, onlyActive bool) int {
	if r == nil {
		return 0
	}
	n := 0
	for _, b := range r.Buildings {
		if b.Kind == kind && (!onlyActive || b.Operational()) {
			n++
		}
	}
	return n
}

// FirstOfKind returns the lowest-ID building of the given kind on the rock,
// or nil. With onlyActive only operational buildings are considered.
func (r *Rock) FirstOfKind(kind BuildingKind, onlyActive bool) *Building {
	if r == nil {
		return nil
	}
	for _, b := range r.Buildings {
		if b.Kind == kind && (!onlyActive || b.Operational()) {
			return b
		}
	}
	return nil
}

// Keystone returns the rock's completed, owned Keystone, or nil when the
// rock has no colony.
func (r *Rock) Keystone() *Building {
	if r == nil {
		return nil
	}
	for _, b := range r.Buildings {
		if b.Kind == Keystone && b.State == StateActive {
			return b
		}
	}
	return nil
}

// Surveyed reports whether faction f has fully surveyed the rock.
func (r *Rock) Surveyed(f FactionID) bool {
	return r != nil && r.SurveyedBy.Has(f)
}

// KnownTo reports whether faction f can see what lies under hex h: the tile
// is revealed to f or the whole rock has been surveyed by f. It is false
// outside the tile disc.
func (r *Rock) KnownTo(h Hex, f FactionID) bool {
	t := r.TileAt(h)
	if t == nil {
		return false
	}
	return t.Known.Has(f) || r.SurveyedBy.Has(f)
}

// RevealWithin reveals to faction f every solid (non-Void) tile within
// radius hexes of center and returns how many tiles were newly revealed.
func (r *Rock) RevealWithin(center Hex, radius int, f FactionID) int {
	if r == nil || !f.Valid() {
		return 0
	}
	n := 0
	for _, h := range center.WithinRadius(radius) {
		t := r.TileAt(h)
		if t != nil && t.Terrain != TerrainVoid && t.Reveal(f) {
			n++
		}
	}
	return n
}

// RevealAll reveals every solid (non-Void) tile of the rock to faction f and
// marks the rock as surveyed by f. It returns how many tiles were newly
// revealed.
func (r *Rock) RevealAll(f FactionID) int {
	if r == nil || !f.Valid() {
		return 0
	}
	n := 0
	for i := range r.Tiles {
		if r.Tiles[i].Terrain != TerrainVoid && r.Tiles[i].Reveal(f) {
			n++
		}
	}
	r.SurveyedBy.Set(f)
	return n
}

// NearestHidden returns the solid (non-Void) tile closest to center, within
// radius hexes, that faction f has not revealed yet. Ties are broken by
// Hex.Spiral order, so the result is deterministic. ok is false when nothing
// in range is hidden.
func (r *Rock) NearestHidden(center Hex, radius int, f FactionID) (h Hex, ok bool) {
	if r == nil || !f.Valid() || r.SurveyedBy.Has(f) {
		return Hex{}, false
	}
	for _, c := range center.Spiral(radius) {
		t := r.TileAt(c)
		if t != nil && t.Terrain != TerrainVoid && !t.Known.Has(f) {
			return c, true
		}
	}
	return Hex{}, false
}

// VisibleDeposit returns the deposit under hex h as faction f sees it. ok is
// true only when the tile is revealed to f (see KnownTo) and holds a deposit
// with material left; otherwise g is GoodNone and amount 0.
func (r *Rock) VisibleDeposit(h Hex, f FactionID) (g Good, amount float64, ok bool) {
	t := r.TileAt(h)
	if t == nil || !t.HasDeposit() || !r.KnownTo(h, f) {
		return GoodNone, 0, false
	}
	return t.Deposit, t.Amount, true
}

// AdjacentToOwned reports whether hex h touches (is a neighbour of) a tile
// holding a building owned by faction f, in any state. Derelicts have no
// owner and therefore never count.
func (r *Rock) AdjacentToOwned(h Hex, f FactionID) bool {
	if r == nil || f == NoFaction {
		return false
	}
	for _, n := range h.Neighbors() {
		if b := r.BuildingAt(n); b != nil && b.Owner == f {
			return true
		}
	}
	return false
}
