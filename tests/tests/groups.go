package tests

import (
	"fmt"
	"slices"
	"tests/util"
)

// --- Multi-group Judge Assignment Tests ---

// GroupJudgingTestSetup starts judging so judges can be added and reassigned
func GroupJudgingTestSetup(context *util.Context) util.Result {
	unpauseRes := util.PostRequest(context.Logger, "/admin/clock/unpause", nil, util.AdminAuth())
	if !util.IsOk(unpauseRes) {
		return util.NewResult(false, "Failed to unpause clock: "+unpauseRes)
	}
	return util.ResultOk()
}

// ReassignSplitsJudgesEvenly verifies that, without proportional assignment,
// judges are split evenly between groups even when the groups are different sizes
func ReassignSplitsJudgesEvenly(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	if res := setupGroups(context, []int64{8, 2, 2}, 12, false); !res.Success {
		return res
	}
	if res := addGroupJudges(context, "even", 12); !res.Success {
		return res
	}

	// Judges added one at a time should already be balanced
	judgeCounts, res := getJudgeGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(judgeCounts, []int64{4, 4, 4}) {
		return util.NewResult(false, fmt.Sprintf("New judges not split evenly: got %v, want [4 4 4]", judgeCounts))
	}

	// Reassigning should keep them balanced
	if res := reassignJudgeGroups(context); !res.Success {
		return res
	}
	judgeCounts, res = getJudgeGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(judgeCounts, []int64{4, 4, 4}) {
		return util.NewResult(false, fmt.Sprintf("Reassigned judges not split evenly: got %v, want [4 4 4]", judgeCounts))
	}

	return util.ResultOk()
}

// NewJudgesAssignedProportionally verifies that, with proportional assignment,
// each new judge is placed so the number of judges in each group matches its number of projects
func NewJudgesAssignedProportionally(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	if res := setupGroups(context, []int64{8, 2, 2}, 12, true); !res.Success {
		return res
	}
	projectCounts, res := getUnevenProjectGroupCounts(context, 3)
	if !res.Success {
		return res
	}

	// With as many judges as projects, each group should get one judge per project
	if res := addGroupJudges(context, "proportional", 12); !res.Success {
		return res
	}
	judgeCounts, res := getJudgeGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(judgeCounts, projectCounts) {
		return util.NewResult(false, fmt.Sprintf("New judges not split proportionally: got %v, want %v", judgeCounts, projectCounts))
	}

	return util.ResultOk()
}

// ReassignSplitsJudgesProportionally verifies that enabling proportional assignment
// and reassigning moves existing judges into proportional groups
func ReassignSplitsJudgesProportionally(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	// Add judges while proportional assignment is off, so they start out even
	if res := setupGroups(context, []int64{8, 2, 2}, 12, false); !res.Success {
		return res
	}
	projectCounts, res := getUnevenProjectGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if res := addGroupJudges(context, "reassign", 12); !res.Success {
		return res
	}

	// Enable proportional assignment and reassign
	if res := setProportionalJudgeGroups(context, true); !res.Success {
		return res
	}
	if res := reassignJudgeGroups(context); !res.Success {
		return res
	}

	judgeCounts, res := getJudgeGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(judgeCounts, projectCounts) {
		return util.NewResult(false, fmt.Sprintf("Reassigned judges not split proportionally: got %v, want %v", judgeCounts, projectCounts))
	}

	return util.ResultOk()
}

