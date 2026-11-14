package sim

import (
	"errors"
	"slices"

	"drifthold/internal/model"
)

// TargetRank returns the bombardment rank of a building kind under a stance:
// lower ranks are hit first.
//
//	Suppress: defences, then the Keystone, then everything else
//	Raid:     industry and power, then defences, then the Keystone, then the rest
//	Raze:     the Keystone, then defences, then everything else
func TargetRank(s model.Stance, def model.BuildingDef) int {
	key := def.Kind == model.Keystone
	switch s {
	case model.StanceRaid:
		switch {
		case def.Category == model.CatIndustry || def.Category == model.CatPower:
			return 0
		case def.IsDefence():
			return 1
		case key:
			return 2
		}
		return 3
	case model.StanceRaze:
		switch {
		case key:
			return 0
		case def.IsDefence():
			return 1
		}
		return 2
	default:
		switch {
		case def.IsDefence():
			return 0
		case key:
			return 1
		}
		return 2
	}
}

// shotCadence spaces out the cosmetic "shot" FX of one shooter: the damage is
// applied every tick, but only every eighth tick emits a cue.
func shotCadence(w *model.World, id int) bool { return (w.Tick+id*3)%8 == 0 }

// damageBuilding applies dmg to a building on rock r, letting covering Aegis
// Projectors absorb it first. attacker is the faction responsible, or
// NoFaction for natural causes. It reports whether the building was destroyed.
func damageBuilding(w *model.World, r *model.Rock, b *model.Building, dmg float64, attacker model.FactionID) bool {
	if b == nil || dmg <= 0 {
		return false
	}
	owner := b.Owner
	if owner != model.NoFaction {
		for _, p := range r.Buildings {
			pd := p.Def()
			if dmg <= 0 {
				break
			}
			if pd.ShieldPool <= 0 || p.Owner != owner || p.Shield <= 0 || !p.Operational() || !p.Powered ||
				p.Hex.Dist(b.Hex) > pd.ShieldRadius {
				continue
			}
			absorbed := min(dmg, p.Shield)
			p.Shield -= absorbed
			dmg -= absorbed
			if shotCadence(w, int(p.ID)) {
				w.Emit(model.NewFX(model.FXShieldHit, r.ID, b.Hex, owner))
			}
		}
	}
	if dmg <= 0 {
		return false
	}
	dealt := min(dmg, b.HP)
	b.HP -= dmg
	if attacker != model.NoFaction {
		if r.Colony != nil && owner != model.NoFaction {
			r.Colony.MarkAttacked(w.Tick)
		}
		if af := w.Faction(attacker); af != nil {
			af.Stats.DamageDealt += dealt
		}
	}
	if of := w.Faction(owner); of != nil {
		of.Stats.DamageTaken += dealt
	}
	if b.HP > 0 {
		return false
	}
	destroyBuilding(w, r, b, attacker)
	return true
}

// destroyBuilding removes a building, leaves rubble, and deals with the
// consequences: a destroyed Keystone ends the colony and may eliminate its
// faction and end the game.
func destroyBuilding(w *model.World, r *model.Rock, b *model.Building, attacker model.FactionID) {
	owner := b.Owner
	r.RemoveBuilding(b.ID)
	if t := r.TileAt(b.Hex); t != nil {
		t.Rubble = true
	}
	fx := model.NewFX(model.FXBuildDestroyed, r.ID, b.Hex, owner)
	fx.Amount = float64(b.Kind)
	w.Emit(fx)
	if of := w.Faction(owner); of != nil {
		of.Stats.BuildingsLost++
	}
	af := w.Faction(attacker)
	if af != nil && attacker != owner {
		af.Stats.BuildingsDestroyed++
	}
	if b.Kind != model.Keystone || owner == model.NoFaction {
		note(w, owner, model.SevAlert, r.ID, "%s destroyed on %s.", b.Def().Name, r.Name)
		return
	}

	// The colony falls.
	r.DissolveColony()
	w.Emit(model.NewFX(model.FXColonyFell, r.ID, b.Hex, owner))
	of := w.Faction(owner)
	ownerName := "A Hold"
	if of != nil {
		of.Stats.ColoniesLost++
		ownerName = of.Name
	}
	if af != nil && attacker != owner {
		af.Stats.KeystonesDestroyed++
		af.AddScrip(model.KeystoneBounty)
		w.Logf(model.NoFaction, model.SevAlert, r.ID, "%s has lost its colony on %s to %s.", ownerName, r.Name, af.Name)
	} else {
		w.Logf(model.NoFaction, model.SevAlert, r.ID, "%s has lost its colony on %s.", ownerName, r.Name)
	}
	for _, fl := range w.Fleets {
		if fl.Owner == owner && fl.At == r.ID {
			fl.Garrison = false
		}
	}
	for _, o := range w.Rocks {
		for _, ob := range o.Buildings {
			if ob.DriverTarget == r.ID {
				ob.DriverTarget = model.NoRock
			}
		}
	}
	checkElimination(w, owner)
}

