package model

// ShipDef is the content-table entry of a ship kind.
type ShipDef struct {
	// Kind is the ship kind this entry describes.
	Kind ShipKind
	// Name is the display name, e.g. "Cutter".
	Name string
	// Hull is the weight class, which turrets apply multipliers to.
	Hull Hull
	// Cost is deducted from the cradle's colony stock when the ship is
	// queued (and refunded in full when the queue entry is cancelled).
	Cost Stock
	// BuildTime is the construction time in game-seconds.
	BuildTime float64
	// Berths is the number of berths the ship occupies, from the moment it
	// is queued.
	Berths int
	// MaxHP is the hit points before Mods.HPMul.
	MaxHP float64
	// Speed is the cruise speed in u per game-second before
	// Mods.ShipSpeedMul.
	Speed float64
	// DPSShips is the damage per second against ships.
	DPSShips float64
	// DPSBuildings is the damage per second against buildings.
	DPSBuildings float64
	// FuelPerU is the Fuel burned per u of distance when leaving a rock
	// owned by the ship's faction.
	FuelPerU float64
	// Unlock is the technology that unlocks the kind.
	Unlock TechID
	// Cargo is the cargo capacity in units (Hauler), 0 for other kinds.
	Cargo float64
	// SensorRange is the radius in u within which the ship reveals enemy
	// fleets.
	SensorRange float64
	// BonusVs is the ship kind this ship deals bonus damage to, or ShipNone.
	BonusVs ShipKind
	// BonusMul is the damage multiplier against BonusVs (0 when there is no
	// bonus).
	BonusMul float64
	// CanSurvey is true when the ship can carry out a Survey order
	// (Prospector).
	CanSurvey bool
	// CanColonize is true when the ship can carry out a Colonize order and
	// is consumed by it (Seedship).
	CanColonize bool
	// Role is a one-line functional summary for tooltips.
	Role string
	// Description is the codex text: one or two sentences.
	Description string
}

// Armed reports whether the ship has any weapon (damage against ships or
// against buildings).
func (d ShipDef) Armed() bool { return d.DPSShips > 0 || d.DPSBuildings > 0 }

// Bomber reports whether the ship deals more damage to buildings than to
// ships (Hammer); such ships bombard from the start of an attack.
func (d ShipDef) Bomber() bool { return d.DPSBuildings > d.DPSShips }

// MulVs returns the damage multiplier of this ship against a ship of kind
// target: BonusMul against BonusVs, 1 otherwise.
func (d ShipDef) MulVs(target ShipKind) float64 {
	if d.BonusMul > 0 && target == d.BonusVs && target.Valid() {
		return d.BonusMul
	}
	return 1
}

// shipDefs is the ship table, indexed by ShipKind.
var shipDefs = [NumShipKinds]ShipDef{
	Prospector: {
		Kind: Prospector, Name: "Prospector", Hull: HullLight,
		Cost: Stock{Alloy: 30, Parts: 10}, BuildTime: 25, Berths: 1, MaxHP: 60, Speed: 9,
		FuelPerU: 0.010, Unlock: OrbitalFlight,
		SensorRange: 220, BonusVs: ShipNone, CanSurvey: true,
		Role:        "Surveys a rock from orbit, revealing every deposit; sensors 220 u.",
		Description: "A quick scout packed with sounding gear. Fifteen seconds in orbit charts an entire rock.",
	},
	Hauler: {
		Kind: Hauler, Name: "Hauler", Hull: HullMedium,
		Cost: Stock{Alloy: 50, Parts: 15}, BuildTime: 35, Berths: 1, MaxHP: 120, Speed: 5,
		FuelPerU: 0.020, Unlock: OrbitalFlight,
		Cargo: 150, SensorRange: DefaultShipSensorRange, BonusVs: ShipNone,
		Role:        "Carries 150 units of cargo along freight routes.",
		Description: "A plain cargo frame that shuttles goods between your colonies, stop after stop.",
	},
	Seedship: {
		Kind: Seedship, Name: "Seedship", Hull: HullMedium,
		Cost: Stock{Alloy: 160, Silica: 60, Parts: 60}, BuildTime: 90, Berths: 2, MaxHP: 200, Speed: 4,
		FuelPerU: 0.040, Unlock: ColonialCharter,
		SensorRange: DefaultShipSensorRange, BonusVs: ShipNone, CanColonize: true,
		Role:        "Founds a Keystone on an unowned rock and is consumed; takes 12 colonists with it.",
		Description: "Carries twelve colonists and a folded Keystone. It lands once and never lifts again.",
	},
	Dart: {
		Kind: Dart, Name: "Dart", Hull: HullLight,
		Cost: Stock{Alloy: 25, Parts: 8}, BuildTime: 20, Berths: 1, MaxHP: 50, Speed: 10,
		DPSShips: 6, DPSBuildings: 1,
		FuelPerU: 0.008, Unlock: PointDefence,
		SensorRange: DefaultShipSensorRange, BonusVs: Hammer, BonusMul: 1.5,
		Role:        "Interceptor; x1.5 damage against Hammers.",
		Description: "A fast, fragile interceptor built to run down bombers before they reach the surface.",
	},
	Cutter: {
		Kind: Cutter, Name: "Cutter", Hull: HullMedium,
		Cost: Stock{Alloy: 60, Parts: 20}, BuildTime: 40, Berths: 2, MaxHP: 160, Speed: 7,
		DPSShips: 10, DPSBuildings: 5,
		FuelPerU: 0.020, Unlock: Gunnery,
		SensorRange: DefaultShipSensorRange, BonusVs: ShipNone,
		Role:        "Line gunship, effective against other ships.",
		Description: "A dependable gunship, the backbone of most battle fleets.",
	},
	Hammer: {
		Kind: Hammer, Name: "Hammer", Hull: HullMedium,
		Cost: Stock{Alloy: 70, Parts: 25}, BuildTime: 45, Berths: 2, MaxHP: 120, Speed: 6,
		DPSShips: 2, DPSBuildings: 22,
		FuelPerU: 0.025, Unlock: SiegeDoctrine,
		SensorRange: DefaultShipSensorRange, BonusVs: ShipNone,
		Role:        "Bomber: heavy damage to buildings, almost none to ships.",
		Description: "A slow bomber that cracks surface structures and little else. Escort it or lose it.",
	},
	Bulwark: {
		Kind: Bulwark, Name: "Bulwark", Hull: HullHeavy,
		Cost: Stock{Alloy: 180, Parts: 60, Lumen: 15}, BuildTime: 100, Berths: 4, MaxHP: 600, Speed: 4,
		DPSShips: 22, DPSBuildings: 12,
		FuelPerU: 0.050, Unlock: CapitalHulls,
		SensorRange: DefaultShipSensorRange, BonusVs: ShipNone,
		Role:        "Heavy frigate: strong against ships and buildings alike.",
		Description: "A heavy frigate with thick plate and guns to match. It anchors a fleet and takes a long time to kill.",
	},
}

// Ships returns the ship table in ShipKind order. The slice is shared and
// read-only: do not modify it.
func Ships() []ShipDef { return shipDefs[:] }

// ShipDefOf returns the table entry of kind. For an invalid kind it returns
// an entry with Kind == ShipNone, Name "Unknown", Unlock == TechNone and
// BonusVs == ShipNone.
func ShipDefOf(kind ShipKind) ShipDef {
	if !kind.Valid() {
		return ShipDef{Kind: ShipNone, Name: "Unknown", Unlock: TechNone, BonusVs: ShipNone}
	}
	return shipDefs[kind]
}