// ReassignAfterChangingGroupSizes verifies that reassigning judges after
// changing the group sizes follows the new number of projects in each group
func ReassignAfterChangingGroupSizes(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	if res := setupGroups(context, []int64{4, 4, 4}, 12, true); !res.Success {
		return res
	}
	oldProjectCounts, res := getProjectGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if res := addGroupJudges(context, "resize", 12); !res.Success {
		return res
	}
	judgeCounts, res := getJudgeGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(judgeCounts, oldProjectCounts) {
		return util.NewResult(false, fmt.Sprintf("New judges not split proportionally: got %v, want %v", judgeCounts, oldProjectCounts))
	}

	// Change the group sizes, which moves projects between groups
	sizesRes := util.PostRequest(context.Logger, "/admin/group-sizes", util.H{"group_sizes": []int64{8, 2, 2}}, util.AdminAuth())
	if !util.IsOk(sizesRes) {
		return util.NewResult(false, "Failed to set group sizes: "+sizesRes)
	}
	projectCounts, res := getUnevenProjectGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if slices.Equal(projectCounts, oldProjectCounts) {
		return util.NewResult(false, fmt.Sprintf("Changing group sizes did not move any projects: still %v", projectCounts))
	}

	// Reassign judges to match the new group sizes
	if res := reassignJudgeGroups(context); !res.Success {
		return res
	}
	judgeCounts, res = getJudgeGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(judgeCounts, projectCounts) {
		return util.NewResult(false, fmt.Sprintf("Judges not reassigned to new group sizes: got %v, want %v", judgeCounts, projectCounts))
	}

	return util.ResultOk()
}

// ProportionalUsesGroupSizesWithoutProjects verifies that, before any projects are added,
// proportional assignment falls back to the configured group sizes
func ProportionalUsesGroupSizesWithoutProjects(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	if res := setupGroups(context, []int64{6, 3, 3}, 0, true); !res.Success {
		return res
	}
	if res := addGroupJudges(context, "noprojects", 12); !res.Success {
		return res
	}

	judgeCounts, res := getJudgeGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(judgeCounts, []int64{6, 3, 3}) {
		return util.NewResult(false, fmt.Sprintf("Judges not split by group sizes: got %v, want [6 3 3]", judgeCounts))
	}

	return util.ResultOk()
}

// TrackJudgesNotAssignedToGroups verifies that track judges do not count towards
// or get moved by multi-group judge assignment
func TrackJudgesNotAssignedToGroups(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	if res := setupGroups(context, []int64{6, 3, 3}, 0, true); !res.Success {
		return res
	}

	// Add track judges, which should not affect the general judges' groups
	for i := 0; i < 3; i++ {
		addRes := util.PostRequest(context.Logger, "/judge/new", util.H{
			"name":    fmt.Sprintf("Group Track Judge %d", i),
			"email":   fmt.Sprintf("group_track_%d@example.com", i),
			"track":   "Design",
			"notes":   "",
			"no_send": true,
		}, util.AdminAuth())
		if !util.IsOk(addRes) {
			return util.NewResult(false, "Failed to add track judge: "+addRes)
		}
	}
	if res := addGroupJudges(context, "withtrack", 12); !res.Success {
		return res
	}
	if res := reassignJudgeGroups(context); !res.Success {
		return res
	}

	judgeCounts, res := getJudgeGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(judgeCounts, []int64{6, 3, 3}) {
		return util.NewResult(false, fmt.Sprintf("Track judges affected group assignment: got %v, want [6 3 3]", judgeCounts))
	}

	return util.ResultOk()
}