// checkElimination removes a faction that has lost its last Keystone and ends
// the game when a winner is decided.
func checkElimination(w *model.World, f model.FactionID) {
	fac := w.Faction(f)
	if fac == nil || !fac.Alive || w.ColonyCount(f) > 0 {
		return
	}
	fac.Alive = false
	fac.EliminatedTick = w.Tick
	fac.Research.Current = model.TechNone
	fac.Research.Queue = nil
	for _, fl := range slices.Clone(w.Fleets) {
		if fl.Owner == f {
			w.RemoveFleet(fl.ID)
		}
	}
	w.Slugs = slices.DeleteFunc(w.Slugs, func(s model.Slug) bool { return s.Owner == f })
	w.Logf(model.NoFaction, model.SevAlert, model.NoRock, "%s has been eliminated.", fac.Name)
	checkVictory(w)
}

// checkVictory ends the game when one faction remains, or when the last human
// player has been eliminated.
func checkVictory(w *model.World) {
	if w.GameOver {
		return
	}
	alive, humans, hadHuman := 0, 0, false
	last := model.NoFaction
	for _, f := range w.Factions {
		if !f.IsAI {
			hadHuman = true
		}
		if f.Alive {
			alive++
			last = f.ID
			if !f.IsAI {
				humans++
			}
		}
	}
	switch {
	case alive <= 1:
		w.GameOver, w.Winner = true, last
	case hadHuman && humans == 0:
		w.GameOver, w.Winner = true, model.NoFaction
	default:
		return
	}
	w.GameOverTick = w.Tick
	if wf := w.Faction(w.Winner); wf != nil {
		w.Logf(model.NoFaction, model.SevGood, model.NoRock, "%s holds the Shoal.", wf.Name)
	}
}

// pendingHit is damage queued against a ship during a combat tick, applied
// after every shooter has fired so that the order of fleets gives no edge.
type pendingHit struct {
	fleet    *model.Fleet
	ship     model.ShipID
	dmg      float64
	attacker model.FactionID
}

// stepCombat resolves one tick of fighting at every rock where hostile
// parties share an orbit, and moves Mass Driver slugs.
func stepCombat(w *model.World) {
	stepSlugs(w)
	if len(w.Fleets) == 0 {
		return
	}
	byRock := make([][]*model.Fleet, len(w.Rocks))
	any := false
	for _, fl := range w.Fleets {
		fl.Engaged = false
		if fl.At >= 0 && int(fl.At) < len(byRock) && !fl.Empty() {
			byRock[fl.At] = append(byRock[fl.At], fl)
			any = true
		}
	}
	if !any {
		return
	}
	var hits []pendingHit
	for ri, fleets := range byRock {
		if len(fleets) == 0 {
			continue
		}
		r := w.Rocks[ri]
		hits = fightAt(w, r, fleets, hits[:0])
		for _, h := range hits {
			s := h.fleet.Ship(h.ship)
			if s == nil || s.HP <= 0 {
				continue
			}
			dealt := min(h.dmg, s.HP)
			s.HP -= h.dmg
			if af := w.Faction(h.attacker); af != nil {
				af.Stats.DamageDealt += dealt
			}
			if of := w.Faction(h.fleet.Owner); of != nil {
				of.Stats.DamageTaken += dealt
			}
			if s.HP <= 0 {
				kind := s.Kind
				h.fleet.RemoveShip(h.ship)
				if of := w.Faction(h.fleet.Owner); of != nil {
					of.Stats.ShipsLost++
				}
				if af := w.Faction(h.attacker); af != nil {
					af.Stats.ShipsDestroyed++
				}
				fx := model.NewFX(model.FXShipDestroyed, r.ID, model.Hex{}, h.fleet.Owner)
				fx.Fleet, fx.A, fx.Amount = h.fleet.ID, w.RockPos(r.ID), float64(kind)
				w.Emit(fx)
			}
		}
	}
	for _, fl := range slices.Clone(w.Fleets) {
		if fl.Empty() {
			note(w, fl.Owner, model.SevAlert, fl.At, "A fleet was lost at %s.", rockName(w, fl.At))
			w.RemoveFleet(fl.ID)
		}
	}
}

