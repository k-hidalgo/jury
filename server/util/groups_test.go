package util

import (
	"reflect"
	"testing"
)

func countGroups(groups []int64, numGroups int) []int64 {
	counts := make([]int64, numGroups)
	for _, g := range groups {
		counts[g]++
	}
	return counts
}

func assertCounts(t *testing.T, got []int64, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestAllocateGroupsEqualWeights(t *testing.T) {
	groups := AllocateGroups([]int64{0, 0, 0}, []float64{1, 1, 1}, 7)
	assertCounts(t, countGroups(groups, 3), []int64{3, 2, 2})
}

func TestAllocateGroupsProportional(t *testing.T) {
	groups := AllocateGroups([]int64{0, 0, 0}, []float64{60, 30, 30}, 12)
	assertCounts(t, countGroups(groups, 3), []int64{6, 3, 3})

	groups = AllocateGroups([]int64{0, 0}, []float64{75, 25}, 10)
	assertCounts(t, countGroups(groups, 2), []int64{8, 2})
}

func TestAllocateGroupsRespectsExistingCounts(t *testing.T) {
	// Group 1 already has many judges, so new judges should go to group 0
	groups := AllocateGroups([]int64{0, 5}, []float64{1, 1}, 3)
	assertCounts(t, countGroups(groups, 2), []int64{3, 0})

	// Adding one judge at a time should match adding them all at once
	counts := []int64{0, 0, 0}
	for i := 0; i < 12; i++ {
		g := AllocateGroups(counts, []float64{60, 30, 30}, 1)[0]
		counts[g]++
	}
	assertCounts(t, counts, []int64{6, 3, 3})
}

func TestAllocateGroupsZeroWeights(t *testing.T) {
	// Groups with no projects get no judges
	groups := AllocateGroups([]int64{0, 0, 0}, []float64{10, 0, 10}, 4)
	assertCounts(t, countGroups(groups, 3), []int64{2, 0, 2})

	// All zero falls back to equal weights
	groups = AllocateGroups([]int64{0, 0}, []float64{0, 0}, 4)
	assertCounts(t, countGroups(groups, 2), []int64{2, 2})
}

func assertWeights(t *testing.T, got []float64, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestJudgeGroupWeightsNotProportional(t *testing.T) {
	weights := JudgeGroupWeights(3, false, []int64{60, 30, 30}, []int64{60, 30, 30})
	assertWeights(t, weights, []float64{1, 1, 1})
}

func TestJudgeGroupWeightsUsesProjectCounts(t *testing.T) {
	// Project counts take priority over the configured group sizes
	weights := JudgeGroupWeights(3, true, []int64{50, 0, 20}, []int64{30, 30, 30})
	assertWeights(t, weights, []float64{50, 0, 20})
}

func TestJudgeGroupWeightsFallsBackToGroupSizes(t *testing.T) {
	weights := JudgeGroupWeights(3, true, []int64{0, 0, 0}, []int64{60, 30, 30})
	assertWeights(t, weights, []float64{60, 30, 30})

	weights = JudgeGroupWeights(3, true, nil, []int64{60, 30, 30})
	assertWeights(t, weights, []float64{60, 30, 30})
}

func TestJudgeGroupWeightsMismatchedLengths(t *testing.T) {
	// Group sizes shorter than the number of groups leaves the rest at 0
	weights := JudgeGroupWeights(3, true, nil, []int64{10, 20})
	assertWeights(t, weights, []float64{10, 20, 0})

	// Extra project counts are ignored
	weights = JudgeGroupWeights(2, true, []int64{5, 6, 7}, nil)
	assertWeights(t, weights, []float64{5, 6})
}

func TestAllocateGroupsNoGroups(t *testing.T) {
	groups := AllocateGroups([]int64{}, []float64{}, 3)
	assertCounts(t, groups, []int64{0, 0, 0})
}

func TestAllocateGroupsDoesNotModifyCounts(t *testing.T) {
	counts := []int64{1, 2}
	AllocateGroups(counts, []float64{1, 1}, 5)
	assertCounts(t, counts, []int64{1, 2})
}

func TestTrackGroupCoverageAndProportions(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		projects, counts, want []int64
		n                      int
		proportional           bool
	}{
		{"nine and two projects, two judges", []int64{9, 2}, []int64{0, 0}, []int64{1, 1}, 2, true},
		{"additional judges restore proportion", []int64{9, 2}, []int64{0, 0}, []int64{9, 2}, 11, true},
		{"existing staffed large room", []int64{9, 2}, []int64{1, 0}, []int64{1, 1}, 1, true},
		{"empty room receives none", []int64{0, 11}, []int64{0, 0}, []int64{0, 4}, 4, true},
		{"fewer judges than rooms", []int64{2, 9, 1}, []int64{0, 0, 0}, []int64{1, 1, 0}, 2, true},
		{"equal mode covers only eligible rooms", []int64{9, 0, 2}, []int64{0, 0, 0}, []int64{2, 0, 2}, 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			weights := TrackJudgeGroupWeights(int64(len(tc.counts)), tc.proportional, tc.projects, nil)
			groups := AllocateTrackGroups(tc.counts, weights, tc.n)
			got := append([]int64(nil), tc.counts...)
			for _, group := range groups {
				got[group]++
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTrackGroupsSequentialMatchesBatch(t *testing.T) {
	weights := TrackJudgeGroupWeights(2, true, []int64{9, 2}, nil)
	batch := AllocateTrackGroups([]int64{0, 0}, weights, 11)
	counts := []int64{0, 0}
	for i, want := range batch {
		got := AllocateTrackGroups(counts, weights, 1)[0]
		if got != want {
			t.Fatalf("registration %d: got %d, want %d", i, got, want)
		}
		counts[got]++
	}
}

func TestTrackGroupFallbackAndInputPreservation(t *testing.T) {
	counts := []int64{0, 0}
	weights := TrackJudgeGroupWeights(2, true, []int64{0, 0}, []int64{9, 2})
	if !reflect.DeepEqual(weights, []float64{9, 2}) {
		t.Fatalf("unexpected fallback %v", weights)
	}
	if got := AllocateTrackGroups(counts, weights, 2); !reflect.DeepEqual(got, []int64{0, 1}) {
		t.Fatalf("got %v", got)
	}
	if !reflect.DeepEqual(counts, []int64{0, 0}) {
		t.Fatal("mutated caller counts")
	}
	if got := AllocateTrackGroups(counts, weights, 0); len(got) != 0 {
		t.Fatalf("zero judges: %v", got)
	}
	if got := AllocateTrackGroups(nil, nil, 1); !reflect.DeepEqual(got, []int64{0}) {
		t.Fatalf("no groups: %v", got)
	}
}
