package model

import (
	"errors"
	"math"
	"testing"
)

func TestStockMutators(t *testing.T) {
	s := Stock{Alloy: 10, Silica: 4}
	s.Add(Stock{Alloy: 5, Food: 2})
	if want := (Stock{Alloy: 15, Silica: 4, Food: 2}); s != want {
		t.Errorf("after Add: %v, want %v", s, want)
	}
	s.Sub(Stock{Alloy: 20, Silica: 4})
	if want := (Stock{Alloy: -5, Food: 2}); s != want {
		t.Errorf("after Sub: %v, want %v", s, want)
	}
	s.ClampMin0()
	if want := (Stock{Food: 2}); s != want {
		t.Errorf("after ClampMin0: %v, want %v", s, want)
	}
	s.Scale(3.5)
	if want := (Stock{Food: 7}); s != want {
		t.Errorf("after Scale: %v, want %v", s, want)
	}
	s.Clear()
	if !s.IsZero() {
		t.Errorf("after Clear: %v", s)
	}
}

func TestStockPureOps(t *testing.T) {
	a := Stock{Alloy: 30, Silica: 10, Parts: 4}
	b := Stock{Alloy: 5, Lumen: 1}
	orig := a
	if got, want := a.Plus(b), (Stock{Alloy: 35, Silica: 10, Parts: 4, Lumen: 1}); got != want {
		t.Errorf("Plus = %v, want %v", got, want)
	}
	if got, want := a.Minus(b), (Stock{Alloy: 25, Silica: 10, Parts: 4, Lumen: -1}); got != want {
		t.Errorf("Minus = %v, want %v", got, want)
	}
	if got, want := a.Scaled(0.5), (Stock{Alloy: 15, Silica: 5, Parts: 2}); got != want {
		t.Errorf("Scaled = %v, want %v", got, want)
	}
	if got, want := a.Plus(b).Only(Alloy, Silica, Parts), (Stock{Alloy: 35, Silica: 10, Parts: 4}); got != want {
		t.Errorf("Only = %v, want %v", got, want)
	}
	if got := a.Only(GoodNone, Good(99)); !got.IsZero() {
		t.Errorf("Only with invalid goods = %v", got)
	}
	if a != orig {
		t.Errorf("pure operations modified the receiver: %v", a)
	}
	if got := a.Total(); got != 44 {
		t.Errorf("Total = %v, want 44", got)
	}
	if a.Get(Alloy) != 30 || a.Get(GoodNone) != 0 || a.Get(Good(50)) != 0 {
		t.Error("Get is wrong")
	}
	if a.IsZero() || !(Stock{}).IsZero() {
		t.Error("IsZero is wrong")
	}
	// Demolition refund: 50 % of Alloy, Silica and Parts only.
	cost := BuildingDefOf(LanceTower).Cost // A90 P30 L10
	refund := cost.Only(Alloy, Silica, Parts).Scaled(DemolishRefundFrac)
	if want := (Stock{Alloy: 45, Parts: 15}); refund != want {
		t.Errorf("demolition refund = %v, want %v", refund, want)
	}
	// Value at base prices.
	if got, want := (Stock{Ore: 10, Lumen: 2, Parts: 1}).Value(), 10*1.0+2*12.0+9.0; math.Abs(got-want) > 1e-12 {
		t.Errorf("Value = %v, want %v", got, want)
	}
}

