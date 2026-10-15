package funcs

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"net/mail"
	"server/database"
	"server/models"
	"strings"
	"time"
)

func ValidInvitationEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email && CheckEmail(email)
}

// Validate the entire batch before saving accounts or sending any emails.
func ValidateJudgeImport(judges []*models.Judge, tracks []string, header bool) []string {
	issues := []string{}
	seen := map[string]bool{}
	offset := 1
	if header {
		offset++
	}
	if len(judges) == 0 {
		return []string{"CSV contains no judges"}
	}
	for i, judge := range judges {
		row := i + offset
		if judge.Name == "" {
			issues = append(issues, fmt.Sprintf("Row %d: name is required", row))
		}
		if !ValidInvitationEmail(judge.Email) {
			issues = append(issues, fmt.Sprintf("Row %d: invalid email address", row))
		}
		if judge.Track != "" && !contains(tracks, judge.Track) {
			issues = append(issues, fmt.Sprintf("Row %d: track %q is not in List of Tracks", row, judge.Track))
		}
		key := strings.ToLower(judge.Email) + "\x00" + judge.Track
		if seen[key] {
			issues = append(issues, fmt.Sprintf("Row %d: duplicate email and track in this CSV", row))
		}
		seen[key] = true
	}
	return issues
}

type InvitationReport struct {
	Accepted int `json:"accepted"`
	Failed   int `json:"failed"`
	Skipped  int `json:"skipped"`
}

// Sends saved accounts only; never creates an account or changes its code.
// A cancelled request leaves pending rows available for explicit retry.
func SendSavedInvitations(ctx context.Context, db *mongo.Database, judges []*models.Judge, hostname string, manual bool, send func(*models.Judge, string) error) (InvitationReport, error) {
	report := InvitationReport{}
	for _, item := range judges {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		judge, err := database.ClaimJudgeInvitation(db, ctx, item.Id, manual)
		if err != nil {
			return report, err
		}
		if judge == nil {
			report.Skipped++
			continue
		}
		if !ValidInvitationEmail(judge.Email) {
			err = fmt.Errorf("invalid recipient email")
		} else {
			err = send(judge, hostname)
		}
		status := "accepted"
		if err != nil {
			status = "failed"
		}
		// Persist the outcome even if the browser disconnects during SMTP delivery.
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		saveErr := database.FinishJudgeInvitation(db, saveCtx, judge, status)
		cancel()
		if saveErr != nil {
			return report, saveErr
		}
		if status == "accepted" {
			report.Accepted++
		} else {
			report.Failed++
		}
	}
	return report, nil
}
