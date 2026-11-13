package model

// GoodInfo is the content-table entry of a good.
type GoodInfo struct {
	// Good is the good this entry describes.
	Good Good
	// Name is the display name, e.g. "Alloy".
	Name string
	// Short is a three-letter upper-case abbreviation for dense UI, e.g.
	// "ALY".
	Short string
	// BaseValue is the base market value per unit in Scrip (the price at
	// zero market pressure).
	BaseValue float64
	// Raw is true for mined goods (Ore, Ice, Silica, Lumen), which occur as
	// tile deposits, and false for produced goods.
	Raw bool
	// Description is a one-sentence codex text.
	Description string
}

// goodInfos is the goods table, indexed by Good.
var goodInfos = [NumGoods]GoodInfo{
	Ore: {
		Good: Ore, Name: "Ore", Short: "ORE", BaseValue: 1.0, Raw: true,
		Description: "Metal-bearing rock bored from Ferric and Stony veins; Smelters reduce it to Alloy.",
	},
	Ice: {
		Good: Ice, Name: "Ice", Short: "ICE", BaseValue: 1.2, Raw: true,
		Description: "Frozen volatiles: colonists breathe it, Greenhouses drink it and Crackers turn it into Fuel.",
	},
	Silica: {
		Good: Silica, Name: "Silica", Short: "SIL", BaseValue: 0.8, Raw: true,
		Description: "Glassy mineral sand used in almost every structure and, with Alloy, in machine Parts.",
	},
	Lumen: {
		Good: Lumen, Name: "Lumen", Short: "LUM", BaseValue: 12.0, Raw: true,
		Description: "A rare light-storing crystal found on Glint rocks, needed for the most advanced works.",
	},
	Alloy: {
		Good: Alloy, Name: "Alloy", Short: "ALY", BaseValue: 3.0, Raw: false,
		Description: "Smelted structural metal, the stuff every building and hull is made of.",
	},
	Fuel: {
		Good: Fuel, Name: "Fuel", Short: "FUE", BaseValue: 3.0, Raw: false,
		Description: "Cracked volatiles that feed Thermal Stacks and every fleet leaving your rocks.",
	},
	Parts: {
		Good: Parts, Name: "Parts", Short: "PRT", BaseValue: 9.0, Raw: false,
		Description: "Precision machinery from the Fabricator, required by advanced buildings and all ships.",
	},
	Food: {
		Good: Food, Name: "Food", Short: "FOD", BaseValue: 1.5, Raw: false,
		Description: "Greenhouse produce; every colonist eats, and a hungry colony shrinks.",
	},
}

// Goods returns the goods table in enum order. The slice is shared and
// read-only: do not modify it.
func Goods() []GoodInfo { return goodInfos[:] }

// GoodInfoOf returns the table entry of g. For an invalid good it returns an
// entry with Good == GoodNone, Name "None" and zero value.
func GoodInfoOf(g Good) GoodInfo {
	if !g.Valid() {
		return GoodInfo{Good: GoodNone, Name: "None", Short: "---"}
	}
	return goodInfos[g]
}

// RawGoods returns the four mined goods in enum order: Ore, Ice, Silica,
// Lumen.
func RawGoods() [4]Good { return [4]Good{Ore, Ice, Silica, Lumen} }
