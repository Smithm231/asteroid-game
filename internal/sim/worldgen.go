package sim

import (
	"fmt"
	"math"
	"sort"

	"drifthold/internal/model"
)

// rockNames is the pool of rock names; the belt draws from it in random order.
var rockNames = []string{
	"Brannoc", "Tessaly", "Corrow", "Wend", "Ashmere", "Kelder", "Pale Anvil",
	"Lantern", "Sable Reach", "Quillon", "Harrow", "Dunmark", "Thresh",
	"Calder", "Orrin", "Fenwick", "Marrowgate", "Sorrel", "Ivory Tooth",
	"Garrick", "Low Candle", "Verity", "Stannet", "Hollin",
}

// NewGame generates a complete, playable world from the given settings: the
// belt of rocks with their terrain and deposits, one faction per player with a
// starting Keystone colony each, and the bookkeeping the systems expect.
// The same settings always produce the same world.
func NewGame(settings model.GameSettings) (*model.World, error) {
	settings = settings.Normalized()
	w := model.NewWorld(settings)
	rng := w.RNG.Derive("worldgen")

	// Factions.
	nf := 1 + settings.Rivals
	kinds := model.RivalPersonalities()
	pool := kinds[:]
	model.ShuffleSlice(&rng, pool)
	used := map[model.Personality]bool{}
	for _, p := range settings.RivalKinds {
		used[p] = true
	}
	next := 0
	for i := 0; i < nf; i++ {
		id := model.FactionID(i + 1)
		var f *model.Faction
		if i == 0 {
			f = model.NewFaction(id, settings.PlayerName, model.PersonalityNone)
		} else {
			p := model.PersonalityNone
			if i-1 < len(settings.RivalKinds) {
				p = settings.RivalKinds[i-1]
			}
			for !p.IsRival() {
				cand := pool[next%len(pool)]
				next++
				if !used[cand] || next > 2*len(pool) {
					p = cand
					used[cand] = true
				}
			}
			f = model.NewFaction(id, p.String(), p)
		}
		f.Scrip = model.StartScrip
		f.RecomputeMods(settings.Difficulty)
		w.Factions = append(w.Factions, f)
	}

	// Orbits: radii evenly spread with jitter, never closer than OrbitMinGap.
	n := settings.BeltSize.RockCount()
	step := (model.OrbitRMax - model.OrbitRMin) / float64(n-1)
	jit := math.Max(0, (step-model.OrbitMinGap)/2)
	radii := make([]float64, n)
	for i := range radii {
		radii[i] = model.OrbitRMin + step*float64(i) + rng.Range(-jit, jit)
		radii[i] = model.Clamp(radii[i], model.OrbitRMin, model.OrbitRMax)
	}

	// Start rocks sit at mid radii, spread evenly in phase.
	startIdx := pickStartIndices(&rng, n, nf)
	isStart := make(map[int]int, nf) // rock index -> faction index
	for fi, ri := range startIdx {
		isStart[ri] = fi
	}
	phase0 := rng.Range(0, 2*math.Pi)

	// Classes.
	classes := make([]model.RockClass, n)
	for i := range classes {
		r := radii[i]
		switch {
		case r > model.FrostMinRadius && rng.Chance(0.5):
			classes[i] = model.ClassFrost
		case r < model.FerricMaxRadius && rng.Chance(0.6), r >= model.FerricMaxRadius && rng.Chance(0.25):
			classes[i] = model.ClassFerric
		default:
			classes[i] = model.ClassStony
		}
	}
	for ri, fi := range isStart {
		if fi%2 == 0 {
			classes[ri] = model.ClassStony
		} else {
			classes[ri] = model.ClassFerric
		}
	}
	// Glint rocks: a few non-start rocks.
	free := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if _, ok := isStart[i]; !ok {
			free = append(free, i)
		}
	}
	model.ShuffleSlice(&rng, free)
	glints := rng.IntRange(model.GlintRocksMin, model.GlintRocksMax)
	for i := 0; i < glints && i < len(free); i++ {
		classes[free[i]] = model.ClassGlint
	}
	// Make sure the belt has Ice to fight over: at least two Frost rocks.
	frost := 0
	for _, c := range classes {
		if c == model.ClassFrost {
			frost++
		}
	}
	for i := n - 1; i >= 0 && frost < 2; i-- {
		if _, ok := isStart[i]; ok || classes[i] == model.ClassGlint || classes[i] == model.ClassFrost {
			continue
		}
		classes[i] = model.ClassFrost
		frost++
	}

	names := append([]string(nil), rockNames...)
	model.ShuffleSlice(&rng, names)

	for i := 0; i < n; i++ {
		rr := rng.Derive(fmt.Sprintf("rock:%d", i))
		radius := rr.IntRange(model.RockRadiusMin, model.RockRadiusMax)
		fi, start := isStart[i]
		switch {
		case start:
			radius = rr.IntRange(9, 10)
		case classes[i] == model.ClassGlint:
			radius = model.RockRadiusMin
		}
		rock := model.NewRock(model.RockID(i), radius)
		rock.Name = names[i%len(names)]
		rock.Class = classes[i]
		rock.OrbitR = radii[i]
		rock.Solar = model.SolarFactor(radii[i])
		rock.ShapeSeed = rr.Uint64()
		if start {
			rock.Phase0 = phase0 + 2*math.Pi*float64(fi)/float64(nf) + rr.Range(-0.15, 0.15)
		} else {
			rock.Phase0 = rr.Range(0, 2*math.Pi)
		}
		genBlob(&rr, rock)
		genBlocked(&rr, rock)
		w.Rocks = append(w.Rocks, rock)

		if start {
			f := w.Factions[fi]
			site := bestSite(rock)
			genStartDeposits(&rr, rock, site)
			genDeposits(&rr, rock)
			if _, err := w.FoundColony(rock.ID, f.ID, site); err != nil {
				return nil, fmt.Errorf("founding start colony on %s: %w", rock.Name, err)
			}
			// The charter survey: a start colony knows its surroundings out to
			// the guaranteed Ice pocket.
			rock.RevealWithin(site, model.StartIceRange, f.ID)
			rock.Colony.Stock = model.StartStock()
			rock.Colony.Pop = model.StartPop
			rock.Colony.Workers = int(model.StartPop)
			f.HomeRock = rock.ID
		} else {
			genDeposits(&rr, rock)
		}
	}

	w.Events.NextRollTick = model.CyclesToTicks(float64(model.EventFirstCycle-1)) +
		model.CyclesToTicks(rng.Range(0, model.EventGapMaxCycles-model.EventGapMinCycles))
	for _, f := range w.Factions {
		f.Directives.Done = make([]bool, len(directiveDefs))
	}
	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("generated world is invalid: %w", err)
	}
	return w, nil
}