func TestStockHasAndShortage(t *testing.T) {
	have := Stock{Alloy: 35, Silica: 10}
	tests := []struct {
		name    string
		cost    Stock
		ok      bool
		missing Stock
		errText string
	}{
		{"nothing", Stock{}, true, Stock{}, ""},
		{"exact", Stock{Alloy: 35, Silica: 10}, true, Stock{}, ""},
		{"less", Stock{Alloy: 20}, true, Stock{}, ""},
		{"alloy short", Stock{Alloy: 36}, false, Stock{Alloy: 1}, "Not enough Alloy (need 36)"},
		{"parts short", Stock{Alloy: 10, Parts: 5}, false, Stock{Parts: 5}, "Not enough Parts (need 5)"},
		{"first lacking good in enum order", Stock{Silica: 12.5, Alloy: 70}, false, Stock{Silica: 2.5, Alloy: 35}, "Not enough Silica (need 12.5)"},
		{"float dust is tolerated", Stock{Alloy: 35 + 1e-12}, true, Stock{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := have.Has(tc.cost); got != tc.ok {
				t.Errorf("Has = %v, want %v", got, tc.ok)
			}
			if got := have.Missing(tc.cost); got != tc.missing {
				t.Errorf("Missing = %v, want %v", got, tc.missing)
			}
			err := have.Shortage(tc.cost)
			if tc.ok {
				if err != nil {
					t.Errorf("Shortage = %v, want nil", err)
				}
				return
			}
			var se *ShortageError
			if !errors.As(err, &se) {
				t.Fatalf("Shortage returned %T, want *ShortageError", err)
			}
			if err.Error() != tc.errText {
				t.Errorf("error text = %q, want %q", err.Error(), tc.errText)
			}
		})
	}
	if got := (&ScripError{Need: 120}).Error(); got != "Not enough Scrip (need 120)" {
		t.Errorf("ScripError = %q", got)
	}
}

