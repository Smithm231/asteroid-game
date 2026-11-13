package model

import (
	"encoding/json"
	"math"
	"slices"
	"testing"
)

func TestRNGReferenceVector(t *testing.T) {
	// The zero RNG is SplitMix64 with state 0; these are the published
	// reference outputs of that generator. Saves depend on the algorithm
	// never changing.
	var r RNG
	want := []uint64{
		0xE220A8397B1DCDAF,
		0x6E789E6AA1B965F4,
		0x06C45D188009454F,
		0xF88BB8A8724C81EC,
	}
	for i, w := range want {
		if got := r.Uint64(); got != w {
			t.Fatalf("output %d = %#016x, want %#016x", i, got, w)
		}
	}
}

func TestRNGSeedStability(t *testing.T) {
	// Pin the seeding function too: the same seed must give the same game
	// forever.
	r := NewRNG(1)
	a, b := r.Uint64(), r.Uint64()
	r2 := NewRNG(1)
	if r2.Uint64() != a || r2.Uint64() != b {
		t.Fatal("NewRNG(1) is not reproducible")
	}
	if want := mix64(1 ^ rngSeedSalt); NewRNG(1).State != want {
		t.Fatalf("NewRNG(1).State = %#x, want %#x", NewRNG(1).State, want)
	}
	const wantFirst, wantSecond = uint64(0x910A2DEC89025CC1), uint64(0xBEEB8DA1658EEC67)
	// The two constants above are the first outputs of SplitMix64 seeded
	// with state 1 and are unrelated to NewRNG; they check mix64 on a
	// second input.
	s := RNG{State: 1}
	if got := s.Uint64(); got != wantFirst {
		t.Fatalf("state 1 first output = %#016x, want %#016x", got, wantFirst)
	}
	if got := s.Uint64(); got != wantSecond {
		t.Fatalf("state 1 second output = %#016x, want %#016x", got, wantSecond)
	}
}

func TestRNGDeterminism(t *testing.T) {
	a, b := NewRNG(12345), NewRNG(12345)
	for i := 0; i < 1000; i++ {
		if x, y := a.Uint64(), b.Uint64(); x != y {
			t.Fatalf("streams diverge at step %d: %x vs %x", i, x, y)
		}
	}
	// Mixed method calls stay in lockstep too.
	for i := 0; i < 200; i++ {
		if a.Intn(17) != b.Intn(17) || a.Float64() != b.Float64() ||
			a.IntRange(-3, 9) != b.IntRange(-3, 9) || a.Chance(0.3) != b.Chance(0.3) ||
			a.Range(2, 5) != b.Range(2, 5) || a.Sign() != b.Sign() {
			t.Fatalf("mixed streams diverge at step %d", i)
		}
	}
	if a != b {
		t.Fatal("states differ after identical call sequences")
	}

	// Different seeds give different streams, including adjacent seeds.
	seen := map[uint64]uint64{}
	for seed := uint64(0); seed < 2000; seed++ {
		r := NewRNG(seed)
		v := r.Uint64()
		if other, dup := seen[v]; dup {
			t.Fatalf("seeds %d and %d share their first output", other, seed)
		}
		seen[v] = seed
	}
	// A copy continues the same stream (value semantics).
	c := a
	if a.Uint64() != c.Uint64() {
		t.Fatal("a copied RNG must produce the same next value")
	}
}

func TestRNGFloat64(t *testing.T) {
	r := NewRNG(99)
	const n = 200000
	sum, sumSq := 0.0, 0.0
	var buckets [10]int
	for i := 0; i < n; i++ {
		x := r.Float64()
		if x < 0 || x >= 1 {
			t.Fatalf("Float64 = %v outside [0,1)", x)
		}
		sum += x
		sumSq += x * x
		buckets[int(x*10)]++
	}
	mean := sum / n
	variance := sumSq/n - mean*mean
	if math.Abs(mean-0.5) > 0.005 {
		t.Errorf("mean = %v, want about 0.5", mean)
	}
	if math.Abs(variance-1.0/12) > 0.002 {
		t.Errorf("variance = %v, want about %v", variance, 1.0/12)
	}
	for i, c := range buckets {
		if math.Abs(float64(c)-n/10) > 0.04*n/10 {
			t.Errorf("bucket %d has %d samples, want about %d", i, c, n/10)
		}
	}
}

