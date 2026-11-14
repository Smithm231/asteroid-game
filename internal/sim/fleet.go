package sim

import (
	"errors"
	"math"
	"slices"

	"drifthold/internal/model"
)

// ---------------------------------------------------------------- shipyards

// QueueShip adds a ship to one of faction f's Launch Cradle queues, paying its
// cost (and, for a Seedship, taking its colonists) up front.
func QueueShip(w *model.World, f model.FactionID, rock model.RockID, cradle model.BuildingID, kind model.ShipKind) error {
	fac, err := w.Issuer(f)
	if err != nil {
		return err
	}
	r, b, err := ownBuilding(w, f, rock, cradle)
	if err != nil {
		return err
	}
	bdef := b.Def()
	if bdef.QueueSize <= 0 || !b.Completed() {
		return errors.New("That structure cannot build ships")
	}
	if !kind.Valid() {
		return model.ErrBadCommand
	}
	if !fac.CanBuildShip(kind) {
		return model.ErrLocked
	}
	if len(b.Queue) >= bdef.QueueSize {
		return errors.New("The build queue is full")
	}
	def := model.ShipDefOf(kind)
	if w.BerthsFree(f) < def.Berths {
		return errors.New("No free berths: build a Dock Ring or found another colony")
	}
	c := r.Colony
	if def.CanColonize && c.Pop < model.SeedshipMinPop {
		return errors.New("A Seedship needs a colony of at least 20 colonists")
	}
	if err := c.Stock.Shortage(def.Cost); err != nil {
		return err
	}
	c.Pay(def.Cost)
	if def.CanColonize {
		c.Pop -= model.SeedshipColonists
	}
	b.Queue = append(b.Queue, kind)
	return nil
}

// CancelShip removes an entry from a Launch Cradle queue with a full refund.
func CancelShip(w *model.World, f model.FactionID, rock model.RockID, cradle model.BuildingID, index int) error {
	if _, err := w.Issuer(f); err != nil {
		return err
	}
	r, b, err := ownBuilding(w, f, rock, cradle)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(b.Queue) {
		return model.ErrBadCommand
	}
	def := model.ShipDefOf(b.Queue[index])
	r.Colony.AddStockClamped(def.Cost)
	if def.CanColonize {
		r.Colony.Pop += model.SeedshipColonists
	}
	b.Queue = slices.Delete(b.Queue, index, index+1)
	if index == 0 {
		b.QueueProgress = 0
	}
	return nil
}

// stepShipyards advances every Launch Cradle queue by dt game-seconds.
func stepShipyards(w *model.World, dt float64) {
	for _, r := range w.Rocks {
		if r.Colony == nil {
			continue
		}
		for _, b := range r.Buildings {
			if b.Def().QueueSize <= 0 || !b.Operational() {
				continue
			}
			if len(b.Queue) == 0 {
				b.QueueProgress = 0
				if b.Idle == model.IdleNone {
					b.Idle = model.IdleNoWork
				}
				continue
			}
			if b.Eff <= 0 {
				continue
			}
			kind := b.Queue[0]
			b.QueueProgress += dt * b.Eff / max(1, model.ShipDefOf(kind).BuildTime)
			if b.QueueProgress >= 1-1e-9 {
				b.Queue = b.Queue[1:]
				b.QueueProgress = 0
				spawnShip(w, r, b, kind)
			}
		}
	}
}

// spawnShip puts a finished ship into the garrison of the rock it was built on.
func spawnShip(w *model.World, r *model.Rock, b *model.Building, kind model.ShipKind) {
	f := r.Owner
	ship := model.NewShip(w.NewShipID(), kind, w.Mods(f).HPMul)
	g := w.GarrisonOf(r.ID, f)
	if g == nil {
		g = model.NewFleet(f, r.ID)
		g.Garrison = true
		g.Pos = w.RockPos(r.ID)
		w.AddFleet(g)
	}
	g.Ships = append(g.Ships, ship)
	if fac := w.Faction(f); fac != nil {
		fac.Stats.ShipsBuilt++
	}
	fx := model.NewFX(model.FXShipBuilt, r.ID, b.Hex, f)
	fx.Amount = float64(kind)
	w.Emit(fx)
	note(w, f, model.SevInfo, r.ID, "%s launched from %s.", model.ShipDefOf(kind).Name, r.Name)
}

