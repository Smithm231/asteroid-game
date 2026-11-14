package sim

import (
	"errors"
	"math"

	"drifthold/internal/model"
)

// CanTrade reports whether the rock has an operating Exchange Dock, which
// market trading and settler sponsorship need.
func CanTrade(r *model.Rock) bool {
	if r == nil || r.Colony == nil {
		return false
	}
	for _, b := range r.Buildings {
		if b.Def().Trade && b.Working() {
			return true
		}
	}
	return false
}

// BuyCost returns the Scrip price of buying units of good g right now.
func BuyCost(w *model.World, g model.Good, units float64) float64 {
	return math.Ceil(w.Market.BuyPrice(g)*units*100) / 100
}

// SellRevenue returns the Scrip paid for selling units of good g right now.
func SellRevenue(w *model.World, g model.Good, units float64) float64 {
	return math.Floor(w.Market.SellPrice(g)*units*100) / 100
}

// Trade buys (positive amount) or sells (negative amount) a good at one of
// faction f's rocks.
func Trade(w *model.World, f model.FactionID, rock model.RockID, g model.Good, amount float64) error {
	fac, err := w.Issuer(f)
	if err != nil {
		return err
	}
	r, err := w.OwnedRock(f, rock)
	if err != nil {
		return err
	}
	units := math.Abs(amount)
	if !g.Valid() || units <= 0 || units > model.MaxTradeUnits || math.IsNaN(amount) {
		return model.ErrBadCommand
	}
	if !CanTrade(r) {
		return errors.New("Trading needs a working Exchange Dock on this rock")
	}
	c := r.Colony
	if amount > 0 {
		if c.Free(g) < units-model.StockEpsilon {
			return errors.New("Not enough storage for that purchase")
		}
		price := BuyCost(w, g, units)
		if !fac.SpendScrip(price) {
			return &model.ScripError{Need: price}
		}
		c.AddClamped(g, units)
		w.Market.AddPressure(g, units*model.MarketPressurePerUnit)
		fac.Stats.Bought[g] += units
	} else {
		if c.Stock[g] < units-model.StockEpsilon {
			return &model.ShortageError{Good: g, Need: units}
		}
		c.Take(g, units)
		fac.AddScrip(SellRevenue(w, g, units))
		w.Market.AddPressure(g, -units*model.MarketPressurePerUnit)
		fac.Stats.Sold[g] += units
	}
	fx := model.NewFX(model.FXTrade, rock, model.Hex{}, f)
	fx.Amount = amount
	w.Emit(fx)
	return nil
}

// Sponsor pays for a group of settlers to join one of faction f's colonies.
func Sponsor(w *model.World, f model.FactionID, rock model.RockID) error {
	fac, err := w.Issuer(f)
	if err != nil {
		return err
	}
	r, err := w.OwnedRock(f, rock)
	if err != nil {
		return err
	}
	if !CanTrade(r) {
		return errors.New("Sponsoring settlers needs a working Exchange Dock on this rock")
	}
	c := r.Colony
	if !c.SponsorReady(w.Tick) {
		return errors.New("Settlers were sponsored here too recently")
	}
	if c.Pop+model.SponsorColonists > c.Housing {
		return errors.New("Not enough free housing for new settlers")
	}
	if !fac.SpendScrip(model.SponsorCost) {
		return &model.ScripError{Need: model.SponsorCost}
	}
	c.Pop += model.SponsorColonists
	c.MarkSponsored(w.Tick)
	fac.Stats.SettlersSponsored += int(model.SponsorColonists)
	fx := model.NewFX(model.FXTrade, rock, model.Hex{}, f)
	w.Emit(fx)
	return nil
}

// stepMarket lets price pressure decay and wander by dt game-seconds.
func stepMarket(w *model.World, dt float64) {
	decay := math.Pow(1-model.MarketDecayPerSecond, dt)
	for g := range w.Market.Pressure {
		p := w.Market.Pressure[g]*decay + w.RNG.Range(-model.MarketWalkStep, model.MarketWalkStep)
		w.Market.Pressure[g] = model.Clamp(p, -model.MarketPressureMax, model.MarketPressureMax)
	}
}
