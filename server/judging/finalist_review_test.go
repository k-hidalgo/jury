package judging

import (
	"reflect"
	"server/models"
	"testing"
)

func TestFinalistReviewCandidates(t *testing.T) {
	unseen := &models.Project{Name: "unseen", Seen: 0}
	low := &models.Project{Name: "ordinary", Seen: 1}
	finalist := &models.Project{Name: "finalist", Seen: 2, Prioritized: true}
	more := &models.Project{Name: "more", Seen: 3, Prioritized: true}
	reached := &models.Project{Name: "reached", Seen: 4, Prioritized: true}
	for _, tc := range []struct {
		name           string
		enabled        bool
		target         int64
		projects, want []*models.Project
	}{
		{"off unchanged", false, 4, []*models.Project{unseen, finalist}, nil},
		{"no priorities unchanged", true, 4, []*models.Project{unseen, low}, nil},
		{"unseen before finalists", true, 4, []*models.Project{finalist, low, unseen}, []*models.Project{unseen}},
		{"finalists before ordinary", true, 4, []*models.Project{low, finalist}, []*models.Project{finalist}},
		{"fewest completed views among finalists", true, 4, []*models.Project{low, more, finalist}, []*models.Project{finalist}},
		{"at target falls back", true, 4, []*models.Project{reached, low}, nil},
		{"above target falls back", true, 2, []*models.Project{more, low}, nil},
		{"legacy target defaults to four", true, 0, []*models.Project{low, finalist}, []*models.Project{finalist}},
		{"empty candidates", true, 4, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := models.NewOptions()
			options.FinalistReview = tc.enabled
			options.FinalistTargetViews = tc.target
			before := append([]*models.Project(nil), tc.projects...)
			got := finalistReviewCandidates(tc.projects, options)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if !reflect.DeepEqual(before, tc.projects) {
				t.Fatal("mutated candidate order")
			}
		})
	}
}