// ------------------------------------------------------------------- orders

// pickShips resolves a command's ship selection against a fleet. It returns
// the selected ships (copies) and whether the selection is the whole fleet.
func pickShips(fl *model.Fleet, ids []model.ShipID) ([]model.Ship, bool, error) {
	if len(ids) == 0 {
		return fl.Ships, true, nil
	}
	seen := make(map[model.ShipID]bool, len(ids))
	out := make([]model.Ship, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		s := fl.Ship(id)
		if s == nil {
			return nil, false, model.ErrNoShip
		}
		out = append(out, *s)
	}
	return out, len(out) == len(fl.Ships), nil
}

// split moves the given ships out of fl into a new fleet at the same place.
func split(w *model.World, fl *model.Fleet, ids []model.ShipID) *model.Fleet {
	nf := model.NewFleet(fl.Owner, fl.At)
	nf.Pos, nf.Dest, nf.Origin, nf.Home, nf.LaunchTick = fl.Pos, fl.Dest, fl.Origin, fl.Home, fl.LaunchTick
	for _, id := range ids {
		if s, ok := fl.RemoveShip(id); ok {
			nf.Ships = append(nf.Ships, s)
		}
	}
	w.AddFleet(nf)
	return nf
}

// commandFleet returns the fleet a command addresses: the whole fleet, or a
// freshly split one holding the selected ships.
func commandFleet(w *model.World, fl *model.Fleet, ids []model.ShipID, all bool) *model.Fleet {
	if all {
		return fl
	}
	return split(w, fl, ids)
}

// HomeFor returns the rock a fleet should fall back to: its Home while its
// faction still owns it, otherwise the nearest rock the faction owns, or
// NoRock.
func HomeFor(w *model.World, fl *model.Fleet) model.RockID {
	if h := w.Rock(fl.Home); h != nil && h.Owner == fl.Owner {
		return fl.Home
	}
	best, bestD := model.NoRock, math.Inf(1)
	pos := w.FleetPos(fl)
	for _, r := range w.Rocks {
		if r.Owner != fl.Owner {
			continue
		}
		if d := pos.Dist(w.RockPos(r.ID)); d < bestD {
			best, bestD = r.ID, d
		}
	}
	return best
}

// LaunchFuel returns the Fuel a fleet made of the given ships must pay to
// leave rock from for rock to: nothing unless from is owned by the faction.
func LaunchFuel(w *model.World, owner model.FactionID, ships []model.Ship, from, to model.RockID) float64 {
	r := w.Rock(from)
	if r == nil || r.Owner != owner || from == to {
		return 0
	}
	per := 0.0
	for i := range ships {
		per += ships[i].Def().FuelPerU
	}
	return per * w.RockDist(from, to)
}

// launch sends a fleet in orbit on its way to target. Fuel has been checked.
func launch(w *model.World, fl *model.Fleet, target model.RockID, fuel float64) {
	origin := fl.At
	if c := w.Colony(origin); c != nil && fuel > 0 {
		c.NoteConsumed(model.Fuel, c.Take(model.Fuel, fuel))
	}
	fl.Pos = w.RockPos(origin)
	fl.Origin = origin
	fl.At = model.NoRock
	fl.Dest = target
	fl.Garrison = false
	fl.Engaged = false
	fl.LaunchTick = w.Tick
	fl.SurveyTimer, fl.DwellTimer, fl.RetryTimer = 0, 0, 0
	for i := range fl.Ships {
		fl.Ships[i].TargetShip, fl.Ships[i].TargetBuilding = 0, 0
	}
	fx := model.NewFX(model.FXFleetLaunch, origin, model.Hex{}, fl.Owner)
	fx.Fleet, fx.A = fl.ID, fl.Pos
	w.Emit(fx)
}

