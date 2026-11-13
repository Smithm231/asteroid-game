package model

import "math/bits"

// RNG is the deterministic pseudo-random number generator of the simulation.
//
// It is a SplitMix64 generator: one 64-bit word of state, a period of 2^64
// and well-mixed 64-bit outputs. The state is exported so that the generator
// serialises as plain JSON and a loaded game continues the exact same random
// sequence. The zero value is a valid generator (the stream of seed state 0).
//
// RNG is meant to be embedded by value (World.RNG) and used through its
// pointer methods: w.RNG.Float64(). Copying an RNG copies its position in the
// stream, so two copies produce the same numbers; use Fork or Derive to get
// independent streams instead.
//
// All simulation randomness must come from World.RNG or from a generator
// derived from it; never use math/rand or the clock.
type RNG struct {
	// State is the complete generator state.
	State uint64
}

const (
	// rngGamma is the SplitMix64 increment (the 64-bit golden ratio).
	rngGamma = 0x9E3779B97F4A7C15
	// rngSeedSalt decorrelates the user-visible seed from the raw state.
	rngSeedSalt = 0xD1B54A32D192ED03
)

// mix64 is the SplitMix64 output finaliser, a bijection on uint64.
func mix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// hashString returns the 64-bit FNV-1a hash of s.
func hashString(s string) uint64 {
	h := uint64(0xCBF29CE484222325)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 0x100000001B3
	}
	return h
}

// NewRNG returns a generator seeded with seed. Different seeds give
// unrelated streams; the same seed always gives the same stream.
func NewRNG(seed uint64) RNG {
	return RNG{State: mix64(seed ^ rngSeedSalt)}
}

// Uint64 returns the next 64 uniformly distributed random bits.
func (r *RNG) Uint64() uint64 {
	r.State += rngGamma
	return mix64(r.State)
}

// Uint32 returns the next 32 uniformly distributed random bits.
func (r *RNG) Uint32() uint32 { return uint32(r.Uint64() >> 32) }

// Float64 returns a uniformly distributed number in [0, 1). It has 53 random
// bits and never returns 1.
func (r *RNG) Float64() float64 {
	return float64(r.Uint64()>>11) * (1.0 / (1 << 53))
}

// Uint64n returns a uniformly distributed number in [0, n) without modulo
// bias. n == 0 yields 0 without consuming randomness.
func (r *RNG) Uint64n(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	hi, lo := bits.Mul64(r.Uint64(), n)
	if lo < n {
		thresh := -n % n
		for lo < thresh {
			hi, lo = bits.Mul64(r.Uint64(), n)
		}
	}
	return hi
}

// Intn returns a uniformly distributed integer in [0, n). A non-positive n
// yields 0 without consuming randomness (it never panics).
func (r *RNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.Uint64n(uint64(n)))
}

// IntRange returns a uniformly distributed integer in [lo, hi], both ends
// inclusive. If hi < lo the bounds are swapped.
func (r *RNG) IntRange(lo, hi int) int {
	if hi < lo {
		lo, hi = hi, lo
	}
	return lo + int(r.Uint64n(uint64(hi-lo)+1))
}

// Range returns a uniformly distributed number in [lo, hi). If hi < lo the
// bounds are swapped; if they are equal the result is lo.
func (r *RNG) Range(lo, hi float64) float64 {
	if hi < lo {
		lo, hi = hi, lo
	}
	return lo + (hi-lo)*r.Float64()
}

// Chance returns true with probability p. p <= 0 never succeeds and p >= 1
// always succeeds; exactly one random number is consumed either way so that
// the stream position does not depend on p.
func (r *RNG) Chance(p float64) bool {
	return r.Float64() < p
}

// Sign returns -1 or +1 with equal probability.
func (r *RNG) Sign() float64 {
	if r.Uint64()&1 == 0 {
		return -1
	}
	return 1
}

// Shuffle randomly permutes n elements using the Fisher-Yates algorithm,
// calling swap(i, j) to exchange elements i and j. It does nothing for
// n < 2 or a nil swap.
func (r *RNG) Shuffle(n int, swap func(i, j int)) {
	if swap == nil {
		return
	}
	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		swap(i, j)
	}
}

// Perm returns a random permutation of the integers [0, n). A non-positive n
// yields nil.
func (r *RNG) Perm(n int) []int {
	if n <= 0 {
		return nil
	}
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	r.Shuffle(n, func(i, j int) { p[i], p[j] = p[j], p[i] })
	return p
}

// Weighted returns a random index into weights, chosen with probability
// proportional to its weight. Negative and NaN weights count as 0. It returns
// -1 when weights is empty or sums to 0. Exactly one random number is
// consumed when a choice is possible.
func (r *RNG) Weighted(weights []float64) int {
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if !(total > 0) {
		return -1
	}
	x := r.Float64() * total
	last := -1
	for i, w := range weights {
		if !(w > 0) {
			continue
		}
		last = i
		if x < w {
			return i
		}
		x -= w
	}
	return last
}

// Fork returns a new, independent generator and advances r by one step. Use
// it to hand a private stream to a sub-task while keeping r's own sequence
// independent of how much randomness the sub-task consumes.
func (r *RNG) Fork() RNG {
	return RNG{State: mix64(r.Uint64() ^ rngSeedSalt)}
}

// Derive returns an independent generator identified by label, computed from
// r's current state WITHOUT advancing r. The same state and label always give
// the same stream; different labels give unrelated streams. Typical use:
// rng.Derive("deposits") or rng.Derive("rock:7") during world generation.
func (r *RNG) Derive(label string) RNG {
	return RNG{State: mix64(mix64(r.State^rngSeedSalt) + hashString(label))}
}

// ShuffleSlice randomly permutes s in place using r.
func ShuffleSlice[T any](r *RNG, s []T) {
	r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

// Pick returns a uniformly chosen element of s and true, or the zero value
// and false when s is empty (in which case no randomness is consumed).
func Pick[T any](r *RNG, s []T) (T, bool) {
	if len(s) == 0 {
		var zero T
		return zero, false
	}
	return s[r.Intn(len(s))], true
}