func TestRNGIntn(t *testing.T) {
	r := NewRNG(5)
	before := r
	if r.Intn(0) != 0 || r.Intn(-7) != 0 {
		t.Error("Intn of a non-positive bound must be 0")
	}
	if r != before {
		t.Error("Intn of a non-positive bound must not consume randomness")
	}
	for i := 0; i < 100; i++ {
		if got := r.Intn(1); got != 0 {
			t.Fatalf("Intn(1) = %d", got)
		}
	}
	for _, n := range []int{2, 3, 7, 10, 100, 1000} {
		counts := make([]int, n)
		const per = 4000
		for i := 0; i < n*per; i++ {
			v := r.Intn(n)
			if v < 0 || v >= n {
				t.Fatalf("Intn(%d) = %d out of range", n, v)
			}
			counts[v]++
		}
		// Chi-square against the uniform distribution; the bound is far in
		// the tail (mean n-1, sd sqrt(2(n-1))), so a correct generator
		// passes with a fixed seed and a biased one fails clearly.
		chi := 0.0
		for _, c := range counts {
			d := float64(c) - per
			chi += d * d / per
		}
		limit := float64(n-1) + 6*math.Sqrt(2*float64(n-1)) + 6
		if chi > limit {
			t.Errorf("Intn(%d): chi-square %v exceeds %v", n, chi, limit)
		}
	}
	// Large bounds work and stay in range.
	for i := 0; i < 1000; i++ {
		if v := r.Uint64n(1<<63 + 12345); v >= 1<<63+12345 {
			t.Fatalf("Uint64n out of range: %d", v)
		}
	}
	if r.Uint64n(0) != 0 {
		t.Error("Uint64n(0) must be 0")
	}
}

func TestRNGIntRangeAndRange(t *testing.T) {
	r := NewRNG(8)
	seen := map[int]int{}
	for i := 0; i < 20000; i++ {
		v := r.IntRange(8, 14)
		if v < 8 || v > 14 {
			t.Fatalf("IntRange(8,14) = %d", v)
		}
		seen[v]++
	}
	for v := 8; v <= 14; v++ {
		if seen[v] < 2400 || seen[v] > 3300 {
			t.Errorf("IntRange(8,14) produced %d %d times, want about 2857", v, seen[v])
		}
	}
	for i := 0; i < 100; i++ {
		if v := r.IntRange(5, 5); v != 5 {
			t.Fatalf("IntRange(5,5) = %d", v)
		}
		if v := r.IntRange(3, -2); v < -2 || v > 3 {
			t.Fatalf("IntRange(3,-2) = %d (bounds must be swapped)", v)
		}
		if v := r.Range(40, 80); v < 40 || v >= 80 {
			t.Fatalf("Range(40,80) = %v", v)
		}
		if v := r.Range(2, -2); v < -2 || v >= 2 {
			t.Fatalf("Range(2,-2) = %v (bounds must be swapped)", v)
		}
		if v := r.Range(3.5, 3.5); v != 3.5 {
			t.Fatalf("Range(3.5,3.5) = %v", v)
		}
	}
	sum := 0.0
	const n = 100000
	for i := 0; i < n; i++ {
		sum += r.Range(0.9, 1.1)
	}
	if mean := sum / n; math.Abs(mean-1.0) > 0.001 {
		t.Errorf("Range(0.9,1.1) mean = %v, want about 1", mean)
	}
}