// tryLaunch launches a fleet toward target if its origin colony can pay the
// Fuel. It reports whether the fleet left.
func tryLaunch(w *model.World, fl *model.Fleet, target model.RockID) bool {
	fuel := LaunchFuel(w, fl.Owner, fl.Ships, fl.At, target)
	if fuel > 0 {
		c := w.Colony(fl.At)
		if c == nil || c.Stock[model.Fuel] < fuel-model.StockEpsilon {
			return false
		}
	}
	launch(w, fl, target, fuel)
	return true
}

// FleetOrder executes a CmdFleetOrder: it gives a fleet, or a subset of its
// ships (which is split off), a new standing order and launches it if needed.
func FleetOrder(w *model.World, c model.Command) error {
	if _, err := w.Issuer(c.Faction); err != nil {
		return err
	}
	fl, err := w.OwnedFleet(c.Faction, c.Fleet)
	if err != nil {
		return err
	}
	ships, all, err := pickShips(fl, c.Ships)
	if err != nil {
		return err
	}
	has := func(pred func(model.ShipDef) bool) bool {
		for i := range ships {
			if pred(ships[i].Def()) {
				return true
			}
		}
		return false
	}
	target := c.Target
	tr := w.Rock(target)
	switch c.Order {
	case model.OrderHold:
		target = model.NoRock
	case model.OrderMove:
		if tr == nil {
			return model.ErrNoRock
		}
	case model.OrderAttack:
		if tr == nil {
			return model.ErrNoRock
		}
		if tr.Owner == c.Faction {
			return errors.New("That rock is your own")
		}
		if !has(model.ShipDef.Armed) {
			return errors.New("Those ships carry no weapons")
		}
		if !c.Stance.Valid() || !model.ValidRetreat(c.Retreat) {
			return model.ErrBadCommand
		}
	case model.OrderSurvey:
		if tr == nil {
			return model.ErrNoRock
		}
		if !has(func(d model.ShipDef) bool { return d.CanSurvey }) {
			return errors.New("A survey needs a Prospector")
		}
	case model.OrderColonize:
		if tr == nil {
			return model.ErrNoRock
		}
		if !has(func(d model.ShipDef) bool { return d.CanColonize }) {
			return errors.New("Founding a colony needs a Seedship")
		}
		if tr.Owner != model.NoFaction {
			return model.ErrRockClaimed
		}
	case model.OrderReturn:
		target = HomeFor(w, fl)
		if target == model.NoRock {
			return errors.New("There is no colony to return to")
		}
	default:
		return model.ErrBadCommand
	}

	launching := !fl.InTransit() && target != model.NoRock && target != fl.At
	fuel := 0.0
	if launching {
		fuel = LaunchFuel(w, fl.Owner, ships, fl.At, target)
		if col := w.Colony(fl.At); fuel > 0 && (col == nil || col.Stock[model.Fuel] < fuel-model.StockEpsilon) {
			return &model.ShortageError{Good: model.Fuel, Need: math.Ceil(fuel)}
		}
	}

	sub := commandFleet(w, fl, c.Ships, all)
	sub.Order = c.Order
	sub.Route = nil
	sub.Garrison = false
	sub.SurveyTimer, sub.DwellTimer, sub.RetryTimer = 0, 0, 0
	sub.HasColonizeHex, sub.ColonizeHex = false, model.Hex{}
	sub.Stance, sub.RetreatAt, sub.InitialMaxHP = model.StanceSuppress, model.RetreatNever, 0
	switch c.Order {
	case model.OrderAttack:
		sub.Stance, sub.RetreatAt, sub.InitialMaxHP = c.Stance, c.Retreat, sub.TotalMaxHP()
	case model.OrderColonize:
		sub.HasColonizeHex, sub.ColonizeHex = c.HasHex, c.Hex
	}
	switch {
	case launching:
		launch(w, sub, target, fuel)
	case sub.InTransit():
		if target != model.NoRock {
			sub.Dest = target
		}
	default:
		// Already where the order points.
		if target != model.NoRock {
			sub.Dest = target
		}
		inOrbit(w, sub, true)
	}
	return nil
}

