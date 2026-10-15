package router

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"server/database"
)

type finalistGroupCoverage struct {
	Group       int64  `json:"group"`
	Name        string `json:"name"`
	Unseen      int    `json:"unseen"`
	Shortlisted int    `json:"shortlisted"`
	BelowTarget int    `json:"below_target"`
}

// GetFinalistReview reports COMPLETED overall views of active projects, never track
// views or in-progress assignments. No project names, judge codes or rankings exposed.
func GetFinalistReview(ctx *gin.Context) {
	state := GetState(ctx)
	options, err := database.GetOptions(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error loading finalist settings"})
		return
	}
	projects, err := database.FindActiveProjects(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error loading finalist coverage"})
		return
	}
	groups := []finalistGroupCoverage{}
	indexes := map[int64]int{}
	addGroup := func(group int64) int {
		if idx, ok := indexes[group]; ok {
			return idx
		}
		name := fmt.Sprintf("Group %d", group+1)
		if group >= 0 && group < int64(len(options.GroupNames)) && options.GroupNames[group] != "" {
			name = options.GroupNames[group]
		}
		idx := len(groups)
		indexes[group] = idx
		groups = append(groups, finalistGroupCoverage{Group: group, Name: name})
		return idx
	}
	if options.MultiGroup {
		for i := int64(0); i < options.NumGroups; i++ {
			addGroup(i)
		}
	} else {
		addGroup(0)
		groups[0].Name = "All projects"
	}
	unseen, shortlisted, below := 0, 0, 0
	for _, p := range projects {
		group := p.Group
		if !options.MultiGroup {
			group = 0
		}
		idx := addGroup(group)
		if p.Seen == 0 {
			unseen++
			groups[idx].Unseen++
		}
		if p.Prioritized {
			shortlisted++
			groups[idx].Shortlisted++
			if p.Seen < options.FinalistTargetViews {
				below++
				groups[idx].BelowTarget++
			}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"enabled": options.FinalistReview, "target_views": options.FinalistTargetViews, "unseen": unseen, "shortlisted": shortlisted, "below_target": below, "groups": groups})
}