// fightAt lets every ship and turret at rock r fire once. Damage to ships is
// returned as pending hits; damage to buildings is applied directly.
func fightAt(w *model.World, r *model.Rock, fleets []*model.Fleet, hits []pendingHit) []pendingHit {
	owner := r.Owner
	// Is there anything to fight about?
	mixed := false
	for _, fl := range fleets {
		if fl.Owner != fleets[0].Owner || (owner != model.NoFaction && fl.Owner != owner) {
			mixed = true
			break
		}
	}
	if !mixed {
		return hits
	}
	type enemyShip struct {
		fl *model.Fleet
		s  *model.Ship
	}
	enemiesOf := func(f model.FactionID, armedOnly bool) []enemyShip {
		var out []enemyShip
		for _, fl := range fleets {
			if fl.Owner == f {
				continue
			}
			for i := range fl.Ships {
				s := &fl.Ships[i]
				if s.HP > 0 && (!armedOnly || s.Def().Armed()) {
					out = append(out, enemyShip{fl, s})
				}
			}
		}
		return out
	}
	pick := func(f model.FactionID, sticky model.ShipID) (enemyShip, bool) {
		if sticky != 0 {
			for _, fl := range fleets {
				if fl.Owner == f {
					continue
				}
				if s := fl.Ship(sticky); s != nil && s.HP > 0 {
					return enemyShip{fl, s}, true
				}
			}
		}
		c := enemiesOf(f, true)
		if len(c) == 0 {
			c = enemiesOf(f, false)
		}
		if len(c) == 0 {
			return enemyShip{}, false
		}
		return c[w.RNG.Intn(len(c))], true
	}
	jitter := func() float64 { return w.RNG.Range(model.DamageJitterMin, model.DamageJitterMax) }

	// Ships.
	for _, fl := range fleets {
		besieging := fl.Order == model.OrderAttack && fl.Dest == r.ID &&
			owner != model.NoFaction && model.Hostile(owner, fl.Owner)
		enemyShips := len(enemiesOf(fl.Owner, false)) > 0
		for i := range fl.Ships {
			s := &fl.Ships[i]
			def := s.Def()
			if s.HP <= 0 || !def.Armed() {
				continue
			}
			bombs := besieging && def.DPSBuildings > 0 && (def.Bomber() || !enemyShips)
			if bombs {
				b := r.BuildingByID(s.TargetBuilding)
				if b == nil || b.Owner != owner {
					b = pickBuilding(w, r, owner, fl.Stance)
					if b == nil {
						continue
					}
					s.TargetBuilding = b.ID
				}
				fl.Engaged = true
				if shotCadence(w, int(s.ID)) {
					w.Emit(model.NewFX(model.FXBomb, r.ID, b.Hex, fl.Owner))
				}
				if damageBuilding(w, r, b, def.DPSBuildings*model.TickDT*jitter(), fl.Owner) {
					s.TargetBuilding = 0
					if r.Owner != owner {
						return hits // the colony fell: nothing left to fight over this tick
					}
				}
				continue
			}
			if def.DPSShips <= 0 || !enemyShips {
				continue
			}
			t, ok := pick(fl.Owner, s.TargetShip)
			if !ok {
				continue
			}
			s.TargetShip = t.s.ID
			fl.Engaged, t.fl.Engaged = true, true
			hits = append(hits, pendingHit{t.fl, t.s.ID,
				def.DPSShips * model.TickDT * def.MulVs(t.s.Kind) * jitter(), fl.Owner})
			if shotCadence(w, int(s.ID)) {
				fx := model.NewFX(model.FXShotShip, r.ID, model.Hex{}, fl.Owner)
				fx.Fleet, fx.Amount = fl.ID, float64(t.s.ID)
				w.Emit(fx)
			}
		}
	}

	// Turrets of the rock's owner.
	if owner == model.NoFaction {
		return hits
	}
	for _, b := range r.Buildings {
		def := b.Def()
		if !def.IsTurret() || b.Owner != owner || !b.Working() {
			continue
		}
		t, ok := pick(owner, b.Target)
		if !ok {
			b.Target = 0
			continue
		}
		b.Target = t.s.ID
		t.fl.Engaged = true
		dps := def.DPSVs(t.s.Def().Hull) * min(1, b.Eff)
		hits = append(hits, pendingHit{t.fl, t.s.ID, dps * model.TickDT * jitter(), owner})
		if shotCadence(w, int(b.ID)) {
			kind := model.FXShotFlak
			if b.Kind == model.LanceTower {
				kind = model.FXShotLance
			}
			fx := model.NewFX(kind, r.ID, b.Hex, owner)
			fx.Amount = float64(t.s.ID)
			w.Emit(fx)
		}
	}
	return hits
}

