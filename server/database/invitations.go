package database

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"server/models"
	"time"
)

// Expired attempts are recoverable after interruption. Each mail exchange has
// a 30-second deadline; the lease also prevents simultaneous retry requests.
const InvitationLease = 2 * time.Minute

func RetryableInvitations(now time.Time) bson.M {
	return bson.M{"active": true, "$or": []bson.M{
		{"invitation_status": bson.M{"$in": []string{"pending", "failed"}}},
		{"invitation_status": "sending", "invitation_updated_at": bson.M{"$lt": primitive.NewDateTimeFromTime(now.Add(-InvitationLease))}},
	}}
}

func ClaimJudgeInvitation(db *mongo.Database, ctx context.Context, id primitive.ObjectID, manual bool) (*models.Judge, error) {
	filter := RetryableInvitations(time.Now())
	filter["_id"] = id
	if manual {
		delete(filter, "active")
		filter["$or"] = []bson.M{
			{"invitation_status": bson.M{"$ne": "sending"}},
			{"invitation_status": "sending", "invitation_updated_at": bson.M{"$lt": primitive.NewDateTimeFromTime(time.Now().Add(-InvitationLease))}},
		}
	}
	update := bson.M{"$set": bson.M{"invitation_status": "sending", "invitation_updated_at": primitive.NewDateTimeFromTime(time.Now()), "invitation_attempt": primitive.NewObjectID()}}
	var judge models.Judge
	err := db.Collection("judges").FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&judge)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &judge, err
}

func FinishJudgeInvitation(db *mongo.Database, ctx context.Context, judge *models.Judge, status string) error {
	result, err := db.Collection("judges").UpdateOne(ctx, bson.M{"_id": judge.Id, "email": judge.Email, "invitation_attempt": judge.InvitationAttempt}, bson.M{"$set": bson.M{"invitation_status": status, "invitation_updated_at": primitive.NewDateTimeFromTime(time.Now())}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return errors.New("judge changed while sending; refresh invitation status")
	}
	return nil
}