func TestRNGChanceAndSign(t *testing.T) {
	r := NewRNG(21)
	for i := 0; i < 1000; i++ {
		if r.Chance(0) || r.Chance(-1) {
			t.Fatal("Chance(<=0) must never succeed")
		}
		if !r.Chance(1) || !r.Chance(2) {
			t.Fatal("Chance(>=1) must always succeed")
		}
	}
	// Chance consumes exactly one number whatever p is.
	a, b := NewRNG(3), NewRNG(3)
	a.Chance(0)
	b.Chance(1)
	if a != b {
		t.Error("Chance must consume the same amount of randomness for every p")
	}
	const n = 100000
	hits, plus := 0, 0
	for i := 0; i < n; i++ {
		if r.Chance(0.3) {
			hits++
		}
		switch s := r.Sign(); s {
		case 1:
			plus++
		case -1:
		default:
			t.Fatalf("Sign = %v", s)
		}
	}
	if p := float64(hits) / n; math.Abs(p-0.3) > 0.01 {
		t.Errorf("Chance(0.3) hit rate = %v", p)
	}
	if p := float64(plus) / n; math.Abs(p-0.5) > 0.01 {
		t.Errorf("Sign positive rate = %v", p)
	}
}

func TestRNGShufflePermPick(t *testing.T) {
	r := NewRNG(77)
	if r.Perm(0) != nil || r.Perm(-3) != nil {
		t.Error("Perm of a non-positive n must be nil")
	}
	r.Shuffle(5, nil) // must not panic
	for n := 1; n <= 30; n++ {
		p := r.Perm(n)
		sorted := slices.Clone(p)
		slices.Sort(sorted)
		for i, v := range sorted {
			if v != i {
				t.Fatalf("Perm(%d) = %v is not a permutation", n, p)
			}
		}
	}
	// Same seed, same shuffle.
	a, b := NewRNG(4), NewRNG(4)
	sa := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	sb := slices.Clone(sa)
	ShuffleSlice(&a, sa)
	ShuffleSlice(&b, sb)
	if !slices.Equal(sa, sb) {
		t.Errorf("shuffles differ: %v vs %v", sa, sb)
	}
	// Every position is reachable for every element (uniformity smoke test).
	var pos [4][4]int
	for i := 0; i < 8000; i++ {
		s := []int{0, 1, 2, 3}
		ShuffleSlice(&r, s)
		for at, v := range s {
			pos[v][at]++
		}
	}
	for v := range pos {
		for at, c := range pos[v] {
			if c < 1800 || c > 2200 {
				t.Errorf("element %d landed at %d %d times, want about 2000", v, at, c)
			}
		}
	}

	if _, ok := Pick(&r, []int(nil)); ok {
		t.Error("Pick from an empty slice must report false")
	}
	before := r
	Pick(&r, []int{})
	if r != before {
		t.Error("Pick from an empty slice must not consume randomness")
	}
	counts := map[string]int{}
	for i := 0; i < 3000; i++ {
		v, ok := Pick(&r, []string{"x", "y", "z"})
		if !ok {
			t.Fatal("Pick reported false for a non-empty slice")
		}
		counts[v]++
	}
	for _, k := range []string{"x", "y", "z"} {
		if counts[k] < 850 || counts[k] > 1150 {
			t.Errorf("Pick chose %q %d times, want about 1000", k, counts[k])
		}
	}
}