// SetRoute executes a CmdSetRoute: it assigns a freight route to a fleet (or
// to a subset of its ships) and starts hauling.
func SetRoute(w *model.World, c model.Command) error {
	if _, err := w.Issuer(c.Faction); err != nil {
		return err
	}
	fl, err := w.OwnedFleet(c.Faction, c.Fleet)
	if err != nil {
		return err
	}
	ships, all, err := pickShips(fl, c.Ships)
	if err != nil {
		return err
	}
	cap := 0.0
	for i := range ships {
		cap += ships[i].Def().Cargo
	}
	if cap <= 0 {
		return errors.New("A freight route needs a Hauler")
	}
	rt := c.Route
	a, b := w.Rock(rt.A), w.Rock(rt.B)
	if a == nil || b == nil || rt.A == rt.B {
		return model.ErrNoRock
	}
	if a.Owner != c.Faction || b.Owner != c.Faction {
		return errors.New("Both ends of a freight route must be your colonies")
	}
	if math.IsNaN(rt.Reserve) || rt.Reserve < 0 {
		rt.Reserve = 0
	}
	sub := commandFleet(w, fl, c.Ships, all)
	sub.Order = model.OrderHaul
	sub.Route = &rt
	sub.Garrison = false
	sub.Home = rt.A
	sub.SurveyTimer, sub.DwellTimer, sub.RetryTimer = 0, 0, 0
	return nil
}

// Scuttle executes a CmdScuttle: it destroys the selected ships.
func Scuttle(w *model.World, c model.Command) error {
	fac, err := w.Issuer(c.Faction)
	if err != nil {
		return err
	}
	fl, err := w.OwnedFleet(c.Faction, c.Fleet)
	if err != nil {
		return err
	}
	ships, _, err := pickShips(fl, c.Ships)
	if err != nil {
		return err
	}
	ids := make([]model.ShipID, len(ships))
	for i := range ships {
		ids[i] = ships[i].ID
	}
	for _, id := range ids {
		if _, ok := fl.RemoveShip(id); ok {
			fac.Stats.ShipsLost++
		}
	}
	if fl.Empty() {
		w.RemoveFleet(fl.ID)
	}
	return nil
}

// ----------------------------------------------------------------- movement

// stepFleets moves every fleet in transit by one tick, handles arrivals and
// the retreat rule, and on slow ticks runs the standing orders of fleets in
// orbit.
func stepFleets(w *model.World) {
	slow := w.SlowTick()
	for _, fl := range slices.Clone(w.Fleets) {
		if fl.Empty() {
			w.RemoveFleet(fl.ID)
			continue
		}
		if fl.InTransit() {
			dest := w.Rock(fl.Dest)
			if dest == nil {
				dest = w.Rock(HomeFor(w, fl))
				if dest == nil {
					continue
				}
				fl.Dest = dest.ID
			}
			target := dest.PosAt(w.Tick)
			fl.Pos = fl.Pos.Toward(target, fl.Speed(w.Mods(fl.Owner))*model.TickDT)
			if fl.Pos.Dist(target) <= model.ArriveRadius {
				arrive(w, fl)
			}
			continue
		}
		if fl.Order == model.OrderAttack && fl.RetreatAt > 0 && fl.TotalHP() < fl.RetreatAt*fl.InitialMaxHP {
			note(w, fl.Owner, model.SevWarn, fl.At, "A fleet is breaking off its attack on %s.", rockName(w, fl.At))
			goHome(w, fl)
			continue
		}
		if slow {
			inOrbit(w, fl, false)
		}
	}
}

func rockName(w *model.World, id model.RockID) string {
	if r := w.Rock(id); r != nil {
		return r.Name
	}
	return "deep space"
}

// arrive puts a fleet in transit into orbit of its destination.
func arrive(w *model.World, fl *model.Fleet) {
	r := w.Rock(fl.Dest)
	fl.At = fl.Dest
	fl.Pos = w.RockPos(fl.At)
	fx := model.NewFX(model.FXFleetArrive, fl.At, model.Hex{}, fl.Owner)
	fx.Fleet, fx.A = fl.ID, fl.Pos
	w.Emit(fx)
	if r.Owner != model.NoFaction && model.Hostile(r.Owner, fl.Owner) && fl.Armed() {
		note(w, r.Owner, model.SevAlert, r.ID, "Hostile ships have entered orbit of %s!", r.Name)
		w.Emit(model.NewFX(model.FXAlert, r.ID, model.Hex{}, r.Owner))
	}
	if fl.Order == model.OrderHaul {
		fl.DwellTimer = model.RouteDwellSeconds
		unloadAll(w, fl, r)
		return
	}
	inOrbit(w, fl, true)
}