// pickStartIndices chooses nf distinct rock indices from the middle of the
// radius-sorted belt.
func pickStartIndices(rng *model.RNG, n, nf int) []int {
	lo, hi := n/4, n-1-n/4
	if hi-lo+1 < nf {
		lo, hi = 0, n-1
	}
	cands := make([]int, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		cands = append(cands, i)
	}
	model.ShuffleSlice(rng, cands)
	out := append([]int(nil), cands[:nf]...)
	return out
}

// genBlob carves the irregular rock shape out of the hex disc: a star-shaped
// outline whose radius wobbles with direction, scaled until the fill fraction
// lands inside the allowed band.
func genBlob(rng *model.RNG, rock *model.Rock) {
	type harm struct{ k, amp, ph float64 }
	hs := []harm{
		{2, rng.Range(0.05, 0.13), rng.Range(0, 2*math.Pi)},
		{3, rng.Range(0.04, 0.10), rng.Range(0, 2*math.Pi)},
		{5, rng.Range(0.02, 0.06), rng.Range(0, 2*math.Pi)},
	}
	hexes := rock.Hexes()
	total := float64(len(hexes))
	target := rng.Range(model.BlobFillMin+0.02, model.BlobFillMax-0.02)
	scale := 0.82
	for iter := 0; iter < 24; iter++ {
		count := 0
		for _, h := range hexes {
			t := rock.TileAt(h)
			p := model.HexToPixel(h, 1)
			d := p.Len() / math.Sqrt(3)
			f := 1.0
			a := p.Angle()
			for _, hm := range hs {
				f += hm.amp * math.Sin(hm.k*a+hm.ph)
			}
			if d <= float64(rock.Radius)*scale*f+0.01 {
				t.Terrain = model.TerrainRegolith
				count++
			} else {
				t.Terrain = model.TerrainVoid
			}
			t.Deposit = model.GoodNone
		}
		fill := float64(count) / total
		if fill >= model.BlobFillMin && fill <= model.BlobFillMax && math.Abs(fill-target) < 0.03 {
			break
		}
		scale *= math.Sqrt(target / math.Max(fill, 0.05))
		scale = model.Clamp(scale, 0.5, 1.2)
	}
}