// setupGroups clears all judges and projects, enables multi-group judging with the given
// group sizes, and adds numProjects projects (which fill the groups in table order)
func setupGroups(context *util.Context, sizes []int64, numProjects int, proportional bool) util.Result {
	for _, resetType := range []string{"judges", "projects"} {
		resetRes := util.PostRequest(context.Logger, "/admin/reset", util.H{"type": resetType}, util.AdminAuth())
		if !util.IsOk(resetRes) {
			return util.NewResult(false, "Failed to reset "+resetType+": "+resetRes)
		}
	}

	optionsRes := util.PostRequest(context.Logger, "/admin/options", util.H{
		"multi_group":               true,
		"judge_tracks":              false,
		"proportional_judge_groups": proportional,
	}, util.AdminAuth())
	if !util.IsOk(optionsRes) {
		return util.NewResult(false, "Failed to enable multi-group judging: "+optionsRes)
	}

	numRes := util.PostRequest(context.Logger, "/admin/num-groups", util.H{"num_groups": len(sizes)}, util.AdminAuth())
	if !util.IsOk(numRes) {
		return util.NewResult(false, "Failed to set number of groups: "+numRes)
	}

	sizesRes := util.PostRequest(context.Logger, "/admin/group-sizes", util.H{"group_sizes": sizes}, util.AdminAuth())
	if !util.IsOk(sizesRes) {
		return util.NewResult(false, "Failed to set group sizes: "+sizesRes)
	}

	for i := 1; i <= numProjects; i++ {
		addRes := util.PostRequest(context.Logger, "/project/new", util.H{
			"name":           fmt.Sprintf("Group Project %d", i),
			"description":    "Test project for multi-group judging",
			"url":            "https://example.com",
			"try_link":       "",
			"video_link":     "",
			"challenge_list": "",
		}, util.AdminAuth())
		if !util.IsOk(addRes) {
			return util.NewResult(false, fmt.Sprintf("Failed to add group project %d: %s", i, addRes))
		}
	}

	return util.ResultOk()
}

// resetGroupOptions turns multi-group judging back off so later tests are unaffected
func resetGroupOptions(context *util.Context) {
	util.PostRequest(context.Logger, "/admin/options", util.H{
		"multi_group":               false,
		"proportional_judge_groups": false,
	}, util.AdminAuth())
}

// setProportionalJudgeGroups toggles proportional judge group assignment
func setProportionalJudgeGroups(context *util.Context, proportional bool) util.Result {
	res := util.PostRequest(context.Logger, "/admin/options", util.H{"proportional_judge_groups": proportional}, util.AdminAuth())
	return util.AssertOk(res, "Failed to set proportional judge groups")
}

// reassignJudgeGroups reassigns all general judges to groups
func reassignJudgeGroups(context *util.Context) util.Result {
	res := util.PostRequest(context.Logger, "/judge/reassign", nil, util.AdminAuth())
	return util.AssertOk(res, "Failed to reassign judge groups")
}

// addGroupJudges adds n general (non-track) judges one at a time
func addGroupJudges(context *util.Context, prefix string, n int) util.Result {
	for i := 0; i < n; i++ {
		addRes := util.PostRequest(context.Logger, "/judge/new", util.H{
			"name":    fmt.Sprintf("Group Judge %s %d", prefix, i),
			"email":   fmt.Sprintf("group_%s_%d@example.com", prefix, i),
			"track":   "",
			"notes":   "",
			"no_send": true,
		}, util.AdminAuth())
		if !util.IsOk(addRes) {
			return util.NewResult(false, "Failed to add group judge: "+addRes)
		}
	}
	return util.ResultOk()
}

// getJudgeGroupCounts returns the number of general (non-track) judges in each group
func getJudgeGroupCounts(context *util.Context, numGroups int) ([]int64, util.Result) {
	var judges []struct {
		Track string `json:"track"`
		Group int64  `json:"group"`
	}
	listRes := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	if err := jsonUnmarshalList(listRes, &judges); err != nil {
		return nil, util.NewResult(false, "Failed to parse judge list: "+listRes)
	}

	counts := make([]int64, numGroups)
	for _, judge := range judges {
		if judge.Track != "" {
			continue
		}
		if judge.Group < 0 || judge.Group >= int64(numGroups) {
			return nil, util.NewResult(false, fmt.Sprintf("Judge assigned to invalid group %d", judge.Group))
		}
		counts[judge.Group]++
	}
	return counts, util.ResultOk()
}

