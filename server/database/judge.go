package database

import (
	"context"
	"errors"
	"server/models"
	"server/util"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// InsertJudge inserts a judge into the database
func InsertJudge(db *mongo.Database, ctx context.Context, judge *models.Judge) error {
	_, err := db.Collection("judges").InsertOne(ctx, judge)
	return err
}

// InsertJudges inserts multiple judges into the database
func InsertJudges(db *mongo.Database, ctx context.Context, judges []*models.Judge) error {
	var docs []any
	for _, judge := range judges {
		docs = append(docs, judge)
	}
	_, err := db.Collection("judges").InsertMany(ctx, docs)
	return err
}

// FindJudge finds a judge by ID
func FindJudge(db *mongo.Database, ctx context.Context, id primitive.ObjectID) (*models.Judge, error) {
	var judge models.Judge
	err := db.Collection("judges").FindOne(ctx, gin.H{"_id": id}).Decode(&judge)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &judge, err
}

// FindJudgeByToken finds a judge by their token.
// Returns judge as nil if no judge was found.
// Rejects empty tokens to prevent matching pre-login judges (Token defaults to "" in NewJudge).
func FindJudgeByToken(db *mongo.Database, token string) (*models.Judge, error) {
	if token == "" {
		return nil, nil
	}
	var judge models.Judge
	err := db.Collection("judges").FindOne(context.Background(), gin.H{"token": token}).Decode(&judge)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &judge, err
}

// FindJudgeByCode finds a judge by their code.
// Returns judge as nil if no judge was found.
func FindJudgeByCode(db *mongo.Database, ctx context.Context, code string) (*models.Judge, error) {
	var judge models.Judge
	err := db.Collection("judges").FindOne(ctx, gin.H{"code": code}).Decode(&judge)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &judge, err
}

func FindJudgesByTrack(db *mongo.Database, ctx context.Context, track string) ([]*models.Judge, error) {
	judges := make([]*models.Judge, 0)
	cursor, err := db.Collection("judges").Find(ctx, gin.H{"track": track})
	if err != nil {
		return nil, err
	}
	err = cursor.All(ctx, &judges)
	if err != nil {
		return nil, err
	}
	return judges, nil
}

// UpdateJudgeToken updates the token of a judge
func UpdateJudgeToken(db *mongo.Database, ctx context.Context, judgeId *primitive.ObjectID, token string) error {
	_, err := db.Collection("judges").UpdateOne(ctx, gin.H{"_id": judgeId}, gin.H{"$set": gin.H{"token": token, "last_activity": util.Now()}})
	return err
}

// UpdateJudgeReadWelcome updates the read_welcome field of a judge to true
func UpdateJudgeReadWelcome(db *mongo.Database, ctx context.Context, judgeId *primitive.ObjectID) error {
	_, err := db.Collection("judges").UpdateOne(ctx, gin.H{"_id": judgeId}, gin.H{"$set": gin.H{"read_welcome": true, "last_activity": util.Now()}})
	return err
}

// UpdateJudgeGroups updates the group and group_seen fields of multiple judges
func UpdateJudgeGroups(db *mongo.Database, sc mongo.SessionContext, judges []*models.Judge) error {
	models := make([]mongo.WriteModel, 0, len(judges))
	for _, judge := range judges {
		models = append(models, mongo.NewUpdateOneModel().SetFilter(gin.H{"_id": judge.Id}).SetUpdate(gin.H{"$set": gin.H{"group": judge.Group, "group_seen": judge.GroupSeen}}))
	}
	opts := options.BulkWrite().SetOrdered(false)
	_, err := db.Collection("judges").BulkWrite(sc, models, opts)
	return err
}

// FindAllJudges returns a list of all judges in the database
func FindAllJudges(db *mongo.Database, ctx context.Context) ([]*models.Judge, error) {
	judges := make([]*models.Judge, 0)
	cursor, err := db.Collection("judges").Find(ctx, gin.H{})
	if err != nil {
		return nil, err
	}
	for cursor.Next(ctx) {
		var judge models.Judge
		err := cursor.Decode(&judge)
		if err != nil {
			return nil, err
		}
		judges = append(judges, &judge)
	}
	return judges, nil
}

// AggregateJudgeStats aggregates statistics about judges
func AggregateJudgeStats(db *mongo.Database) (*models.JudgeStats, error) {
	// Get the total number of judges
	totalJudges, err := db.Collection("judges").EstimatedDocumentCount(context.Background())
	if err != nil {
		return nil, err
	}

	// Get the total number of active judges and the average number of votes using an aggregation pipeline
	cursor, err := db.Collection("judges").Aggregate(context.Background(), []gin.H{
		{"$match": gin.H{"active": true}},
		{"$group": gin.H{
			"_id": nil,
			"avgSeen": gin.H{
				"$avg": "$seen",
			},
			"numActive": gin.H{
				"$sum": 1,
			},
		}},
	})
	if err != nil {
		return nil, err
	}

	// Get the first document from the cursor
	var stats models.JudgeStats
	cursor.Next(context.Background())
	err = cursor.Decode(&stats)
	if err != nil {
		if err.Error() == "EOF" {
			stats = models.JudgeStats{Num: 0, AvgSeen: 0, NumActive: 0}
		} else {
			return nil, err
		}
	}

	// Set the total number of judges
	stats.Num = totalJudges

	return &stats, nil
}

// DeleteJudgeById deletes a judge from the database by their id
func DeleteJudgeById(db *mongo.Database, ctx context.Context, id primitive.ObjectID) error {
	_, err := db.Collection("judges").DeleteOne(ctx, gin.H{"_id": id})
	return err
}

// UpdateAfterSeen updates the judge's seen projects and increments the seen count.
// This should be run in a transaction.
func UpdateAfterSeen(db *mongo.Database, ctx context.Context, judge *models.Judge, seenProject *models.JudgedProject) error {
	// Update the judge's seen projects
	_, err := db.Collection("judges").UpdateOne(
		ctx,
		gin.H{"_id": judge.Id},
		gin.H{
			"$push": gin.H{"seen_projects": seenProject},
			"$inc":  gin.H{"seen": 1},
			"$set":  gin.H{"current": nil, "group": judge.Group, "group_seen": judge.GroupSeen, "last_activity": util.Now()},
		},
	)
	if err != nil {
		return errors.New("error updating judge: " + err.Error())
	}

	if judge.Track != "" {
		err = IncrementTrackSeen(db, ctx, seenProject.ProjectId, judge.Track)
	} else {
		_, err = db.Collection("projects").UpdateOne(
			ctx,
			gin.H{"_id": seenProject.ProjectId},
			gin.H{"$inc": gin.H{"seen": 1}, "$set": gin.H{"last_activity": util.Now()}},
		)
	}
	if err != nil {
		return errors.New("error updating project: " + err.Error())
	}

	return nil
}

// SetJudgeActive sets the active field of a judge
func SetJudgeActive(db *mongo.Database, id *primitive.ObjectID, active bool) error {
	_, err := db.Collection("judges").UpdateOne(
		context.Background(),
		gin.H{"_id": id},
		gin.H{"$set": gin.H{"active": active}},
	)
	return err
}

// SetJudgesActive sets the active field of multiple judges
func SetJudgesActive(db *mongo.Database, ids []primitive.ObjectID, active bool) error {
	_, err := db.Collection("judges").UpdateMany(
		context.Background(),
		gin.H{"_id": gin.H{"$in": ids}},
		gin.H{"$set": gin.H{"active": active}},
	)
	return err
}

// UpdateJudgeBasicInfo updates the basic info of a judge (name, email, notes)
func UpdateJudgeBasicInfo(db *mongo.Database, judgeId *primitive.ObjectID, addRequest *models.AddJudgeRequest) error {
	ctx := context.Background()
	current, err := FindJudge(db, ctx, *judgeId)
	if err != nil {
		return err
	}
	if current == nil {
		return errors.New("judge not found")
	}
	fields := gin.H{"name": addRequest.Name, "email": addRequest.Email, "notes": addRequest.Notes}
	if current.Email != addRequest.Email {
		fields["invitation_status"] = "not_sent"
		fields["invitation_attempt"] = primitive.NilObjectID
		fields["invitation_updated_at"] = primitive.NewDateTimeFromTime(time.Now())
	}
	_, err = db.Collection("judges").UpdateOne(ctx, gin.H{"_id": judgeId}, gin.H{"$set": fields})
	return err
}

// UpdateJudgeRanking updates the judge's ranking array
func UpdateJudgeRanking(db *mongo.Database, ctx context.Context, id primitive.ObjectID, rankings []primitive.ObjectID, rankingsAgg []models.AggRanking) error {
	_, err := db.Collection("judges").UpdateOne(
		ctx,
		gin.H{"_id": id},
		gin.H{"$set": gin.H{"rankings": rankings, "rankings_agg": rankingsAgg}},
	)
	return err
}

// UpdateJudgeSeenProjects updates the seen projects of a judge
func UpdateJudgeSeenProjects(db *mongo.Database, ctx context.Context, judge *models.Judge) error {
	_, err := db.Collection("judges").UpdateOne(ctx, gin.H{"_id": judge.Id}, gin.H{"$set": gin.H{"seen_projects": judge.SeenProjects}})
	return err
}

// UpdateJudgeNotes updates the notes of a single seen project for a judge
func UpdateJudgeNotes(db *mongo.Database, ctx context.Context, judgeId *primitive.ObjectID, projIndex int, notes string) error {
	notesKey := "seen_projects." + strconv.Itoa(projIndex) + ".notes"
	_, err := db.Collection("judges").UpdateOne(ctx, gin.H{"_id": judgeId}, gin.H{"$set": gin.H{notesKey: notes}})
	return err
}

// UpdateJudgeStars updates the star value of a single seen project for a judge
func UpdateJudgeStars(db *mongo.Database, ctx context.Context, judgeId primitive.ObjectID, projIndex int, starred bool) error {
	starredKey := "seen_projects." + strconv.Itoa(projIndex) + ".starred"
	_, err := db.Collection("judges").UpdateOne(ctx, gin.H{"_id": judgeId}, gin.H{"$set": gin.H{starredKey: starred}})
	return err

}

// Reset list of projects that judge skipped due to busy
func ResetBusyProjectListForJudge(db *mongo.Database, ctx context.Context, judge *models.Judge) error {
	_, err := db.Collection("flags").DeleteMany(ctx, gin.H{
		"judge_id": judge.Id,
		"reason":   "busy",
	})
	if err != nil {
		return err
	}
	return nil
}

// countByGroup returns the number of documents in each group (0 to numGroups - 1) of a collection
// that match the filter. Documents in groups outside of that range are ignored.
func countByGroup(db *mongo.Database, ctx context.Context, collection string, match gin.H, numGroups int64) ([]int64, error) {
	cursor, err := db.Collection(collection).Aggregate(ctx, []gin.H{
		{"$match": match},
		{"$group": gin.H{
			"_id":   "$group",
			"count": gin.H{"$sum": 1},
		}},
	})
	if err != nil {
		return nil, err
	}

	defer cursor.Close(ctx)
	counts := make([]int64, numGroups)
	for cursor.Next(ctx) {
		var result struct {
			Group int64 `bson:"_id"`
			Count int64 `bson:"count"`
		}
		err := cursor.Decode(&result)
		if err != nil {
			return nil, err
		}
		if result.Group >= 0 && result.Group < numGroups {
			counts[result.Group] = result.Count
		}
	}

	return counts, cursor.Err()
}

// getJudgeGroupWeights returns how heavily each group should be weighted when assigning judges
func getJudgeGroupWeights(db *mongo.Database, ctx context.Context, options *models.Options) ([]float64, error) {
	var projectCounts []int64
	if options.ProportionalJudgeGroups {
		var err error
		projectCounts, err = countByGroup(db, ctx, "projects", gin.H{"active": true}, options.NumGroups)
		if err != nil {
			return nil, err
		}
	}

	return util.JudgeGroupWeights(options.NumGroups, options.ProportionalJudgeGroups, projectCounts, options.GroupSizes), nil
}

// GetMinJudgeGroup returns the group a single new judge should be assigned to
func GetMinJudgeGroup(db *mongo.Database, ctx context.Context, track string) (int64, error) {
	groups, err := getNextNJudgeGroupsForTrack(db, ctx, 1, false, track)
	if err != nil {
		return -1, err
	}
	return groups[0], nil
}

// GetNextNJudgeGroups returns the next n groups to assign new judges to.
// The groups are chosen based on the current number of judges in each group,
// with the goal to balance the number of judges in each group (in proportion to
// the number of projects in each group if proportional judge groups are enabled).
// If reset is true, the current judge counts are ignored.
func GetNextNJudgeGroups(db *mongo.Database, ctx context.Context, n int, reset bool) ([]int64, error) {
	return getNextNJudgeGroupsForTrack(db, ctx, n, reset, "")
}

// Each track has its own staffing counts and project weights; general judging
// retains its existing allocator. No group moves occur during this calculation.
func getNextNJudgeGroupsForTrack(db *mongo.Database, ctx context.Context, n int, reset bool, track string) ([]int64, error) {
	// Get options
	options, err := GetOptions(db, ctx)
	if err != nil {
		return nil, err
	}
	if options.NumGroups < 1 || (track != "" && !options.MultiGroup) {
		return make([]int64, n), nil
	}

	// Get the current number of judges in each group
	judgeCounts := make([]int64, options.NumGroups)
	if !reset {
		judgeCounts, err = countByGroup(db, ctx, "judges", gin.H{"active": true, "track": track}, options.NumGroups)
		if err != nil {
			return nil, err
		}
	}

	if track != "" {
		projects, err := countByGroup(db, ctx, "projects", gin.H{
			"active": true, "challenge_list": track,
		}, options.NumGroups)
		if err != nil {
			return nil, err
		}
		weights := util.TrackJudgeGroupWeights(options.NumGroups, options.ProportionalJudgeGroups, projects, options.GroupSizes)
		return util.AllocateTrackGroups(judgeCounts, weights, n), nil
	}

	weights, err := getJudgeGroupWeights(db, ctx, options)
	if err != nil {
		return nil, err
	}

	return util.AllocateGroups(judgeCounts, weights, n), nil
}

// AssignJudgeGroups assigns a batch independently per track, preserving input order.
// Used by CSV import and by pre-judging reassignment. With reset, existing counts
// are ignored separately for general judging and for each track.
func AssignJudgeGroups(db *mongo.Database, ctx context.Context, judges []*models.Judge, reset bool) error {
	byTrack := map[string][]*models.Judge{}
	tracks := []string{}
	for _, judge := range judges {
		// Hidden track accounts do not provide staffing coverage. Leave their
		// existing group alone, matching the active-only registration counts.
		if judge.Track != "" && !judge.Active {
			continue
		}
		if _, exists := byTrack[judge.Track]; !exists {
			tracks = append(tracks, judge.Track)
		}
		byTrack[judge.Track] = append(byTrack[judge.Track], judge)
	}
	for _, track := range tracks {
		batch := byTrack[track]
		groups, err := getNextNJudgeGroupsForTrack(db, ctx, len(batch), reset, track)
		if err != nil {
			return err
		}
		for i, judge := range batch {
			judge.Group = groups[i]
			judge.GroupSeen = 0
		}
	}
	return nil
}

// PutJudgesInGroups reassigns general and track judges independently.
func PutJudgesInGroups(db *mongo.Database) error {
	return WithTransaction(db, func(sc mongo.SessionContext) error {
		judges, err := FindAllJudges(db, sc)
		if err != nil {
			return err
		}
		if err := AssignJudgeGroups(db, sc, judges, true); err != nil {
			return err
		}
		return UpdateJudgeGroups(db, sc, judges)
	})
}

// SetJudgeGroup sets the group of a judge
func SetJudgeGroup(db *mongo.Database, ctx context.Context, judgeId *primitive.ObjectID, group int64) error {
	_, err := db.Collection("judges").UpdateOne(ctx, gin.H{"_id": judgeId}, gin.H{"$set": gin.H{"group": group, "group_seen": 0}})
	return err
}

// SetJudgesGroup sets the group of multiple judges
func SetJudgesGroup(db *mongo.Database, ctx context.Context, judgeIds []primitive.ObjectID, group int64) error {
	_, err := db.Collection("judges").UpdateMany(ctx, gin.H{"_id": gin.H{"$in": judgeIds}}, gin.H{"$set": gin.H{"group": group, "group_seen": 0}})
	return err
}

// UpdateSeenProjectNumber will change the table number for all instances of a seen project
func UpdateSeenProjectNumber(db *mongo.Database, ctx context.Context, projectId *primitive.ObjectID, newLocation int64) error {
	_, err := db.Collection("judges").UpdateMany(
		ctx,
		gin.H{"seen_projects.project_id": projectId},
		gin.H{"$set": gin.H{"seen_projects.$[elem].location": newLocation}},
		options.Update().SetArrayFilters(options.ArrayFilters{
			Filters: []any{gin.H{"elem.project_id": projectId}},
		}),
	)
	return err
}

// DeleteSeenProject will delete all instances of a given project from both judge seen and judge ranking arrays
// This will also decrement the seen count for those judges by 1
func DeleteSeenProject(db *mongo.Database, ctx context.Context, projectId *primitive.ObjectID) error {
	_, err := db.Collection("judges").UpdateMany(
		ctx,
		gin.H{"seen_projects.project_id": projectId},
		gin.H{"$pull": gin.H{"seen_projects": gin.H{"project_id": projectId}, "rankings": projectId}, "$inc": gin.H{"seen": -1}},
	)
	return err
}

// IncrementTrackSeen increments the track_seen count for a project in a specific track
func IncrementTrackSeen(db *mongo.Database, ctx context.Context, projectId primitive.ObjectID, track string) error {
	project, err := FindProject(db, ctx, &projectId)
	if err != nil {
		return errors.New("error finding project: " + err.Error())
	}
	if project.TrackSeen == nil {
		project.TrackSeen = make(map[string]int64)
	}
	project.TrackSeen[track] += 1
	_, err = db.Collection("projects").UpdateOne(
		ctx,
		gin.H{"_id": projectId},
		gin.H{"$set": gin.H{"track_seen": project.TrackSeen, "last_activity": util.Now()}},
	)
	return err
}
