package model

import (
	"errors"
	"fmt"
)

// ValidateContent checks that the content tables are complete and mutually
// consistent. It returns nil when everything is in order, and otherwise an
// error joining every problem found. It is run by the package tests and may
// be called by tools at start-up as a cheap sanity check.
//
// It verifies, among other things, that:
//
//   - every Good, BuildingKind, ShipKind and TechID has a table entry whose
//     key matches its index, with a unique non-empty name and a description;
//   - costs, rates and hit points are non-negative and sensible;
//   - a start-unlocked building has no unlock tech, and every other building
//     (except the Keystone) and every ship is unlocked by exactly one tech,
//     which is the one named by its Unlock field;
//   - tech prerequisites are valid, free of duplicates, of lower-or-equal
//     tier, and acyclic;
//   - every enumeration value has a distinct display name.
func ValidateContent() error {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	checkGoods(fail)
	checkBuildings(fail)
	checkShips(fail)
	checkTechs(fail)
	checkUnlocks(fail)
	checkEnumNames(fail)

	return errors.Join(errs...)
}

// failFunc records one content problem.
type failFunc func(format string, args ...any)

// checkStockNonNegative reports negative amounts in s.
func checkStockNonNegative(fail failFunc, what string, s Stock) {
	for g, v := range s {
		if v < 0 {
			fail("%s: negative %s amount %v", what, Good(g), v)
		}
	}
}

// checkGoods validates the goods table.
func checkGoods(fail failFunc) {
	names := map[string]bool{}
	shorts := map[string]bool{}
	for i, info := range goodInfos {
		g := Good(i)
		if info.Good != g {
			fail("good %d: table entry has Good %d", i, info.Good)
		}
		if info.Name == "" || info.Short == "" || info.Description == "" {
			fail("good %d: missing name, short name or description", i)
		}
		if info.Name != goodNames[i] {
			fail("good %d: name %q does not match String() %q", i, info.Name, goodNames[i])
		}
		if names[info.Name] {
			fail("good %d: duplicate name %q", i, info.Name)
		}
		names[info.Name] = true
		if shorts[info.Short] {
			fail("good %d: duplicate short name %q", i, info.Short)
		}
		shorts[info.Short] = true
		if !(info.BaseValue > 0) {
			fail("good %s: base value must be positive", info.Name)
		}
		if info.Raw != g.Raw() {
			fail("good %s: Raw flag disagrees with Good.Raw", info.Name)
		}
	}
}

// checkBuildings validates the building table.
func checkBuildings(fail failFunc) {
	names := map[string]bool{}
	for i, d := range buildingDefs {
		k := BuildingKind(i)
		what := fmt.Sprintf("building %d (%s)", i, d.Name)
		if d.Kind != k {
			fail("%s: table entry has Kind %d", what, d.Kind)
		}
		if d.Name == "" || d.Role == "" || d.Description == "" {
			fail("%s: missing name, role or description", what)
		}
		if names[d.Name] {
			fail("%s: duplicate name", what)
		}
		names[d.Name] = true
		if !d.Category.Valid() {
			fail("%s: invalid category %d", what, d.Category)
		}
		checkStockNonNegative(fail, what+" cost", d.Cost)
		checkStockNonNegative(fail, what+" inputs", d.Inputs)
		checkStockNonNegative(fail, what+" outputs", d.Outputs)
		if !(d.MaxHP > 0) {
			fail("%s: MaxHP must be positive", what)
		}
		if d.Staff < 0 || d.MaxPerRock < 0 || d.Berths < 0 {
			fail("%s: negative staff, per-rock limit or berths", what)
		}
		if k == Keystone {
			if d.BuildTime != 0 || !d.Cost.IsZero() || d.StartUnlocked || d.Unlock != TechNone {
				fail("%s: the Keystone must be free, instant and not command-buildable", what)
			}
			if d.MaxPerRock != 1 {
				fail("%s: the Keystone limit must be 1 per rock", what)
			}
		} else {
			if !(d.BuildTime > 0) {
				fail("%s: BuildTime must be positive", what)
			}
			if d.Cost.IsZero() {
				fail("%s: must have a cost", what)
			}
			if !d.CommandBuildable() {
				fail("%s: neither start-unlocked nor unlocked by a tech", what)
			}
		}
		if d.StartUnlocked && d.Unlock != TechNone {
			fail("%s: start-unlocked building must have Unlock == TechNone", what)
		}
		if d.Unlock != TechNone && !d.Unlock.Valid() {
			fail("%s: invalid unlock tech %d", what, d.Unlock)
		}
		if d.SolarPower && !(d.Power > 0) {
			fail("%s: SolarPower requires positive Power", what)
		}
		if d.IsTurret() {
			for h, m := range d.HullMul {
				if !(m > 0) {
					fail("%s: turret needs a positive multiplier for hull %s", what, Hull(h))
				}
			}
		} else if d.HullMul != ([NumHulls]float64{}) {
			fail("%s: hull multipliers on a non-turret", what)
		}
		if (d.ShieldPool > 0) != (d.ShieldRadius > 0) || (d.ShieldPool > 0) != (d.ShieldRegen > 0) {
			fail("%s: shield pool, regen and radius must be set together", what)
		}
		if (d.RepairRate > 0) != (d.RepairRadius > 0) {
			fail("%s: repair rate and radius must be set together", what)
		}
		if (d.SurveyRadius > 0) != (d.SurveyInterval > 0) {
			fail("%s: survey radius and interval must be set together", what)
		}
		if (d.DriverRange > 0) != (d.DriverDamage > 0) || (d.DriverRange > 0) != (d.DriverCooldown > 0) {
			fail("%s: driver range, damage and cooldown must be set together", what)
		}
		if d.Produces() && d.Inputs.IsZero() {
			fail("%s: producer without inputs", what)
		}
		if d.DefaultPriority() < PriorityLow || d.DefaultPriority() > PriorityHigh {
			fail("%s: default priority out of range", what)
		}
	}
}

