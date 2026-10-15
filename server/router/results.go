package router

import (
	"net/http"
	"server/database"
	"server/funcs"
	"server/judging"
	"time"

	"github.com/gin-gonic/gin"
)

// ExportFinalResults downloads an Excel workbook of the current stored results.
// It is registered under the existing admin authentication middleware.
func ExportFinalResults(ctx *gin.Context) {
	state := GetState(ctx)
	projects, err := database.FindAllProjects(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting projects: " + err.Error()})
		return
	}
	// Match ListProjects: scores and track stars are aggregated from judges,
	// rather than trusting the legacy totals stored on project documents.
	scores, err := judging.AggregateScores(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error calculating scores: " + err.Error()})
		return
	}
	for _, project := range projects {
		if score, ok := scores[project.Id]; ok {
			project.Score = score.Score
			project.Stars = score.Stars
			project.TrackStars = score.TrackStars
		}
	}
	options, err := database.GetOptions(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error getting options: " + err.Error()})
		return
	}
	exportedAt := time.Now().UTC()
	data, err := funcs.CreateFinalResultsWorkbook(projects, options, exportedAt)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "error creating results workbook: " + err.Error()})
		return
	}
	ctx.Header("Content-Disposition", "attachment; filename=\"final-results-"+exportedAt.Format("2006-01-02-150405")+".xlsx\"")
	ctx.Header("Cache-Control", "no-store")
	state.Logger.AdminLogf("Exported final results workbook")
	ctx.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}
