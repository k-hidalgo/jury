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
			options.MinViews = 1
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

// A finalist must not bypass an eligible project needing another overall view.
func TestFinalistReviewRespectsGeneralMinimum(t *testing.T) {
	unseen := &models.Project{Name: "unseen", Seen: 0}
	once := &models.Project{Name: "once", Seen: 1}
	alsoOnce := &models.Project{Name: "also once", Seen: 1}
	twice := &models.Project{Name: "twice", Seen: 2}
	finalist := &models.Project{Name: "finalist", Seen: 3, Prioritized: true}
	for _, tc := range []struct {
		name           string
		minimum        int64
		projects, want []*models.Project
	}{
		{"second view before finalist", 2, []*models.Project{finalist, once}, []*models.Project{once}},
		{"third view before finalist", 3, []*models.Project{finalist, twice}, []*models.Project{twice}},
		{"fewest views first", 3, []*models.Project{twice, finalist, once, unseen}, []*models.Project{unseen}},
		{"equal coverage needs", 2, []*models.Project{once, finalist, alsoOnce}, []*models.Project{once, alsoOnce}},
		{"minimum met releases finalist", 2, []*models.Project{twice, finalist}, []*models.Project{finalist}},
		{"zero disables minimum coverage", 0, []*models.Project{unseen, finalist}, []*models.Project{finalist}},
		{"general minimum above finalist target", 5, []*models.Project{finalist, once}, []*models.Project{once}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := models.NewOptions()
			options.MinViews, options.FinalistReview, options.FinalistTargetViews = tc.minimum, true, 4
			got := finalistReviewCandidates(tc.projects, options)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v; want %v", got, tc.want)
			}
		})
	}
}