func TestRNGWeighted(t *testing.T) {
	r := NewRNG(31)
	if got := r.Weighted(nil); got != -1 {
		t.Errorf("Weighted(nil) = %d, want -1", got)
	}
	if got := r.Weighted([]float64{0, 0, -1, math.NaN()}); got != -1 {
		t.Errorf("Weighted(all zero) = %d, want -1", got)
	}
	weights := []float64{1, 0, 3, -5, 6}
	counts := make([]int, len(weights))
	const n = 100000
	for i := 0; i < n; i++ {
		idx := r.Weighted(weights)
		if idx < 0 || idx >= len(weights) {
			t.Fatalf("Weighted = %d out of range", idx)
		}
		counts[idx]++
	}
	if counts[1] != 0 || counts[3] != 0 {
		t.Errorf("zero or negative weights were chosen: %v", counts)
	}
	for i, want := range []float64{0.1, 0, 0.3, 0, 0.6} {
		if got := float64(counts[i]) / n; math.Abs(got-want) > 0.01 {
			t.Errorf("weight %d chosen with frequency %v, want about %v", i, got, want)
		}
	}
	if got := r.Weighted([]float64{0, 0, 2.5}); got != 2 {
		t.Errorf("single positive weight: got %d, want 2", got)
	}
}

func TestRNGForkAndDerive(t *testing.T) {
	parent := NewRNG(1000)
	reference := parent

	// Derive does not advance the parent and is a pure function of state
	// and label.
	d1 := parent.Derive("deposits")
	d2 := parent.Derive("deposits")
	d3 := parent.Derive("rocks")
	if parent != reference {
		t.Fatal("Derive advanced the parent")
	}
	if d1 != d2 {
		t.Fatal("Derive with the same label gave different generators")
	}
	if d1 == d3 || d1 == parent || d3 == parent {
		t.Fatal("derived generators must differ from each other and from the parent")
	}
	// Derived streams differ from the start and stay different.
	same := 0
	x, y, p := d1, d3, parent
	for i := 0; i < 1000; i++ {
		a, b, c := x.Uint64(), y.Uint64(), p.Uint64()
		if a == b || a == c || b == c {
			same++
		}
	}
	if same != 0 {
		t.Errorf("derived streams collided %d times in 1000 draws", same)
	}
	// Deriving from a different state gives a different stream.
	parent.Uint64()
	if parent.Derive("deposits") == d1 {
		t.Error("Derive must depend on the parent's state")
	}

	// Fork advances the parent by exactly one draw.
	a, b := NewRNG(55), NewRNG(55)
	child := a.Fork()
	b.Uint64()
	if a != b {
		t.Fatal("Fork must advance the parent by exactly one step")
	}
	// How much the child consumes does not disturb the parent.
	for i := 0; i < 500; i++ {
		child.Uint64()
	}
	if a.Uint64() != b.Uint64() {
		t.Fatal("parent stream changed after using the fork")
	}
	// Two successive forks are different streams.
	c := NewRNG(55)
	f1, f2 := c.Fork(), c.Fork()
	if f1 == f2 {
		t.Fatal("successive forks are identical")
	}
	if f1.Uint64() == f2.Uint64() {
		t.Error("successive forks start with the same value")
	}
}

func TestRNGJSONRoundTrip(t *testing.T) {
	r := NewRNG(0xFFFFFFFFFFFFFFF1)
	for i := 0; i < 37; i++ {
		r.Uint64()
	}
	// Force a state above 2^63 so that precision loss would be caught.
	for r.State < 1<<63 {
		r.Uint64()
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back RNG
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back != r {
		t.Fatalf("state changed in the round trip: %#x -> %#x (%s)", r.State, back.State, data)
	}
	for i := 0; i < 100; i++ {
		if a, b := r.Uint64(), back.Uint64(); a != b {
			t.Fatalf("restored generator diverges at step %d", i)
		}
	}
	data2, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	// Both generators advanced identically, so they serialise identically.
	want, _ := json.Marshal(r)
	if string(data2) != string(want) {
		t.Errorf("serialised forms differ: %s vs %s", data2, want)
	}

	// The extreme states survive too.
	for _, state := range []uint64{0, 1, math.MaxUint64, 1 << 63, 1<<53 + 1} {
		in := RNG{State: state}
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		var out RNG
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Errorf("state %#x came back as %#x", state, out.State)
		}
	}
}