// pickBuilding chooses the next bombardment target on rock r among the
// buildings of owner: a random one of the best rank under the stance.
func pickBuilding(w *model.World, r *model.Rock, owner model.FactionID, stance model.Stance) *model.Building {
	best := 99
	var cands []*model.Building
	for _, b := range r.Buildings {
		if b.Owner != owner || b.Derelict() {
			continue
		}
		rank := TargetRank(stance, b.Def())
		if rank < best {
			best, cands = rank, cands[:0]
		}
		if rank == best {
			cands = append(cands, b)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	return cands[w.RNG.Intn(len(cands))]
}

// ------------------------------------------------------------- mass drivers

// SetDriverTarget aims one of faction f's Mass Drivers at an enemy rock, or
// clears its target when target is NoRock.
func SetDriverTarget(w *model.World, f model.FactionID, rock model.RockID, id model.BuildingID, target model.RockID) error {
	if _, err := w.Issuer(f); err != nil {
		return err
	}
	_, b, err := ownBuilding(w, f, rock, id)
	if err != nil {
		return err
	}
	if b.Def().DriverRange <= 0 {
		return errors.New("That structure is not a Mass Driver")
	}
	if target == model.NoRock {
		b.DriverTarget = model.NoRock
		return nil
	}
	tr := w.Rock(target)
	if tr == nil {
		return model.ErrNoRock
	}
	if tr.Owner == model.NoFaction || !model.Hostile(tr.Owner, f) {
		return errors.New("A Mass Driver can only target a rival colony")
	}
	b.DriverTarget = target
	return nil
}

// stepDrivers runs one slow step of every Mass Driver.
func stepDrivers(w *model.World, dt float64) {
	for _, r := range w.Rocks {
		if r.Colony == nil {
			continue
		}
		for _, b := range r.Buildings {
			def := b.Def()
			if def.DriverRange <= 0 || !b.Operational() || b.Owner != r.Owner {
				continue
			}
			if b.Cooldown > 0 {
				b.Cooldown = max(0, b.Cooldown-dt)
			}
			if b.Eff <= 0 {
				continue
			}
			tr := w.Rock(b.DriverTarget)
			if tr == nil || tr.Owner == model.NoFaction || !model.Hostile(tr.Owner, r.Owner) {
				b.DriverTarget = model.NoRock
				b.Idle = model.IdleNoTarget
				continue
			}
			if w.RockDist(r.ID, tr.ID) > def.DriverRange {
				b.Idle = model.IdleNoTarget // out of range for now
				continue
			}
			if b.Cooldown > 1e-9 {
				continue
			}
			if r.Colony.Stock[model.Alloy] < def.DriverAmmo {
				b.Idle = model.IdleNoInput
				continue
			}
			r.Colony.NoteConsumed(model.Alloy, r.Colony.Take(model.Alloy, def.DriverAmmo))
			b.Cooldown = def.DriverCooldown
			from := w.RockPos(r.ID)
			w.Slugs = append(w.Slugs, model.Slug{Owner: r.Owner, From: r.ID, Target: tr.ID, Pos: from, Damage: def.DriverDamage})
			fx := model.NewFX(model.FXSlugFired, r.ID, b.Hex, r.Owner)
			fx.A, fx.B = from, w.RockPos(tr.ID)
			w.Emit(fx)
		}
	}
}

// stepSlugs moves every Mass Driver slug by one tick and resolves impacts.
func stepSlugs(w *model.World) {
	if len(w.Slugs) == 0 {
		return
	}
	old := w.Slugs
	w.Slugs = nil
	for _, s := range old {
		tr := w.Rock(s.Target)
		if of := w.Faction(s.Owner); tr == nil || of == nil || !of.Alive {
			continue
		}
		target := tr.PosAt(w.Tick)
		s.Pos = s.Pos.Toward(target, model.SlugSpeed*model.TickDT)
		if s.Pos.Dist(target) > model.ArriveRadius {
			w.Slugs = append(w.Slugs, s)
			continue
		}
		if tr.Owner == model.NoFaction || !model.Hostile(tr.Owner, s.Owner) {
			continue // nothing left to hit
		}
		if b := pickBuilding(w, tr, tr.Owner, model.StanceSuppress); b != nil {
			w.Emit(model.NewFX(model.FXSlugImpact, tr.ID, b.Hex, s.Owner))
			damageBuilding(w, tr, b, s.Damage, s.Owner)
		}
	}
}