// genBlocked scatters small clusters of Crag and Fissure tiles, never cutting
// the open ground into separate pieces.
func genBlocked(rng *model.RNG, rock *model.Rock) {
	blob := rock.BlobHexes()
	want := int(rng.Range(model.BlockedFracMin, model.BlockedFracMax) * float64(len(blob)))
	placed := 0
	for attempt := 0; attempt < 200 && placed < want; attempt++ {
		seed := blob[rng.Intn(len(blob))]
		if seed.Len() <= 2 {
			continue // keep the heart of the rock open
		}
		kind := model.TerrainCrag
		if rng.Chance(0.4) {
			kind = model.TerrainFissure
		}
		size := rng.IntRange(1, 4)
		cluster := []model.Hex{}
		cur := seed
		for i := 0; i < size; i++ {
			t := rock.TileAt(cur)
			if t == nil || t.Terrain != model.TerrainRegolith || cur.Len() <= 2 {
				break
			}
			t.Terrain = kind
			cluster = append(cluster, cur)
			cur = cur.Neighbor(rng.Intn(6))
		}
		if len(cluster) == 0 {
			continue
		}
		if !regolithConnected(rock) {
			for _, h := range cluster {
				rock.TileAt(h).Terrain = model.TerrainRegolith
			}
			continue
		}
		placed += len(cluster)
	}
}

// regolithConnected reports whether every Regolith tile can be reached from
// the centre-most Regolith tile by stepping over Regolith.
func regolithConnected(rock *model.Rock) bool {
	total := 0
	var start model.Hex
	found := false
	for _, h := range (model.Hex{}).Spiral(rock.Radius) {
		t := rock.TileAt(h)
		if t != nil && t.Terrain == model.TerrainRegolith {
			if !found {
				start, found = h, true
			}
			total++
		}
	}
	if !found {
		return false
	}
	seen := map[model.Hex]bool{start: true}
	queue := []model.Hex{start}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		for _, nb := range h.Neighbors() {
			if seen[nb] {
				continue
			}
			t := rock.TileAt(nb)
			if t == nil || t.Terrain != model.TerrainRegolith {
				continue
			}
			seen[nb] = true
			queue = append(queue, nb)
		}
	}
	return len(seen) == total
}

// bestSite returns the open tile closest to the rock's centre that has open
// ground all around it: where a Keystone is founded when no site is requested.
func bestSite(rock *model.Rock) model.Hex {
	best, bestScore := model.Hex{}, -1
	for _, h := range (model.Hex{}).Spiral(rock.Radius) {
		t := rock.TileAt(h)
		if t == nil || !t.Buildable() {
			continue
		}
		open := 0
		for _, nb := range h.Neighbors() {
			if nt := rock.TileAt(nb); nt != nil && nt.Buildable() {
				open++
			}
		}
		// Closer to the centre wins unless a site is clearly roomier.
		score := open*3 - h.Len()
		if score > bestScore {
			best, bestScore = h, score
		}
		if open == 6 && h.Len() <= 1 {
			break
		}
	}
	return best
}

