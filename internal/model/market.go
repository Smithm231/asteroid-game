package model

// Market is the state of the Exchange, shared by every faction.
type Market struct {
	// Pressure is the price pressure per good, in
	// [-MarketPressureMax, +MarketPressureMax]. The price of a good is
	// base value x (1 + Pressure). Buying raises it, selling lowers it, and
	// it decays toward 0.
	Pressure [NumGoods]float64
}

// Price returns the current mid price of one unit of g in Scrip:
// BaseValue x (1 + Pressure). An invalid good costs 0.
func (m *Market) Price(g Good) float64 {
	if m == nil || !g.Valid() {
		return 0
	}
	return goodInfos[g].BaseValue * (1 + m.Pressure[g])
}

// BuyPrice returns what a faction pays per unit of g when buying:
// Price x MarketBuyMul.
func (m *Market) BuyPrice(g Good) float64 { return m.Price(g) * MarketBuyMul }

// SellPrice returns what a faction receives per unit of g when selling:
// Price x MarketSellMul.
func (m *Market) SellPrice(g Good) float64 { return m.Price(g) * MarketSellMul }

// AddPressure changes the pressure of g by delta and clamps the result to
// [-MarketPressureMax, +MarketPressureMax]. Invalid goods are ignored.
func (m *Market) AddPressure(g Good, delta float64) {
	if m == nil || !g.Valid() {
		return
	}
	m.Pressure[g] = Clamp(m.Pressure[g]+delta, -MarketPressureMax, MarketPressureMax)
}
