package router

import (
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"server/database"
	"server/funcs"
	"server/models"
	"server/util"
	"time"
)

type invitationItem struct {
	ID     primitive.ObjectID `json:"id"`
	Name   string             `json:"name"`
	Email  string             `json:"email"`
	Track  string             `json:"track"`
	Status string             `json:"status"`
}

// Admin-only summary survives page refreshes. Legacy accounts have unknown
// status and are never automatically included in bulk retry.
func GetInvitations(ctx *gin.Context) {
	state := GetState(ctx)
	judges, err := database.FindAllJudges(state.Db, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not load invitations"})
		return
	}
	counts := map[string]int{"pending": 0, "sending": 0, "accepted": 0, "failed": 0, "not_sent": 0, "unknown": 0}
	items := []invitationItem{}
	for _, judge := range judges {
		if !judge.Active {
			continue
		}
		status := judge.InvitationStatus
		if status == "" {
			status = "unknown"
		}
		counts[status]++
		if status == "pending" || status == "sending" || status == "failed" {
			items = append(items, invitationItem{judge.Id, judge.Name, judge.Email, judge.Track, status})
		}
	}
	retryable, err := state.Db.Collection("judges").CountDocuments(ctx, database.RetryableInvitations(time.Now()))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not load retry status"})
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, gin.H{"counts": counts, "items": items, "retryable": retryable})
}

func RetryInvitations(ctx *gin.Context) {
	state := GetState(ctx)
	cursor, err := state.Db.Collection("judges").Find(ctx, database.RetryableInvitations(time.Now()))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not load pending invitations"})
		return
	}
	defer cursor.Close(ctx)
	judges := []*models.Judge{}
	if err := cursor.All(ctx, &judges); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not read pending invitations"})
		return
	}
	report, err := funcs.SendSavedInvitations(ctx, state.Db, judges, util.GetFullHostname(ctx), false, funcs.SendJudgeEmail)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Accounts are saved. Refresh invitation status before retrying."})
		return
	}
	state.Logger.AdminLogf("Retried judge invitations: %d accepted, %d failed", report.Accepted, report.Failed)
	ctx.JSON(http.StatusOK, gin.H{"ok": 1, "invitations": report})
}

func ResendJudgeInvitation(ctx *gin.Context) {
	state := GetState(ctx)
	id, err := primitive.ObjectIDFromHex(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid judge ID"})
		return
	}
	judge, err := database.FindJudge(state.Db, ctx, id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not load judge"})
		return
	}
	if judge == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "judge not found"})
		return
	}
	report, err := funcs.SendSavedInvitations(ctx, state.Db, []*models.Judge{judge}, util.GetFullHostname(ctx), true, funcs.SendJudgeEmail)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Account is saved. Refresh invitation status before retrying."})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": 1, "invitations": report})
}