// getProjectGroupCounts returns the number of active projects in each group
func getProjectGroupCounts(context *util.Context, numGroups int) ([]int64, util.Result) {
	var projects []struct {
		Active bool  `json:"active"`
		Group  int64 `json:"group"`
	}
	listRes := util.GetRequest(context.Logger, "/project/list", util.AdminAuth())
	if err := jsonUnmarshalList(listRes, &projects); err != nil {
		return nil, util.NewResult(false, "Failed to parse project list: "+listRes)
	}

	counts := make([]int64, numGroups)
	for _, project := range projects {
		if !project.Active {
			continue
		}
		if project.Group < 0 || project.Group >= int64(numGroups) {
			return nil, util.NewResult(false, fmt.Sprintf("Project assigned to invalid group %d", project.Group))
		}
		counts[project.Group]++
	}
	return counts, util.ResultOk()
}

// getUnevenProjectGroupCounts returns the number of active projects in each group,
// failing if the groups are all the same size (which would make proportional and even assignment identical)
func getUnevenProjectGroupCounts(context *util.Context, numGroups int) ([]int64, util.Result) {
	counts, res := getProjectGroupCounts(context, numGroups)
	if !res.Success {
		return nil, res
	}
	if slices.Min(counts) == slices.Max(counts) {
		return nil, util.NewResult(false, fmt.Sprintf("Expected uneven project groups, got %v", counts))
	}
	return counts, util.ResultOk()
}

// MoveJudgeGroupValidatesRange verifies that judges can be moved to an existing group,
// but not to a group outside of the configured number of groups
func MoveJudgeGroupValidatesRange(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	if res := setupGroups(context, []int64{6, 3, 3}, 0, false); !res.Success {
		return res
	}
	if res := addGroupJudges(context, "move", 2); !res.Success {
		return res
	}
	id, res := findJudgeIDByEmail(context, "group_move_0@example.com")
	if !res.Success {
		return res
	}
	otherId, res := findJudgeIDByEmail(context, "group_move_1@example.com")
	if !res.Success {
		return res
	}

	// Moving to the last group (stored as 2) should work
	moveRes := util.PutRequest(context.Logger, "/judge/move/group/"+id, util.H{"group": 2}, util.AdminAuth())
	if !util.IsOk(moveRes) {
		return util.NewResult(false, "Failed to move judge to a valid group: "+moveRes)
	}
	listRes := util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	if group := findJudgeFieldByEmail(listRes, "group_move_0@example.com", "group"); group != "2" {
		return util.NewResult(false, "Judge not moved to group 2, got "+group)
	}

	// Groups outside of the range should be rejected, for one judge or many
	for _, group := range []int{-1, 3} {
		moveRes = util.PutRequest(context.Logger, "/judge/move/group/"+id, util.H{"group": group}, util.AdminAuth())
		if util.IsOk(moveRes) {
			return util.NewResult(false, fmt.Sprintf("Moving a judge to group %d should fail", group))
		}

		moveRes = util.PostRequest(context.Logger, "/judge/move/group", util.H{"items": []string{id, otherId}, "group": group}, util.AdminAuth())
		if util.IsOk(moveRes) {
			return util.NewResult(false, fmt.Sprintf("Moving selected judges to group %d should fail", group))
		}
	}

	// The judge should still be in the group they were moved to
	listRes = util.GetRequest(context.Logger, "/judge/list", util.AdminAuth())
	if group := findJudgeFieldByEmail(listRes, "group_move_0@example.com", "group"); group != "2" {
		return util.NewResult(false, "Judge group changed after invalid move, got "+group)
	}

	return util.ResultOk()
}

