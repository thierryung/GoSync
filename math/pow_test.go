package math

import (
	"testing"
)

func TestPow(t *testing.T) {
	// Run basic tests
	cases := []struct {
		i    uint64
		j    int
		want uint64
	}{
		{4, 3, 64},
		{85, 3, 614125},
		{8, 19, 144115188075855872},
	}
	for _, c := range cases {
		got := Pow(c.i, c.j)
		if got != c.want {
			t.Errorf("Pow(%d, %d) == %d, want %d", c.i, c.j, got, c.want)
		}
	}

	// Run n tests, starting from start, for up to power of max
	n, start, max := 10000, 1000, 300
	var want, got uint64
	for i := start; i < start+n; i++ {
		for j := 0; j < max; j++ {
			got = Pow(uint64(i), j)
			want = pow(i, j)
			if got != want {
				t.Errorf("Pow(%d, %d) == %d, want %d", i, j, got, want)
			}
		}
	}
}

func pow(i, j int) uint64 {
	var res uint64 = 1
	for c := 0; c < j; c++ {
		res *= uint64(i)
	}
	return res
}
