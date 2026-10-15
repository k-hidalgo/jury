package judging

import (
	"context"
	"net/url"
	"os"
	"reflect"
	"server/database"
	"server/models"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Optional local integration check. Never use Atlas or the application's database.
func TestFinalistReviewAssignmentsWithMongo(t *testing.T) {
	uri := os.Getenv("JURY_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set JURY_TEST_MONGO_URI to a disposable localhost MongoDB")
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
	db := client.Database("jury_finalist_review_test_" + primitive.NewObjectID().Hex())
	defer db.Drop(context.Background())
	set := func(collection string, docs ...interface{}) {
		t.Helper()
		if _, err := db.Collection(collection).DeleteMany(ctx, bson.M{}); err != nil {
			t.Fatal(err)
		}
		if len(docs) > 0 {
			if _, err := db.Collection(collection).InsertMany(ctx, docs); err != nil {
				t.Fatal(err)
			}
		}
	}
	project := func(name string, seen int64, priority bool, group int64) *models.Project {
		p := models.NewProject(name, seen+1, group, "", "https://example.com", "", "", []string{"Test Track"})
		p.Id = primitive.NewObjectID()
		p.Seen = seen
		p.Prioritized = priority
		return p
	}
	ordinary := project("ordinary", 1, false, 0)
	finalist := project("finalist", 2, true, 0)
	unseen := project("unseen", 0, false, 0)
	otherRoom := project("other room", 0, true, 1)
	reached := project("reached", 4, true, 0)
	hidden := project("hidden", 0, true, 0)
	hidden.Active = false
	judge := models.NewJudge("Local test", "test@example.com", "", "", 0)
	judge.Id = primitive.NewObjectID()
	settings := models.NewOptions()
	settings.FinalistReview = true
	settings.MinViews = 1
	settings.MultiGroup = true
	settings.NumGroups = 2
	settings.GroupSizes = []int64{10, 10}
	settings.FinalistTargetViews = 4
	reset := func(ps ...*models.Project) {
		t.Helper()
		set("options", settings)
		set("judges")
		set("flags")
		docs := []interface{}{}
		for _, p := range ps {
			docs = append(docs, p)
		}
		set("projects", docs...)
		judge.SeenProjects = []models.JudgedProject{}
		judge.Flagged = nil
		judge.Group = 0
		judge.Track = ""
	}
	pick := func() *models.Project {
		t.Helper()
		all, err := database.FindAllProjects(db, ctx)
		if err != nil {
			t.Fatal(err)
		}
		p, err := PickNextProject(db, ctx, judge, CreateComparisons(all, nil))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	assertPick := func(want *models.Project) {
		t.Helper()
		got := pick()
		if got == nil || got.Id != want.Id {
			t.Fatalf("got %+v, want %s", got, want.Name)
		}
	}
	t.Run("unseen first and room restriction", func(t *testing.T) { reset(ordinary, finalist, unseen, otherRoom, hidden); assertPick(unseen) })
	t.Run("configured second view before finalist", func(t *testing.T) {
		settings.MinViews = 2
		defer func() { settings.MinViews = 1 }()
		reset(ordinary, finalist, otherRoom)
		assertPick(ordinary)
		if judge.Group != 0 {
			t.Fatal("left room before local minimum coverage")
		}
	})
	t.Run("minimum met releases finalist", func(t *testing.T) {
		settings.MinViews = 2
		defer func() { settings.MinViews = 1 }()
		covered := project("covered", 2, false, 0)
		reset(covered, finalist, otherRoom)
		assertPick(finalist)
	})
	t.Run("seen under-covered project stays excluded", func(t *testing.T) {
		settings.MinViews = 2
		defer func() { settings.MinViews = 1 }()
		reset(ordinary, finalist)
		judge.SeenProjects = []models.JudgedProject{{ProjectId: ordinary.Id}}
		assertPick(finalist)
	})
	t.Run("zero minimum allows finalist before unseen", func(t *testing.T) {
		settings.MinViews = 0
		defer func() { settings.MinViews = 1 }()
		reset(unseen, finalist)
		assertPick(finalist)
	})
	t.Run("finalist overrides lowest view filter", func(t *testing.T) {
		reset(ordinary, finalist, otherRoom)
		assertPick(finalist)
		if judge.Group != 0 {
			t.Fatal("moved judge for finalist")
		}
	})
	t.Run("target reached returns ordinary", func(t *testing.T) { reset(ordinary, reached); assertPick(ordinary) })
	t.Run("already seen finalist excluded", func(t *testing.T) {
		reset(ordinary, finalist)
		judge.SeenProjects = []models.JudgedProject{{ProjectId: finalist.Id}}
		assertPick(ordinary)
	})
	t.Run("flagged finalist excluded", func(t *testing.T) {
		reset(ordinary, finalist)
		judge.Flagged = []primitive.ObjectID{finalist.Id}
		assertPick(ordinary)
	})
	t.Run("busy finalist excluded when another project free", func(t *testing.T) {
		reset(ordinary, finalist)
		busy := models.NewJudge("Busy", "busy@example.com", "", "", 0)
		busy.Current = &finalist.Id
		set("judges", busy)
		assertPick(ordinary)
	})
	t.Run("off restores original filter", func(t *testing.T) {
		settings.FinalistReview = false
		reset(ordinary, finalist)
		assertPick(ordinary)
		settings.FinalistReview = true
	})
	t.Run("no priorities matches normal candidates", func(t *testing.T) {
		reset(ordinary, unseen)
		on, err := FindAvailableItems(db, ctx, judge)
		if err != nil {
			t.Fatal(err)
		}
		settings.FinalistReview = false
		set("options", settings)
		off, err := FindAvailableItems(db, ctx, judge)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(on, off) {
			t.Fatal("normal candidates changed")
		}
		settings.FinalistReview = true
	})
	t.Run("track view cap unchanged", func(t *testing.T) {
		settings.JudgeTracks = true
		settings.Tracks = []string{"Test Track"}
		settings.TrackViews = []int64{2}
		reset(ordinary, finalist)
		judge.Track = "Test Track"
		finalist.TrackSeen["Test Track"] = 2
		set("projects", ordinary, finalist)
		assertPick(ordinary)
		settings.JudgeTracks = false
	})
	t.Run("older database defaults off and target four", func(t *testing.T) {
		reset(ordinary)
		if _, err := db.Collection("options").UpdateMany(ctx, bson.M{}, bson.M{"$unset": bson.M{"finalist_review": "", "finalist_target_views": ""}}); err != nil {
			t.Fatal(err)
		}
		got, err := database.GetOptions(db, ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got.FinalistReview || got.FinalistTargetViews != 4 {
			t.Fatalf("legacy defaults %+v", got)
		}
	})
}
