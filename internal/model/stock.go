package model

import (
	"math"
	"strconv"
	"strings"
)

// StockEpsilon is the tolerance used when comparing stock amounts, so that
// accumulated floating-point error never blocks an action the player can
// plainly afford.
const StockEpsilon = 1e-9

// Stock is an amount of every good, indexed by Good:
//
//	cost := model.Stock{model.Alloy: 30, model.Silica: 10}
//	alloy := colony.Stock[model.Alloy]
//
// Stock is a plain array value: assignment copies it. The mutating methods
// (Add, Sub, Scale, Clear, ClampMin0) have pointer receivers and return
// nothing; the pure variants (Plus, Minus, Scaled, Only) return a new Stock.
type Stock [NumGoods]float64

// Add adds o to s in place, good by good.
func (s *Stock) Add(o Stock) {
	for g := range s {
		s[g] += o[g]
	}
}

// Sub subtracts o from s in place, good by good. Amounts may go negative;
// check Has first (or call ClampMin0 afterwards) when that is not wanted.
func (s *Stock) Sub(o Stock) {
	for g := range s {
		s[g] -= o[g]
	}
}

// Scale multiplies every amount of s by k in place.
func (s *Stock) Scale(k float64) {
	for g := range s {
		s[g] *= k
	}
}

// Clear sets every amount of s to zero.
func (s *Stock) Clear() { *s = Stock{} }

// ClampMin0 raises every negative amount of s to zero in place.
func (s *Stock) ClampMin0() {
	for g := range s {
		if s[g] < 0 {
			s[g] = 0
		}
	}
}

// Plus returns s + o without modifying either.
func (s Stock) Plus(o Stock) Stock {
	s.Add(o)
	return s
}

// Minus returns s - o without modifying either.
func (s Stock) Minus(o Stock) Stock {
	s.Sub(o)
	return s
}

// Scaled returns s with every amount multiplied by k, without modifying s.
func (s Stock) Scaled(k float64) Stock {
	s.Scale(k)
	return s
}

// Only returns a copy of s that keeps the listed goods and zeroes all others,
// for example cost.Only(Alloy, Silica, Parts) for a demolition refund.
func (s Stock) Only(goods ...Good) Stock {
	var out Stock
	for _, g := range goods {
		if g.Valid() {
			out[g] = s[g]
		}
	}
	return out
}

// Get returns the amount of good g, or 0 when g is not a valid good.
func (s Stock) Get(g Good) float64 {
	if !g.Valid() {
		return 0
	}
	return s[g]
}

// Has reports whether s covers cost: for every good, s holds at least the
// cost amount (within StockEpsilon).
func (s Stock) Has(cost Stock) bool {
	for g := range s {
		if s[g]+StockEpsilon < cost[g] {
			return false
		}
	}
	return true
}

// Missing returns, per good, how much of cost s cannot cover (0 where s is
// sufficient).
func (s Stock) Missing(cost Stock) Stock {
	var out Stock
	for g := range s {
		if d := cost[g] - s[g]; d > StockEpsilon {
			out[g] = d
		}
	}
	return out
}

// Shortage returns nil when s covers cost, and otherwise a *ShortageError
// naming the first good (in enum order) that is lacking and the amount the
// cost requires, e.g. "Not enough Alloy (need 35)".
func (s Stock) Shortage(cost Stock) error {
	for g := range s {
		if s[g]+StockEpsilon < cost[g] {
			return &ShortageError{Good: Good(g), Need: cost[g]}
		}
	}
	return nil
}

// Total returns the sum of all amounts.
func (s Stock) Total() float64 {
	t := 0.0
	for _, v := range s {
		t += v
	}
	return t
}

// IsZero reports whether every amount is exactly zero.
func (s Stock) IsZero() bool { return s == Stock{} }

// Value returns the base market value of the stock in Scrip: the sum of
// amount x GoodInfo.BaseValue.
func (s Stock) Value() float64 {
	t := 0.0
	for g, v := range s {
		t += v * goodInfos[g].BaseValue
	}
	return t
}

// String lists the non-zero amounts in enum order, e.g.
// "Alloy 30, Silica 10". An all-zero stock formats as "nothing".
func (s Stock) String() string {
	var b strings.Builder
	for g, v := range s {
		if v == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteString(goodNames[g])
		b.WriteByte(' ')
		b.WriteString(FormatQty(v))
	}
	if b.Len() == 0 {
		return "nothing"
	}
	return b.String()
}

// FormatQty formats an amount for display: whole numbers without decimals
// ("35"), everything else rounded to one decimal ("0.5", "12.3").
func FormatQty(v float64) string {
	r := math.Round(v*10) / 10
	if r == math.Trunc(r) {
		return strconv.FormatFloat(r, 'f', 0, 64)
	}
	return strconv.FormatFloat(r, 'f', 1, 64)
}