// checkShips validates the ship table.
func checkShips(fail failFunc) {
	names := map[string]bool{}
	for i, d := range shipDefs {
		k := ShipKind(i)
		what := fmt.Sprintf("ship %d (%s)", i, d.Name)
		if d.Kind != k {
			fail("%s: table entry has Kind %d", what, d.Kind)
		}
		if d.Name == "" || d.Role == "" || d.Description == "" {
			fail("%s: missing name, role or description", what)
		}
		if names[d.Name] {
			fail("%s: duplicate name", what)
		}
		names[d.Name] = true
		if !d.Hull.Valid() {
			fail("%s: invalid hull %d", what, d.Hull)
		}
		checkStockNonNegative(fail, what+" cost", d.Cost)
		if d.Cost.IsZero() {
			fail("%s: must have a cost", what)
		}
		if !(d.BuildTime > 0) || !(d.MaxHP > 0) || !(d.Speed > 0) || !(d.FuelPerU > 0) {
			fail("%s: build time, HP, speed and fuel use must be positive", what)
		}
		if d.Berths < 1 {
			fail("%s: must occupy at least one berth", what)
		}
		if d.DPSShips < 0 || d.DPSBuildings < 0 || d.Cargo < 0 {
			fail("%s: negative damage or cargo", what)
		}
		if !(d.SensorRange > 0) {
			fail("%s: sensor range must be positive", what)
		}
		if !d.Unlock.Valid() {
			fail("%s: invalid unlock tech %d", what, d.Unlock)
		}
		if d.BonusMul == 0 {
			if d.BonusVs != ShipNone {
				fail("%s: BonusVs set without BonusMul", what)
			}
		} else if !d.BonusVs.Valid() || d.BonusMul < 0 {
			fail("%s: invalid damage bonus", what)
		}
	}
}

// checkTechs validates the technology table and its prerequisite graph.
func checkTechs(fail failFunc) {
	names := map[string]bool{}
	for i, d := range techDefs {
		t := TechID(i)
		what := fmt.Sprintf("tech %d (%s)", i, d.Name)
		if d.ID != t {
			fail("%s: table entry has ID %d", what, d.ID)
		}
		if d.Name == "" || d.EffectText == "" || d.Description == "" {
			fail("%s: missing name, effect text or description", what)
		}
		if names[d.Name] {
			fail("%s: duplicate name", what)
		}
		names[d.Name] = true
		if d.Tier < 1 || d.Tier > 3 {
			fail("%s: tier %d out of range", what, d.Tier)
		}
		if !(d.Cost > 0) || d.Lumen < 0 {
			fail("%s: cost must be positive and Lumen non-negative", what)
		}
		seen := map[TechID]bool{}
		for _, req := range d.Requires {
			switch {
			case !req.Valid():
				fail("%s: invalid prerequisite %d", what, req)
				continue
			case req == t:
				fail("%s: requires itself", what)
			case seen[req]:
				fail("%s: duplicate prerequisite %s", what, req)
			case techDefs[req].Tier > d.Tier:
				fail("%s: prerequisite %s has a higher tier", what, req)
			}
			seen[req] = true
		}
		if len(d.UnlockBuildings) == 0 && len(d.UnlockShips) == 0 && d.Effect.IsZero() {
			fail("%s: grants nothing", what)
		}
		for _, k := range d.UnlockBuildings {
			if !k.Valid() {
				fail("%s: unlocks invalid building %d", what, k)
			}
		}
		for _, k := range d.UnlockShips {
			if !k.Valid() {
				fail("%s: unlocks invalid ship %d", what, k)
			}
		}
	}

	// Cycle detection by depth-first search: 0 = unvisited, 1 = on the
	// current path, 2 = finished.
	var state [NumTechs]int8
	var visit func(t TechID) bool
	visit = func(t TechID) bool {
		switch state[t] {
		case 1:
			return false
		case 2:
			return true
		}
		state[t] = 1
		for _, req := range techDefs[t].Requires {
			if req.Valid() && !visit(req) {
				return false
			}
		}
		state[t] = 2
		return true
	}
	for i := range techDefs {
		if !visit(TechID(i)) {
			fail("tech %d (%s): prerequisite cycle", i, techDefs[i].Name)
			break
		}
	}
}