// goHome turns a fleet toward the rock it falls back to.
func goHome(w *model.World, fl *model.Fleet) {
	home := HomeFor(w, fl)
	fl.Route = nil
	if home == model.NoRock {
		fl.Order = model.OrderHold
		return
	}
	fl.Order = model.OrderReturn
	fl.Dest = home
	if fl.InTransit() {
		return
	}
	if home == fl.At {
		settle(w, fl)
		return
	}
	if !tryLaunch(w, fl, home) {
		fl.Order = model.OrderHold
	}
}

// settle ends a fleet's task in orbit: at a friendly rock it unloads and joins
// the garrison, elsewhere it simply holds.
func settle(w *model.World, fl *model.Fleet) {
	r := w.Rock(fl.At)
	fl.Order = model.OrderHold
	fl.Route = nil
	fl.Dest = model.NoRock
	if r == nil || r.Owner != fl.Owner {
		fl.Garrison = false
		return
	}
	unloadAll(w, fl, r)
	fl.Home = fl.At
	g := w.GarrisonOf(fl.At, fl.Owner)
	if g == nil || g == fl {
		fl.Garrison = true
		return
	}
	g.Ships = append(g.Ships, fl.Ships...)
	fl.Ships = nil
	w.RemoveFleet(fl.ID)
}

// inOrbit runs the standing order of a fleet in orbit. fresh is true right
// after it arrived or received the order.
func inOrbit(w *model.World, fl *model.Fleet, fresh bool) {
	r := w.Rock(fl.At)
	if r == nil {
		return
	}
	switch fl.Order {
	case model.OrderHold:
		if !fl.Garrison && (fresh || r.Owner == fl.Owner) {
			settle(w, fl)
		}
	case model.OrderMove:
		settle(w, fl)
	case model.OrderReturn:
		home := HomeFor(w, fl)
		if home == fl.At || home == model.NoRock {
			settle(w, fl)
		} else if !tryLaunch(w, fl, home) {
			fl.Order = model.OrderHold
		}
	case model.OrderAttack:
		if fl.At != fl.Dest {
			return
		}
		if fl.InitialMaxHP <= 0 {
			fl.InitialMaxHP = fl.TotalMaxHP()
		}
		if fresh {
			return
		}
		hostileRock := r.Owner != model.NoFaction && model.Hostile(r.Owner, fl.Owner)
		if !hostileRock && !hostilesAt(w, fl) {
			goHome(w, fl)
		}
	case model.OrderSurvey:
		if fl.At != fl.Dest {
			return
		}
		if !fresh {
			fl.SurveyTimer += model.SlowDT
		}
		if fl.SurveyTimer >= model.SurveySeconds-1e-9 {
			if !r.Surveyed(fl.Owner) {
				r.RevealAll(fl.Owner)
				if fac := w.Faction(fl.Owner); fac != nil {
					fac.Stats.RocksSurveyed++
				}
				note(w, fl.Owner, model.SevGood, r.ID, "Survey of %s complete.", r.Name)
			}
			goHome(w, fl)
		}
	case model.OrderColonize:
		if fl.At == fl.Dest {
			tryColonize(w, fl, r)
		}
	case model.OrderHaul:
		stepHaul(w, fl, r)
	}
}

// hostilesAt reports whether a fleet hostile to fl shares its orbit.
func hostilesAt(w *model.World, fl *model.Fleet) bool {
	for _, o := range w.Fleets {
		if o.At == fl.At && o != fl && model.Hostile(o.Owner, fl.Owner) && !o.Empty() {
			return true
		}
	}
	return false
}

