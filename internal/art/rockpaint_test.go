package art

import (
	"math"
	"testing"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// TestTileProjection checks that the three ways of placing a tile agree:
// where HexPos puts its centre, which tile PickHex finds under a point, and
// the outline the surface is painted with (which is the one TileStroke
// draws).
func TestTileProjection(t *testing.T) {
	if !(Tilt > 0 && Tilt <= 1) {
		t.Fatalf("Tilt = %v", Tilt)
	}
	centre := gfx.P(412.5, 233.25)
	for _, size := range []float32{14, 22, 30, 47.3} {
		for _, h := range (model.Hex{}).WithinRadius(7) {
			p := HexPos(centre, h, size)
			if got := PickHex(centre, p.X, p.Y, size); got != h {
				t.Fatalf("size %v: PickHex at the centre of %v found %v", size, h, got)
			}
			// The surface paints tile h as tileGround(hexCentre(h)), in
			// hex units on the ground.
			x, y := hexCentre(h)
			if dx, dy := centre.X+x*size-p.X, centre.Y+y*Tilt*size-p.Y; math.Abs(float64(dx)) > 1e-2 || math.Abs(float64(dy)) > 1e-2 {
				t.Fatalf("size %v: the surface puts %v %.3f, %.3f away from HexPos", size, h, dx, dy)
			}
			corners := tileGround(x, y, 0)
			outline := tilePoints(p.X, p.Y, size, 1)
			for i := 0; i < 6; i++ {
				cx, cy := centre.X+corners[2*i]*size, centre.Y+corners[2*i+1]*Tilt*size
				// Every painted corner is a corner of the stroked outline.
				near := false
				for _, o := range outline {
					if math.Hypot(float64(o.X-cx), float64(o.Y-cy)) < 0.02 {
						near = true
					}
				}
				if !near {
					t.Fatalf("size %v: corner %d of %v is painted at %.2f, %.2f, which is no corner of the TileStroke outline %v", size, i, h, cx, cy, outline)
				}
				// Just inside each corner is still this tile.
				ix, iy := p.X+(cx-p.X)*0.93, p.Y+(cy-p.Y)*0.93
				if got := PickHex(centre, ix, iy, size); got != h {
					t.Fatalf("size %v: PickHex inside corner %d of %v found %v", size, i, h, got)
				}
			}
		}
	}
}

// TestRockShape checks the body the surface gives every rock: a column
// under every tile and none under the void, and a picture big enough for
// all of it.
func TestRockShape(t *testing.T) {
	w := testWorld(t)
	for _, rock := range w.Rocks {
		sh := shapeOf(rock)
		if len(sh.depth) != len(rock.Tiles) {
			t.Fatalf("%s: %d depths for %d tiles", rock.Name, len(sh.depth), len(rock.Tiles))
		}
		solid := 0
		for i := range rock.Tiles {
			d := sh.depth[i]
			h, ok := rock.HexAt(i)
			if !ok || rock.Tiles[i].Terrain == model.TerrainVoid {
				if d != 0 {
					t.Fatalf("%s: a column %.2f deep under the void", rock.Name, d)
				}
				continue
			}
			solid++
			if !(d >= 0.4 && d < 5.5) || !(sh.tip[i] > 0 && sh.tip[i] < 1.2) {
				t.Fatalf("%s: tile %v has a column %.2f deep with a point of %.2f", rock.Name, h, d, sh.tip[i])
			}
			x, y := hexCentre(h)
			if x-sqrt3/2 < sh.minX || x+sqrt3/2 > sh.maxX || (y-1)*Tilt < sh.minY {
				t.Fatalf("%s: tile %v lies outside the picture", rock.Name, h)
			}
			if foot := max((y+1)*Tilt+d, y*Tilt+d+sh.tip[i]); foot > sh.maxY-1 {
				t.Fatalf("%s: the column of %v ends at %.2f, the picture at %.2f", rock.Name, h, foot, sh.maxY)
			}
		}
		if solid == 0 {
			continue
		}
		// Surface.Draw culls by surfaceReach around the tiles.
		reach := float32(2*rock.Radius+1) + surfaceReach
		if -sh.minX > reach || sh.maxX > reach || -sh.minY > reach || sh.maxY > reach {
			t.Errorf("%s (radius %d): the picture spans %.1f..%.1f by %.1f..%.1f hex units, beyond the %.1f Draw allows for",
				rock.Name, rock.Radius, sh.minX, sh.maxX, sh.minY, sh.maxY, reach)
		}
		// The same rock always gets the same body.
		again := shapeOf(rock)
		for i := range sh.depth {
			if sh.depth[i] != again.depth[i] || sh.tip[i] != again.tip[i] {
				t.Fatalf("%s: the body changed between two calls", rock.Name)
			}
		}
	}
	// An empty rock has no body.
	if sh := shapeOf(model.NewRock(5, 4)); sh.keel != 0 {
		t.Errorf("an empty rock has a keel of %v", sh.keel)
	}
}

// TestSurfaceLayerCoversRock renders layers and checks their geometry: the
// image holds the whole rock, cliff included, with the centre hex where
// Draw will put it.
func TestSurfaceLayerCoversRock(t *testing.T) {
	defer ReleaseSprites()
	w := testWorld(t)
	cv := testCanvas(1)
	for _, rock := range w.Rocks[:min(len(w.Rocks), 6)] {
		for _, hex := range []float32{9, 22} {
			s := NewSurface()
			s.Draw(cv, rock, gfx.P(320, 240), hex, model.PlayerFaction, false)
			e := s.entries[len(s.entries)-1]
			sh := shapeOf(rock)
			if e.ox < -sh.minX*hex-1 || e.w-e.ox < sh.maxX*hex-1 || e.oy < -sh.minY*hex-1 || e.h-e.oy < sh.maxY*hex-1 {
				t.Errorf("%s at %v: layer %v × %v with origin %v, %v does not hold %.1f..%.1f by %.1f..%.1f hexes",
					rock.Name, hex, e.w, e.h, e.ox, e.oy, sh.minX, sh.maxX, sh.minY, sh.maxY)
			}
			// More of the picture lies below the centre than above it:
			// that is the cliff.
			if !(e.h-e.oy > e.oy) && rock.Radius >= 3 {
				t.Errorf("%s at %v: the layer has %.0f pt above the centre and %.0f below", rock.Name, hex, e.oy, e.h-e.oy)
			}
			s.Release()
		}
	}
}
