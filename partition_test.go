package meso

import "testing"

func TestPartition_WellFormed(t *testing.T) {
	valid := []struct {
		name string
		p    Partition
		n    int
	}{
		{"singletons", Partition{0, 1, 2}, 3},
		{"all one community", Partition{0, 0, 0}, 3},
		{"labels within range", Partition{2, 0, 2}, 3},
		{"empty graph", Partition{}, 0},
	}
	for _, tc := range valid {
		if err := tc.p.wellFormed(tc.n); err != nil {
			t.Errorf("%s: wellFormed(%d) = %v, want nil", tc.name, tc.n, err)
		}
	}

	invalid := []struct {
		name string
		p    Partition
		n    int
	}{
		{"too short", Partition{0, 1}, 3},
		{"too long", Partition{0, 1, 2, 0}, 3},
		{"label out of range high", Partition{0, 3, 1}, 3},
		{"label negative", Partition{0, -1, 1}, 3},
	}
	for _, tc := range invalid {
		if err := tc.p.wellFormed(tc.n); err == nil {
			t.Errorf("%s: wellFormed(%d) = nil, want error", tc.name, tc.n)
		}
	}
}
