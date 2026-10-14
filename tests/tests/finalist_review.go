package tests

import (
	stdcontext "context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"tests/util"
)

func FinalistReviewSettingsAndCoverage(c *util.Context) util.Result {
	defer resetGroupOptions(c)
	defer util.PostRequest(c.Logger, "/admin/options", util.H{"finalist_review": false, "finalist_target_views": 4}, util.AdminAuth())
	if res := setupGroups(c, []int64{2, 1}, 3, false); !res.Success {
		return res
	}
	util.PostRequest(c.Logger, "/admin/options", util.H{"finalist_review": false, "finalist_target_views": 4, "group_names": []string{"Grand", "Rion"}}, util.AdminAuth())
	status, _ := util.GetRequestWithStatus(c.Logger, "/admin/finalist-review", util.DefaultAuth())
	if status >= 200 && status < 300 {
		return util.NewResult(false, "unauthenticated coverage exposed")
	}
	for _, bad := range []any{0, -1, 1.5} {
		status, _ := util.PostRequestWithStatus(c.Logger, "/admin/options", util.H{"finalist_review": true, "finalist_target_views": bad}, util.AdminAuth())
		if status != 400 {
			return util.NewResult(false, "invalid target accepted")
		}
	}
	var settings struct {
		Enabled bool  `json:"enabled"`
		Target  int64 `json:"target_views"`
	}
	if err := json.Unmarshal([]byte(util.GetRequest(c.Logger, "/admin/finalist-review", util.AdminAuth())), &settings); err != nil {
		return util.NewResult(false, err.Error())
	}
	if settings.Enabled || settings.Target != 4 {
		return util.NewResult(false, "invalid update partially changed settings")
	}
	for location, seen := range map[int]int{2: 2, 3: 4} {
		_, err := c.Db.Collection("projects").UpdateMany(stdcontext.Background(), bson.M{"location": location}, bson.M{"$set": bson.M{"seen": seen, "prioritized": true, "track_seen": bson.M{"Track": 999}}})
		if err != nil {
			return util.NewResult(false, err.Error())
		}
	}
	if res := util.PostRequest(c.Logger, "/admin/options", util.H{"finalist_review": true, "finalist_target_views": 4}, util.AdminAuth()); !util.IsOk(res) {
		return util.NewResult(false, "enable failed")
	}
	var coverage struct {
		Enabled     bool `json:"enabled"`
		Unseen      int  `json:"unseen"`
		Shortlisted int  `json:"shortlisted"`
		Below       int  `json:"below_target"`
		Groups      []struct {
			Group  int `json:"group"`
			Unseen int `json:"unseen"`
			Below  int `json:"below_target"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(util.GetRequest(c.Logger, "/admin/finalist-review", util.AdminAuth())), &coverage); err != nil {
		return util.NewResult(false, err.Error())
	}
	if !coverage.Enabled || coverage.Unseen != 1 || coverage.Shortlisted != 2 || coverage.Below != 1 || len(coverage.Groups) != 2 {
		return util.NewResult(false, "incorrect completed-overall coverage")
	}
	if coverage.Groups[0].Unseen != 1 || coverage.Groups[0].Below != 1 || coverage.Groups[1].Below != 0 {
		return util.NewResult(false, "incorrect coverage by room")
	}
	return util.ResultOk()
}

func FinalistReviewKeepsCurrentAssignment(c *util.Context) util.Result {
	defer resetGroupOptions(c)
	defer util.PostRequest(c.Logger, "/admin/options", util.H{"finalist_review": false}, util.AdminAuth())
	if res := setupGroups(c, []int64{2}, 2, false); !res.Success {
		return res
	}
	util.PostRequest(c.Logger, "/admin/options", util.H{"finalist_review": false}, util.AdminAuth())
	token, res := createNamedJudge(c, "finalist_current@example.com", "Current assignment")
	if !res.Success {
		return res
	}
	auth := util.JudgeAuth(token)
	first := util.PostRequest(c.Logger, "/judge/next", nil, auth)
	id := util.ExtractString(first, "project_id")
	if id == "" {
		return util.NewResult(false, "could not assign initial project")
	}
	if _, err := c.Db.Collection("projects").UpdateMany(stdcontext.Background(), bson.M{}, bson.M{"$set": bson.M{"prioritized": true}}); err != nil {
		return util.NewResult(false, err.Error())
	}
	util.PostRequest(c.Logger, "/admin/options", util.H{"finalist_review": true}, util.AdminAuth())
	second := util.PostRequest(c.Logger, "/judge/next", nil, auth)
	if util.ExtractString(second, "project_id") != id {
		return util.NewResult(false, "enabling review replaced current assignment")
	}
	return util.ResultOk()
}

// Exercise the actual Next Project endpoint so CI checks selection as well as settings.
func FinalistReviewAssignmentOrder(c *util.Context) util.Result {
	defer resetGroupOptions(c)
	defer util.PostRequest(c.Logger, "/admin/options", util.H{"finalist_review": false}, util.AdminAuth())
	if res := setupGroups(c, []int64{2, 1}, 3, false); !res.Success {
		return res
	}
	for i, tc := range []struct {
		enabled, priority  bool
		ordinary, finalist int
		want               int
	}{
		{false, true, 1, 2, 1}, // Original lowest-view filtering.
		{true, true, 1, 2, 2},  // Finalist priority now crosses that filter.
		{true, true, 0, 2, 1},  // Unseen coverage is protected.
		{true, true, 1, 4, 1},  // Target reached: normal assignment.
		{true, false, 1, 2, 1}, // On with no shortlist is normal assignment.
	} {
		if _, err := c.Db.Collection("judges").DeleteMany(stdcontext.Background(), bson.M{}); err != nil {
			return util.NewResult(false, err.Error())
		}
		for location, fields := range map[int]bson.M{
			1: {"seen": tc.ordinary, "prioritized": false},
			2: {"seen": tc.finalist, "prioritized": tc.priority},
			3: {"seen": 0, "prioritized": true}, // Never leave room 1 just to chase this project.
		} {
			if _, err := c.Db.Collection("projects").UpdateMany(stdcontext.Background(), bson.M{"location": location}, bson.M{"$set": fields}); err != nil {
				return util.NewResult(false, err.Error())
			}
		}
		util.PostRequest(c.Logger, "/admin/options", util.H{"finalist_review": tc.enabled, "finalist_target_views": 4}, util.AdminAuth())
		email := fmt.Sprintf("finalist-order-%d@example.com", i)
		token, res := createNamedJudge(c, email, "Finalist order")
		if !res.Success {
			return res
		}
		if _, err := c.Db.Collection("judges").UpdateMany(stdcontext.Background(), bson.M{"email": email}, bson.M{"$set": bson.M{"group": 0}}); err != nil {
			return util.NewResult(false, err.Error())
		}
		next := util.PostRequest(c.Logger, "/judge/next", nil, util.JudgeAuth(token))
		id, err := primitive.ObjectIDFromHex(util.ExtractString(next, "project_id"))
		if err != nil {
			return util.NewResult(false, "assignment failed: "+next)
		}
		var p struct {
			Location int `bson:"location"`
		}
		if err := c.Db.Collection("projects").FindOne(stdcontext.Background(), bson.M{"_id": id}).Decode(&p); err != nil {
			return util.NewResult(false, err.Error())
		}
		if p.Location != tc.want {
			return util.NewResult(false, fmt.Sprintf("case %d: got table %d, want %d", i, p.Location, tc.want))
		}
	}
	return util.ResultOk()
}