// tryColonize lands a Seedship: it founds a Keystone on the rock unless the
// rock was claimed meanwhile or armed enemies contest the orbit.
func tryColonize(w *model.World, fl *model.Fleet, r *model.Rock) {
	if r.Owner != model.NoFaction {
		note(w, fl.Owner, model.SevWarn, r.ID, "%s was claimed before the Seedship arrived.", r.Name)
		goHome(w, fl)
		return
	}
	for _, o := range w.Fleets {
		if o.At == fl.At && model.Hostile(o.Owner, fl.Owner) && o.Armed() {
			return // wait for the orbit to clear
		}
	}
	var seed model.ShipID
	for i := range fl.Ships {
		if fl.Ships[i].Def().CanColonize {
			seed = fl.Ships[i].ID
			break
		}
	}
	if seed == 0 {
		settle(w, fl)
		return
	}
	site := bestSite(r)
	if fl.HasColonizeHex {
		if t := r.TileAt(fl.ColonizeHex); t != nil && t.Buildable() {
			site = fl.ColonizeHex
		}
	}
	ks, err := w.FoundColony(r.ID, fl.Owner, site)
	if err != nil {
		note(w, fl.Owner, model.SevWarn, r.ID, "No landing site could be found on %s.", r.Name)
		goHome(w, fl)
		return
	}
	r.Colony.AddStockClamped(model.SeedshipStock())
	r.Colony.Pop = model.SeedshipColonists
	r.Colony.Workers = int(model.SeedshipColonists)
	fl.RemoveShip(seed)
	fac := w.Faction(fl.Owner)
	if fac != nil {
		fac.Stats.ColoniesFounded++
		w.Logf(model.NoFaction, model.SevInfo, r.ID, "%s has founded a colony on %s.", fac.Name, r.Name)
	}
	w.Emit(model.NewFX(model.FXColonyFounded, r.ID, ks.Hex, fl.Owner))
	if fl.Empty() {
		w.RemoveFleet(fl.ID)
		return
	}
	settle(w, fl)
}

// ------------------------------------------------------------------ freight

// unloadAll moves as much of a fleet's cargo as fits into the colony of rock r.
func unloadAll(w *model.World, fl *model.Fleet, r *model.Rock) {
	if r == nil || r.Colony == nil || r.Owner != fl.Owner {
		return
	}
	cargo := fl.CargoTotal()
	for g, amt := range cargo {
		if amt <= 0 {
			continue
		}
		added := r.Colony.AddClamped(model.Good(g), amt)
		fl.UnloadCargo(model.Good(g), added)
	}
}

// loadRoute fills a hauling fleet with the goods its route selects at this
// stop, never drawing a good below the route's reserve.
func loadRoute(fl *model.Fleet, r *model.Rock, rt *model.Route) {
	sel := rt.Loads(r.ID)
	var goods []model.Good
	for g, on := range sel {
		if on {
			goods = append(goods, model.Good(g))
		}
	}
	for pass := 0; pass < 2 && len(goods) > 0; pass++ {
		share := fl.CargoFree() / float64(len(goods))
		if share <= model.StockEpsilon {
			return
		}
		for _, g := range goods {
			avail := r.Colony.Stock[g] - rt.Reserve
			if avail <= 0 {
				continue
			}
			loaded := fl.LoadCargo(g, min(share, avail))
			r.Colony.Take(g, loaded)
		}
	}
}

// stepHaul runs one slow step of a hauling fleet waiting at a stop.
func stepHaul(w *model.World, fl *model.Fleet, r *model.Rock) {
	rt := fl.Route
	if rt == nil {
		settle(w, fl)
		return
	}
	a, b := w.Rock(rt.A), w.Rock(rt.B)
	if a == nil || b == nil || a.Owner != fl.Owner || b.Owner != fl.Owner {
		note(w, fl.Owner, model.SevWarn, fl.At, "A freight route was cancelled: one of its colonies is gone.")
		goHome(w, fl)
		return
	}
	if fl.DwellTimer > 0 {
		fl.DwellTimer -= model.SlowDT
		if fl.DwellTimer > 1e-9 {
			return
		}
	}
	if fl.RetryTimer > 0 {
		fl.RetryTimer -= model.SlowDT
		if fl.RetryTimer > 1e-9 {
			return
		}
	}
	if fl.At != rt.A && fl.At != rt.B {
		if !tryLaunch(w, fl, rt.A) {
			fl.RetryTimer = model.LaunchRetrySeconds
		}
		return
	}
	unloadAll(w, fl, r)
	loadRoute(fl, r, rt)
	other := rt.Other(fl.At)
	otherLoads := false
	for _, on := range rt.Loads(other) {
		otherLoads = otherLoads || on
	}
	if fl.CargoUsed() <= model.StockEpsilon && !otherLoads {
		fl.RetryTimer = model.LaunchRetrySeconds // nothing to carry yet
		return
	}
	if !tryLaunch(w, fl, other) {
		// Put the goods back while waiting for Fuel.
		unloadAll(w, fl, r)
		fl.RetryTimer = model.LaunchRetrySeconds
	}
}