// ProjectsFillGroupsBySize verifies that projects are split into groups
// according to the configured group sizes
func ProjectsFillGroupsBySize(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	if res := setupGroups(context, []int64{8, 2, 2}, 12, false); !res.Success {
		return res
	}
	projectCounts, res := getProjectGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(projectCounts, []int64{8, 2, 2}) {
		return util.NewResult(false, fmt.Sprintf("Projects not split by group sizes: got %v, want [8 2 2]", projectCounts))
	}

	// Changing the group sizes should move projects to match
	sizesRes := util.PostRequest(context.Logger, "/admin/group-sizes", util.H{"group_sizes": []int64{4, 4, 4}}, util.AdminAuth())
	if !util.IsOk(sizesRes) {
		return util.NewResult(false, "Failed to set group sizes: "+sizesRes)
	}
	projectCounts, res = getProjectGroupCounts(context, 3)
	if !res.Success {
		return res
	}
	if !slices.Equal(projectCounts, []int64{4, 4, 4}) {
		return util.NewResult(false, fmt.Sprintf("Projects not split by new group sizes: got %v, want [4 4 4]", projectCounts))
	}

	return util.ResultOk()
}

// MoveProjectGroupUsesGroupTables verifies that projects moved to another group get the
// next table in that group, that full groups are rejected, and that no table numbers are duplicated
func MoveProjectGroupUsesGroupTables(context *util.Context) util.Result {
	defer resetGroupOptions(context)

	// Groups 1 and 2 are full, group 3 is empty
	if res := setupGroups(context, []int64{4, 4, 4}, 8, false); !res.Success {
		return res
	}
	firstId, res := findProjectIDByName(context, "Group Project 1")
	if !res.Success {
		return res
	}
	secondId, res := findProjectIDByName(context, "Group Project 2")
	if !res.Success {
		return res
	}

	// Moving into the empty group should use that group's first table (9)
	moveRes := util.PutRequest(context.Logger, "/project/move/group/"+firstId, util.H{"group": 2}, util.AdminAuth())
	if !util.IsOk(moveRes) {
		return util.NewResult(false, "Failed to move project to empty group: "+moveRes)
	}
	listRes := util.GetRequest(context.Logger, "/project/list", util.AdminAuth())
	if location := findProjectFieldByName(listRes, "Group Project 1", "location"); location != "9" {
		return util.NewResult(false, "Project moved to empty group should be at table 9, got "+location)
	}

	// Moving into a non-empty group should use the next table in that group (10)
	moveRes = util.PutRequest(context.Logger, "/project/move/group/"+secondId, util.H{"group": 2}, util.AdminAuth())
	if !util.IsOk(moveRes) {
		return util.NewResult(false, "Failed to move project to group: "+moveRes)
	}
	listRes = util.GetRequest(context.Logger, "/project/list", util.AdminAuth())
	if location := findProjectFieldByName(listRes, "Group Project 2", "location"); location != "10" {
		return util.NewResult(false, "Project should be moved to table 10, got "+location)
	}

	// Group 2 is still full, so moving a project there should fail, for one project or many
	moveRes = util.PutRequest(context.Logger, "/project/move/group/"+firstId, util.H{"group": 1}, util.AdminAuth())
	if util.IsOk(moveRes) {
		return util.NewResult(false, "Moving a project to a full group should fail")
	}
	moveRes = util.PostRequest(context.Logger, "/project/move/group", util.H{"items": []string{firstId, secondId}, "group": 1}, util.AdminAuth())
	if util.IsOk(moveRes) {
		return util.NewResult(false, "Moving selected projects to a full group should fail")
	}

	// Every project should still have its own table
	var projects []struct {
		Name     string `json:"name"`
		Location int64  `json:"location"`
	}
	listRes = util.GetRequest(context.Logger, "/project/list", util.AdminAuth())
	if err := jsonUnmarshalList(listRes, &projects); err != nil {
		return util.NewResult(false, "Failed to parse project list: "+listRes)
	}
	tables := make(map[int64]string)
	for _, project := range projects {
		if other, ok := tables[project.Location]; ok {
			return util.NewResult(false, fmt.Sprintf("%s and %s are both at table %d", other, project.Name, project.Location))
		}
		tables[project.Location] = project.Name
	}

	return util.ResultOk()
}