func TestStockStringAndFormatQty(t *testing.T) {
	if got := (Stock{}).String(); got != "nothing" {
		t.Errorf("empty String = %q", got)
	}
	if got := (Stock{Silica: 10, Alloy: 30}).String(); got != "Silica 10, Alloy 30" {
		t.Errorf("String = %q", got)
	}
	if got := BuildingDefOf(LumenCore).Cost.String(); got != "Lumen 20, Alloy 150, Parts 60" {
		t.Errorf("Lumen Core cost = %q", got)
	}
	for _, tc := range []struct {
		v    float64
		want string
	}{
		{0, "0"}, {35, "35"}, {0.5, "0.5"}, {12.34, "12.3"}, {12.96, "13"}, {1e6, "1000000"}, {-2.25, "-2.3"}, {0.04, "0"},
	} {
		if got := FormatQty(tc.v); got != tc.want {
			t.Errorf("FormatQty(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestColonyStorage(t *testing.T) {
	c := NewColony(120)
	if c.FoundedTick != 120 || c.Morale != StartMorale || c.StorageCap != 400 || c.Housing != 12 ||
		c.LifeSupport != 12 || c.PowerSupply != 20 {
		t.Fatalf("NewColony = %+v", c)
	}
	c.Stock = Stock{Alloy: 390, Food: 400, Ice: 10}

	if got := c.Free(Alloy); got != 10 {
		t.Errorf("Free(Alloy) = %v, want 10", got)
	}
	if got := c.Free(Food); got != 0 {
		t.Errorf("Free(Food) = %v, want 0", got)
	}
	if got := c.Free(GoodNone); got != 0 {
		t.Errorf("Free(GoodNone) = %v", got)
	}
	// Over capacity never yields negative free space.
	c.Stock[Fuel] = 500
	if got := c.Free(Fuel); got != 0 {
		t.Errorf("Free above capacity = %v, want 0", got)
	}
	c.Stock[Fuel] = 0

	addTests := []struct {
		g         Good
		amt, want float64
		after     float64
	}{
		{Alloy, 4, 4, 394},
		{Alloy, 50, 6, 400},
		{Alloy, 1, 0, 400},
		{Food, 5, 0, 400},
		{Ice, 0, 0, 10},
		{Ice, -5, 0, 10},
		{Ice, math.NaN(), 0, 10},
		{Parts, 25, 25, 25},
	}
	for _, tc := range addTests {
		if got := c.AddClamped(tc.g, tc.amt); got != tc.want {
			t.Errorf("AddClamped(%s, %v) = %v, want %v", tc.g, tc.amt, got, tc.want)
		}
		if c.Stock[tc.g] != tc.after {
			t.Errorf("after AddClamped(%s, %v): stock %v, want %v", tc.g, tc.amt, c.Stock[tc.g], tc.after)
		}
	}
	if got := c.AddClamped(GoodNone, 5); got != 0 {
		t.Errorf("AddClamped(GoodNone) = %v", got)
	}

	takeTests := []struct {
		g         Good
		amt, want float64
		after     float64
	}{
		{Parts, 10, 10, 15},
		{Parts, 40, 15, 0},
		{Parts, 1, 0, 0},
		{Ice, -3, 0, 10},
		{Ice, 0, 0, 10},
		{Ice, 10, 10, 0},
	}
	for _, tc := range takeTests {
		if got := c.Take(tc.g, tc.amt); got != tc.want {
			t.Errorf("Take(%s, %v) = %v, want %v", tc.g, tc.amt, got, tc.want)
		}
		if c.Stock[tc.g] != tc.after {
			t.Errorf("after Take(%s, %v): stock %v, want %v", tc.g, tc.amt, c.Stock[tc.g], tc.after)
		}
	}
	if got := c.Take(Good(77), 5); got != 0 {
		t.Errorf("Take(invalid) = %v", got)
	}

	// AddStockClamped reports what actually fitted.
	c.Stock = Stock{Alloy: 380, Silica: 0}
	added := c.AddStockClamped(Stock{Alloy: 45, Silica: 15, Parts: 0})
	if want := (Stock{Alloy: 20, Silica: 15}); added != want {
		t.Errorf("AddStockClamped added %v, want %v", added, want)
	}
	if want := (Stock{Alloy: 400, Silica: 15}); c.Stock != want {
		t.Errorf("stock after AddStockClamped = %v, want %v", c.Stock, want)
	}

	// Pay is all-or-nothing.
	c.Stock = Stock{Alloy: 60, Parts: 5}
	if c.Pay(Stock{Alloy: 60, Parts: 10}) {
		t.Error("Pay succeeded without enough Parts")
	}
	if want := (Stock{Alloy: 60, Parts: 5}); c.Stock != want {
		t.Errorf("failed Pay changed the stock to %v", c.Stock)
	}
	if !c.CanAfford(Stock{Alloy: 60, Parts: 5}) || !c.Pay(Stock{Alloy: 60, Parts: 5}) {
		t.Error("Pay failed although affordable")
	}
	if !c.Stock.IsZero() {
		t.Errorf("stock after paying everything = %v", c.Stock)
	}

	// Nil colonies are inert.
	var none *Colony
	if none.Free(Alloy) != 0 || none.AddClamped(Alloy, 5) != 0 || none.Take(Alloy, 5) != 0 ||
		none.CanAfford(Stock{}) || none.Pay(Stock{}) || none.PowerDeficit() || none.IdleWorkers() != 0 ||
		none.AttackedWithin(10, 5) || none.SponsorReady(10) || none.NetRate(Alloy) != 0 ||
		!none.AddStockClamped(Stock{Alloy: 1}).IsZero() {
		t.Error("nil colony helpers must be inert")
	}
	none.MarkAttacked(5)
	none.MarkSponsored(5)
	none.NoteProduced(Alloy, 1)
	none.NoteConsumed(Alloy, 1)
	none.RollRates(1)
}

func TestColonyTimers(t *testing.T) {
	c := NewColony(0)
	if c.AttackedWithin(0, MoraleAttackWindowTicks) || c.AttackedWithin(5000, MoraleAttackWindowTicks) {
		t.Error("a new colony must not count as attacked")
	}
	c.MarkAttacked(1000)
	for _, tc := range []struct {
		now, window int
		want        bool
	}{
		{1000, MoraleAttackWindowTicks, true},
		{2199, MoraleAttackWindowTicks, true},
		{2200, MoraleAttackWindowTicks, false},
		{1599, SelfRepairQuietTicks, true},
		{1600, SelfRepairQuietTicks, false},
	} {
		if got := c.AttackedWithin(tc.now, tc.window); got != tc.want {
			t.Errorf("AttackedWithin(%d, %d) = %v, want %v", tc.now, tc.window, got, tc.want)
		}
	}
	// An attack at tick 0 is still an attack.
	c2 := NewColony(0)
	c2.MarkAttacked(0)
	if !c2.AttackedWithin(3, 10) {
		t.Error("attack at tick 0 was lost")
	}

	if !c.SponsorReady(0) || !c.SponsorReady(99999) {
		t.Error("a colony that never sponsored must be ready")
	}
	c.MarkSponsored(700)
	if c.SponsorReady(700) || c.SponsorReady(1299) {
		t.Error("sponsorship must be on cooldown for one cycle")
	}
	if !c.SponsorReady(1300) {
		t.Error("sponsorship must be ready after one cycle")
	}

	c.PowerSupply, c.PowerDemand = 20, 20
	if c.PowerDeficit() {
		t.Error("supply == demand is not a deficit")
	}
	c.PowerDemand = 21
	if !c.PowerDeficit() {
		t.Error("demand > supply is a deficit")
	}
	c.Workers, c.WorkersUsed = 14, 9
	if c.IdleWorkers() != 5 {
		t.Errorf("IdleWorkers = %d", c.IdleWorkers())
	}
	c.WorkersUsed = 20
	if c.IdleWorkers() != 0 {
		t.Errorf("IdleWorkers = %d, want 0", c.IdleWorkers())
	}
}

func TestColonyRates(t *testing.T) {
	c := NewColony(0)
	// A steady 0.6 Alloy produced and 0.5 Ore consumed per second (36 and
	// 30 per cycle) must converge on those per-cycle rates.
	for i := 0; i < 200; i++ {
		c.NoteProduced(Alloy, 0.6)
		c.NoteConsumed(Ore, 0.2)
		c.NoteConsumed(Ore, 0.3)
		c.NoteProduced(GoodNone, 5) // ignored
		c.NoteProduced(Food, -1)    // ignored
		c.RollRates(SlowDT)
		if !c.ProducedAcc.IsZero() || !c.ConsumedAcc.IsZero() {
			t.Fatal("RollRates must clear the accumulators")
		}
	}
	if got := c.Produced[Alloy]; math.Abs(got-36) > 1e-6 {
		t.Errorf("Produced[Alloy] = %v, want 36", got)
	}
	if got := c.Consumed[Ore]; math.Abs(got-30) > 1e-6 {
		t.Errorf("Consumed[Ore] = %v, want 30", got)
	}
	if got := c.NetRate(Alloy); math.Abs(got-36) > 1e-6 {
		t.Errorf("NetRate(Alloy) = %v, want 36", got)
	}
	if got := c.NetRate(Ore); math.Abs(got+30) > 1e-6 {
		t.Errorf("NetRate(Ore) = %v, want -30", got)
	}
	if c.Produced[Food] != 0 || c.NetRate(GoodNone) != 0 {
		t.Error("ignored notes leaked into the rates")
	}
	// The first step moves the rate by dt/RateSmoothingSeconds of the way.
	c2 := NewColony(0)
	c2.NoteProduced(Food, 1)
	c2.RollRates(1)
	if got, want := c2.Produced[Food], 60.0/RateSmoothingSeconds; math.Abs(got-want) > 1e-12 {
		t.Errorf("first smoothed rate = %v, want %v", got, want)
	}
	// When production stops the rate decays toward zero.
	for i := 0; i < 200; i++ {
		c.RollRates(SlowDT)
	}
	if got := c.Produced[Alloy]; got > 1e-6 {
		t.Errorf("rate did not decay: %v", got)
	}
	// Non-positive dt is ignored.
	c.NoteProduced(Ice, 3)
	c.RollRates(0)
	if c.ProducedAcc[Ice] != 3 {
		t.Error("RollRates(0) must do nothing")
	}
}
