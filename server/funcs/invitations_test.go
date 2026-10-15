package funcs

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/url"
	"os"
	"reflect"
	"server/database"
	"server/models"
	"testing"
	"time"
)

func TestJudgeCSVValidation(t *testing.T) {
	text := "\ufeffName,Email,Track,Notes\r\n Ada , ada@example.com ,,\"design, notes\"\r\nBob,bob@example.com,Hardware\r\n"
	judges, err := ParseJudgeCSV(text, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(judges) != 2 || judges[0].Name != "Ada" || judges[0].Email != "ada@example.com" || judges[0].Notes != "design, notes" {
		t.Fatalf("unexpected parsed rows: %+v", judges)
	}
	if issues := ValidateJudgeImport(judges, []string{"Hardware"}, true); len(issues) != 0 {
		t.Fatal(issues)
	}
	judges[1].Email = "ADA@example.com"
	judges[1].Track = ""
	if issues := ValidateJudgeImport(judges, nil, true); len(issues) != 1 {
		t.Fatal("expected duplicate", issues)
	}
	judges[1].Track = "Hardware"
	if issues := ValidateJudgeImport(judges, []string{"Hardware"}, true); len(issues) != 0 {
		t.Fatal("different tracks must be allowed", issues)
	}
	judges[1].Name = ""
	judges[1].Email = "bad@"
	judges[1].Track = "Typo"
	if issues := ValidateJudgeImport(judges, []string{"Hardware"}, true); len(issues) != 3 {
		t.Fatal("expected all row issues", issues)
	}
	if _, err := ParseJudgeCSV("A,a@example.com,x,y,z", false); err == nil {
		t.Fatal("extra columns accepted")
	}
	if issues := ValidateJudgeImport(nil, nil, false); len(issues) == 0 {
		t.Fatal("empty import accepted")
	}
}

// Uses only a disposable localhost database and an injected fake email sender.
func TestSavedInvitationsWithMongo(t *testing.T) {
	uri := os.Getenv("JURY_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set JURY_TEST_MONGO_URI to disposable localhost MongoDB")
	}
	parsed, err := url.Parse(uri)
	if err != nil || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost") {
		t.Fatal("test MongoDB must be localhost")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database("jury_invitation_test_" + primitive.NewObjectID().Hex())
	defer db.Drop(context.Background())
	first := models.NewJudge("First", "first@example.com", "", "", 0)
	second := models.NewJudge("Second", "second@example.com", "Hardware", "", 1)
	for _, judge := range []*models.Judge{first, second} {
		judge.Id = primitive.NewObjectID()
		judge.InvitationStatus = "pending"
	}
	judges := []*models.Judge{first, second}
	if err := database.InsertJudges(db, ctx, judges); err != nil {
		t.Fatal(err)
	}
	originalCodes := []string{first.Code, second.Code}
	calls := 0
	report, err := SendSavedInvitations(ctx, db, judges, "https://example.com", false, func(j *models.Judge, _ string) error {
		calls++
		saved, e := database.FindJudge(db, ctx, j.Id)
		if e != nil || saved == nil || saved.Code != j.Code {
			t.Fatal("email attempted before account saved", e)
		}
		if j.Id == first.Id {
			return errors.New("simulated transport failure")
		}
		return nil
	})
	if err != nil || report.Accepted != 1 || report.Failed != 1 || calls != 2 {
		t.Fatal(report, err, calls)
	}
	// Reload from MongoDB, as a fresh browser request would after a refresh.
	saved, err := database.FindAllJudges(db, ctx)
	if err != nil {
		t.Fatal(err)
	}
	report, err = SendSavedInvitations(ctx, db, saved, "https://example.com", false, func(j *models.Judge, _ string) error {
		if j.Id != first.Id {
			t.Fatal("resent an accepted invitation")
		}
		return nil
	})
	if err != nil || report.Accepted != 1 || report.Skipped != 1 {
		t.Fatal(report, err)
	}
	for i, j := range judges {
		got, e := database.FindJudge(db, ctx, j.Id)
		if e != nil || got.Code != originalCodes[i] || got.Group != j.Group || got.InvitationStatus != "accepted" {
			t.Fatal("account changed", got, e)
		}
	}
	count, _ := db.Collection("judges").CountDocuments(ctx, bson.M{})
	if count != 2 {
		t.Fatal("retry created accounts")
	}
	// Opted-out accounts are excluded from bulk retry, but manual resend is allowed.
	db.Collection("judges").UpdateOne(ctx, bson.M{"_id": first.Id}, bson.M{"$set": bson.M{"invitation_status": "not_sent"}})
	report, err = SendSavedInvitations(ctx, db, []*models.Judge{first}, "", false, func(*models.Judge, string) error { t.Fatal("opt-out emailed"); return nil })
	if err != nil || report.Skipped != 1 {
		t.Fatal(report, err)
	}
	// A claim is exclusive until finished or expired, and old attempts cannot
	// overwrite a recovered attempt's result.
	claimed, err := database.ClaimJudgeInvitation(db, ctx, first.Id, true)
	if err != nil || claimed == nil {
		t.Fatal(err)
	}
	again, err := database.ClaimJudgeInvitation(db, ctx, first.Id, true)
	if err != nil || again != nil {
		t.Fatal("duplicate sending claim", err)
	}
	db.Collection("judges").UpdateOne(ctx, bson.M{"_id": first.Id}, bson.M{"$set": bson.M{"invitation_updated_at": primitive.NewDateTimeFromTime(time.Now().Add(-3 * time.Minute))}})
	recovered, err := database.ClaimJudgeInvitation(db, ctx, first.Id, false)
	if err != nil || recovered == nil {
		t.Fatal("expired attempt not retryable", err)
	}
	if err := database.FinishJudgeInvitation(db, ctx, claimed, "accepted"); err == nil {
		t.Fatal("old attempt overwrote new attempt")
	}
	if err := database.FinishJudgeInvitation(db, ctx, recovered, "failed"); err != nil {
		t.Fatal(err)
	}
	// Editing the address invalidates its old invitation status, preserving code.
	if err := database.UpdateJudgeBasicInfo(db, &first.Id, &models.AddJudgeRequest{Name: first.Name, Email: "corrected@example.com"}); err != nil {
		t.Fatal(err)
	}
	report, err = SendSavedInvitations(ctx, db, []*models.Judge{first}, "", true, func(j *models.Judge, _ string) error {
		if j.Email != "corrected@example.com" || j.Code != originalCodes[0] {
			t.Fatal("resend used old address or new code")
		}
		return nil
	})
	if err != nil || report.Accepted != 1 {
		t.Fatal(report, err)
	}
	// A cancelled request leaves remaining accounts pending, not lost.
	db.Collection("judges").UpdateOne(ctx, bson.M{"_id": second.Id}, bson.M{"$set": bson.M{"invitation_status": "pending"}})
	stopped, stop := context.WithCancel(ctx)
	stop()
	_, err = SendSavedInvitations(stopped, db, []*models.Judge{second}, "", false, func(*models.Judge, string) error { t.Fatal("sent after cancellation"); return nil })
	got, _ := database.FindJudge(db, ctx, second.Id)
	if !errors.Is(err, context.Canceled) || got.InvitationStatus != "pending" {
		t.Fatal("cancelled import lost pending status", err)
	}
	// No send is possible for a nonexistent/unsaved account.
	phantom := *first
	phantom.Id = primitive.NewObjectID()
	report, err = SendSavedInvitations(ctx, db, []*models.Judge{&phantom}, "", true, func(*models.Judge, string) error { t.Fatal("emailed nonexistent account"); return nil })
	if err != nil || !reflect.DeepEqual(report, InvitationReport{Skipped: 1}) {
		t.Fatal(report, err)
	}
}