// ------------------------------------------------------------------ queries

// Sensor is one source of sensor coverage: a position in the belt and the
// radius within which enemy fleets are seen.
type Sensor struct {
	Pos   model.Vec2
	Range float64
}

// Sensors returns faction f's sensor coverage at the current tick: one entry
// per owned rock (Keystone, or a working Farseer Array) and one per fleet.
func Sensors(w *model.World, f model.FactionID) []Sensor {
	mul := w.SensorMul()
	var out []Sensor
	for _, r := range w.Rocks {
		if r.Owner != f {
			continue
		}
		best := 0.0
		for _, b := range r.Buildings {
			def := b.Def()
			if def.SensorRange <= 0 || b.Owner != f {
				continue
			}
			if b.Kind == model.Keystone || b.Working() {
				best = max(best, def.SensorRange)
			}
		}
		if best > 0 {
			out = append(out, Sensor{w.RockPos(r.ID), best * mul})
		}
	}
	for _, fl := range w.Fleets {
		if fl.Owner != f {
			continue
		}
		best := 0.0
		for i := range fl.Ships {
			best = max(best, fl.Ships[i].Def().SensorRange)
		}
		if best > 0 {
			out = append(out, Sensor{w.FleetPos(fl), best * mul})
		}
	}
	return out
}

// Visible reports whether faction viewer can currently see the fleet: its own
// fleets always, others only inside one of its sensor radii.
func Visible(w *model.World, viewer model.FactionID, fl *model.Fleet) bool {
	if fl == nil {
		return false
	}
	if fl.Owner == viewer {
		return true
	}
	return covered(Sensors(w, viewer), w.FleetPos(fl))
}

// VisibleFleets returns every fleet faction viewer can currently see.
func VisibleFleets(w *model.World, viewer model.FactionID) []*model.Fleet {
	sensors := Sensors(w, viewer)
	var out []*model.Fleet
	for _, fl := range w.Fleets {
		if fl.Owner == viewer || covered(sensors, w.FleetPos(fl)) {
			out = append(out, fl)
		}
	}
	return out
}

func covered(sensors []Sensor, p model.Vec2) bool {
	for _, s := range sensors {
		if s.Pos.DistSq(p) <= s.Range*s.Range {
			return true
		}
	}
	return false
}

// TravelTime estimates, in game-seconds, how long a flight at the given speed
// from pos to rock takes, chasing the rock along its orbit. ok is false when
// the rock does not exist, the speed is not positive or the flight would take
// longer than an hour.
func TravelTime(w *model.World, pos model.Vec2, rock model.RockID, speed float64) (seconds float64, ok bool) {
	r := w.Rock(rock)
	if r == nil || speed <= 0 {
		return 0, false
	}
	const stepS = 1.0
	stepTicks := model.SecondsToTicks(stepS)
	tick := w.Tick
	for t := 0.0; t < 3600; t += stepS {
		target := r.PosAt(tick)
		d := pos.Dist(target)
		if d <= model.ArriveRadius {
			return t, true
		}
		if d <= speed*stepS {
			return t + d/speed, true
		}
		pos = pos.Toward(target, speed*stepS)
		tick += stepTicks
	}
	return 0, false
}

// ETA estimates the time in game-seconds until a fleet in transit arrives.
func ETA(w *model.World, fl *model.Fleet) (seconds float64, ok bool) {
	if fl == nil || !fl.InTransit() {
		return 0, false
	}
	return TravelTime(w, fl.Pos, fl.Dest, fl.Speed(w.Mods(fl.Owner)))
}
