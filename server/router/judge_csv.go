package router

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"net/http"
	"server/database"
	"server/funcs"
	"server/util"
	"strings"
)

// AddJudgesCsv validates all rows and commits accounts before sending any mail.
func AddJudgesCsv(ctx *gin.Context) {
	state := GetState(ctx)
	file, err := ctx.FormFile("csv")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "could not read CSV upload"})
		return
	}
	f, err := file.Open()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "could not open CSV upload"})
		return
	}
	defer f.Close()
	content, err := io.ReadAll(io.LimitReader(f, (5<<20)+1))
	if err != nil || len(content) > 5<<20 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "CSV could not be read or exceeds 5 MB"})
		return
	}
	// Accept the previous client field during rolling deployments as well.
	header := ctx.PostForm("hasHeader") == "true" || ctx.PostForm("headerRow") == "true"
	noSend := ctx.PostForm("noSend") == "true"
	judges, err := funcs.ParseJudgeCSV(string(content), header)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "error parsing CSV: " + err.Error()})
		return
	}
	options, err := database.GetOptions(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not load configured tracks"})
		return
	}
	issues := funcs.ValidateJudgeImport(judges, options.Tracks, header)
	if len(issues) > 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": strings.Join(issues, "\n")})
		return
	}
	existing, err := database.FindAllJudges(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not check existing judges"})
		return
	}
	accounts := map[string]bool{}
	for _, judge := range existing {
		accounts[strings.ToLower(strings.TrimSpace(judge.Email))+"\x00"+judge.Track] = true
	}
	for i, judge := range judges {
		if accounts[strings.ToLower(judge.Email)+"\x00"+judge.Track] {
			row := i + 1
			if header {
				row++
			}
			issues = append(issues, fmt.Sprintf("Row %d: an account already exists for this email and track. Use Resend Invitation instead of importing again.", row))
		}
		judge.Id = primitive.NewObjectID()
		judge.InvitationStatus = "pending"
		if noSend {
			judge.InvitationStatus = "not_sent"
		}
	}
	if len(issues) > 0 {
		ctx.JSON(http.StatusConflict, gin.H{"error": strings.Join(issues, "\n")})
		return
	}
	err = database.WithTransaction(state.Db, func(sc mongo.SessionContext) error {
		if err := database.AssignJudgeGroups(state.Db, sc, judges, false); err != nil {
			return err
		}
		return database.InsertJudges(state.Db, sc, judges)
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "No invitations sent: could not save judge accounts"})
		return
	}
	report := funcs.InvitationReport{}
	warning := ""
	if !noSend {
		report, err = funcs.SendSavedInvitations(ctx, state.Db, judges, util.GetFullHostname(ctx), false, funcs.SendJudgeEmail)
		if err != nil {
			warning = "Accounts are saved. Refresh invitation status and retry pending invitations; do not reimport this CSV."
		}
	}
	state.Logger.AdminLogf("Added %d judges from CSV; invitations: %d accepted, %d failed", len(judges), report.Accepted, report.Failed)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1, "created": len(judges), "invitations": report, "warning": warning})
}