// checkUnlocks validates that tech unlock lists and the Unlock fields of
// buildings and ships describe the same relation.
func checkUnlocks(fail failFunc) {
	var bCount [NumBuildingKinds]int
	var bTech [NumBuildingKinds]TechID
	var sCount [NumShipKinds]int
	var sTech [NumShipKinds]TechID
	for i, d := range techDefs {
		for _, k := range d.UnlockBuildings {
			if k.Valid() {
				bCount[k]++
				bTech[k] = TechID(i)
			}
		}
		for _, k := range d.UnlockShips {
			if k.Valid() {
				sCount[k]++
				sTech[k] = TechID(i)
			}
		}
	}
	for i, d := range buildingDefs {
		switch {
		case d.StartUnlocked || BuildingKind(i) == Keystone:
			if bCount[i] != 0 {
				fail("building %s: unlocked from the start (or never) but also listed by a tech", d.Name)
			}
		case bCount[i] != 1:
			fail("building %s: must be unlocked by exactly one tech, found %d", d.Name, bCount[i])
		case bTech[i] != d.Unlock:
			fail("building %s: Unlock is %s but tech %s lists it", d.Name, d.Unlock, bTech[i])
		}
	}
	for i, d := range shipDefs {
		switch {
		case sCount[i] != 1:
			fail("ship %s: must be unlocked by exactly one tech, found %d", d.Name, sCount[i])
		case sTech[i] != d.Unlock:
			fail("ship %s: Unlock is %s but tech %s lists it", d.Name, d.Unlock, sTech[i])
		}
	}
}

// checkEnumNames validates that every value of every enumeration has a
// distinct, non-empty display name.
func checkEnumNames(fail failFunc) {
	check := func(typ string, n int, name func(i int) string) {
		seen := map[string]bool{}
		for i := 0; i < n; i++ {
			s := name(i)
			if s == "" {
				fail("%s(%d): empty name", typ, i)
			}
			if seen[s] {
				fail("%s(%d): duplicate name %q", typ, i, s)
			}
			seen[s] = true
		}
	}
	check("Good", NumGoods, func(i int) string { return Good(i).String() })
	check("Terrain", NumTerrains, func(i int) string { return Terrain(i).String() })
	check("RockClass", NumRockClasses, func(i int) string { return RockClass(i).String() })
	check("Category", NumCategories, func(i int) string { return Category(i).String() })
	check("BuildState", NumBuildStates, func(i int) string { return BuildState(i).String() })
	check("BuildingKind", NumBuildingKinds, func(i int) string { return BuildingKind(i).String() })
	check("IdleReason", NumIdleReasons, func(i int) string { return IdleReason(i).String() })
	check("Hull", NumHulls, func(i int) string { return Hull(i).String() })
	check("ShipKind", NumShipKinds, func(i int) string { return ShipKind(i).String() })
	check("OrderKind", NumOrderKinds, func(i int) string { return OrderKind(i).String() })
	check("Stance", NumStances, func(i int) string { return Stance(i).String() })
	check("TechID", NumTechs, func(i int) string { return TechID(i).String() })
	check("CmdKind", NumCmdKinds, func(i int) string { return CmdKind(i).String() })
	check("Severity", NumSeverities, func(i int) string { return Severity(i).String() })
	check("FXKind", NumFXKinds, func(i int) string { return FXKind(i).String() })
	check("EventKind", NumEventKinds, func(i int) string { return EventKind(i).String() })
	check("Personality", NumPersonalities, func(i int) string { return Personality(i).String() })
	check("Difficulty", NumDifficulties, func(i int) string { return Difficulty(i).String() })
	check("BeltSize", NumBeltSizes, func(i int) string { return BeltSize(i).String() })

	ranks := map[int]bool{}
	for i := 0; i < NumCategories; i++ {
		r := Category(i).Rank()
		if r < 0 || r >= NumCategories || ranks[r] {
			fail("Category(%d): rank %d is out of range or duplicated", i, r)
		}
		ranks[r] = true
	}
}