// growVein lays a deposit of good g starting at seed and spreading to
// neighbouring solid tiles without a deposit. It returns the tiles used.
func growVein(rng *model.RNG, rock *model.Rock, seed model.Hex, g model.Good, tiles int, lo, hi float64) int {
	frontier := []model.Hex{seed}
	done := 0
	for done < tiles && len(frontier) > 0 {
		i := rng.Intn(len(frontier))
		h := frontier[i]
		frontier = append(frontier[:i], frontier[i+1:]...)
		t := rock.TileAt(h)
		if t == nil || !t.Terrain.Solid() || t.Deposit != model.GoodNone {
			continue
		}
		t.Deposit = g
		t.Amount = math.Round(rng.Range(lo, hi))
		done++
		for _, nb := range h.Neighbors() {
			frontier = append(frontier, nb)
		}
	}
	return done
}

// randomVein places one vein of good g somewhere on the rock.
func randomVein(rng *model.RNG, rock *model.Rock, g model.Good) {
	blob := rock.BlobHexes()
	for attempt := 0; attempt < 30; attempt++ {
		seed := blob[rng.Intn(len(blob))]
		if rock.TileAt(seed).Deposit != model.GoodNone {
			continue
		}
		if g == model.Lumen {
			growVein(rng, rock, seed, g, rng.IntRange(model.LumenTilesMin, model.LumenTilesMax),
				model.LumenAmountMin, model.LumenAmountMax)
		} else {
			growVein(rng, rock, seed, g, rng.IntRange(model.VeinTilesMin, model.VeinTilesMax),
				model.VeinAmountMin, model.VeinAmountMax)
		}
		return
	}
}

// genDeposits fills a rock with the veins its class favours, scaled with its
// size.
func genDeposits(rng *model.RNG, rock *model.Rock) {
	area := float64(rock.BlobSize()) / 170.0
	count := func(lo, hi int) int {
		return int(math.Round(float64(rng.IntRange(lo, hi)) * area))
	}
	type vein struct {
		g model.Good
		n int
	}
	var plan []vein
	switch rock.Class {
	case model.ClassFerric:
		plan = []vein{{model.Ore, max(2, count(4, 6))}, {model.Silica, max(1, count(1, 2))}}
		if rock.OrbitR > model.FrostMinRadius {
			plan = append(plan, vein{model.Ice, count(0, 1)})
		}
	case model.ClassStony:
		plan = []vein{{model.Silica, max(2, count(3, 5))}, {model.Ore, max(1, count(1, 3))}, {model.Ice, count(0, 1)}}
	case model.ClassFrost:
		plan = []vein{{model.Ice, max(3, count(4, 6))}, {model.Silica, max(1, count(1, 2))}, {model.Ore, count(0, 2)}}
	case model.ClassGlint:
		plan = []vein{{model.Lumen, rng.IntRange(2, 3)}, {model.Ore, 1}, {model.Silica, 1}}
	}
	for _, v := range plan {
		for i := 0; i < v.n; i++ {
			randomVein(rng, rock, v.g)
		}
	}
}

// genStartDeposits guarantees a start rock the veins a new colony needs close
// to its Keystone site: Ore and Silica within StartVeinRange and a small Ice
// pocket within StartIceRange.
func genStartDeposits(rng *model.RNG, rock *model.Rock, site model.Hex) {
	place := func(g model.Good, minD, maxD, tiles int, lo, hi float64) {
		var cands []model.Hex
		for d := minD; d <= maxD; d++ {
			for _, h := range site.Ring(d) {
				t := rock.TileAt(h)
				if t != nil && t.Terrain.Solid() && t.Deposit == model.GoodNone {
					cands = append(cands, h)
				}
			}
		}
		// Deterministic order before the random pick.
		sort.Slice(cands, func(i, j int) bool {
			if cands[i].R != cands[j].R {
				return cands[i].R < cands[j].R
			}
			return cands[i].Q < cands[j].Q
		})
		for attempt := 0; attempt < 12 && len(cands) > 0; attempt++ {
			seed := cands[rng.Intn(len(cands))]
			if growVein(rng, rock, seed, g, tiles, lo, hi) >= min(tiles, 2) {
				return
			}
		}
	}
	place(model.Ore, 2, model.StartVeinRange-1, rng.IntRange(5, 7), 700, model.VeinAmountMax)
	place(model.Silica, 2, model.StartVeinRange-1, rng.IntRange(4, 6), 600, model.VeinAmountMax)
	place(model.Ice, 3, model.StartIceRange-1, rng.IntRange(3, 4), 400, 650)
}
