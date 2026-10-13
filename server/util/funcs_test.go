package util

import (
	"server/models"
	"testing"
)

func TestGroupFromTableFillsGroupsBySize(t *testing.T) {
	op := &models.Options{GroupSizes: []int64{8, 2, 2}}

	counts := make([]int64, 3)
	for table := int64(1); table <= 12; table++ {
		counts[GroupFromTable(op, table)]++
	}
	assertCounts(t, counts, []int64{8, 2, 2})
}

func TestGroupFromTableBoundaries(t *testing.T) {
	op := &models.Options{GroupSizes: []int64{8, 2, 2}}

	tests := []struct {
		table int64
		group int64
	}{
		{1, 0},
		{8, 0},
		{9, 1},
		{10, 1},
		{11, 2},
		{12, 2},
		// Tables past the end of the group sizes overflow into the last group
		{13, 2},
		{100, 2},
	}
	for _, test := range tests {
		if got := GroupFromTable(op, test.table); got != test.group {
			t.Errorf("GroupFromTable(%d) = %d, want %d", test.table, got, test.group)
		}
	}
}

func TestGroupFromTableSingleOrNoGroups(t *testing.T) {
	if got := GroupFromTable(&models.Options{GroupSizes: []int64{5}}, 20); got != 0 {
		t.Errorf("single group: got %d, want 0", got)
	}
	if got := GroupFromTable(&models.Options{GroupSizes: []int64{}}, 3); got != 0 {
		t.Errorf("no group sizes: got %d, want 0", got)
	}
}

func TestGroupFirstAndLastTable(t *testing.T) {
	op := &models.Options{GroupSizes: []int64{8, 2, 2}}

	tests := []struct {
		group int64
		first int64
		last  int64
	}{
		{0, 1, 8},
		{1, 9, 10},
		{2, 11, 12},
	}
	for _, test := range tests {
		if got := GroupFirstTable(op, test.group); got != test.first {
			t.Errorf("GroupFirstTable(%d) = %d, want %d", test.group, got, test.first)
		}
		if got := GroupLastTable(op, test.group); got != test.last {
			t.Errorf("GroupLastTable(%d) = %d, want %d", test.group, got, test.last)
		}
	}
}

func TestGroupTablesMatchGroupFromTable(t *testing.T) {
	op := &models.Options{GroupSizes: []int64{3, 1, 4, 2}}

	for group := int64(0); group < 4; group++ {
		for table := GroupFirstTable(op, group); table <= GroupLastTable(op, group); table++ {
			if got := GroupFromTable(op, table); got != group {
				t.Errorf("table %d is in group %d, but GroupFromTable returned %d", table, group, got)
			}
		}
	}
}
