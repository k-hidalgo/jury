package tests

import (
	"bytes"
	stdcontext "context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"slices"
	"tests/util"

	"go.mongodb.org/mongo-driver/bson"
)

// Track A has nine projects in room 1 and two in room 2. Track B only has
// projects in room 2, despite most projects overall being in room 1.
func setupTrackGroups(c *util.Context) util.Result {
	if res := setupGroups(c, []int64{9, 2}, 11, true); !res.Success {
		return res
	}
	if _, err := c.Db.Collection("projects").UpdateMany(stdcontext.Background(), bson.M{}, bson.M{"$set": bson.M{"challenge_list": []string{"Track A"}}}); err != nil {
		return util.NewResult(false, err.Error())
	}
	if _, err := c.Db.Collection("projects").UpdateMany(stdcontext.Background(), bson.M{"group": 1}, bson.M{"$set": bson.M{"challenge_list": []string{"Track A", "Track B"}}}); err != nil {
		return util.NewResult(false, err.Error())
	}
	return util.AssertOk(util.PostRequest(c.Logger, "/admin/tracks", util.H{"tracks": []string{"Track A", "Track B"}}, util.AdminAuth()), "set test tracks")
}

func assertTrackCounts(c *util.Context, track string, want []int64) util.Result {
	var judges []struct {
		Track  string `json:"track"`
		Active bool   `json:"active"`
		Group  int64  `json:"group"`
	}
	if err := jsonUnmarshalList(util.GetRequest(c.Logger, "/judge/list", util.AdminAuth()), &judges); err != nil {
		return util.NewResult(false, err.Error())
	}
	got := make([]int64, len(want))
	for _, j := range judges {
		if j.Track == track && j.Active {
			if j.Group < 0 || int(j.Group) >= len(got) {
				return util.NewResult(false, "invalid group")
			}
			got[j.Group]++
		}
	}
	if !slices.Equal(got, want) {
		return util.NewResult(false, fmt.Sprintf("track %q: got %v, want %v", track, got, want))
	}
	return util.ResultOk()
}

func TrackQRAndManualGroupAssignment(c *util.Context) util.Result {
	defer resetGroupOptions(c)
	if res := setupTrackGroups(c); !res.Success {
		return res
	}
	for _, track := range []string{"Track A", "Track B"} {
		gen := util.PostRequest(c.Logger, "/admin/qr/"+url.PathEscape(track), nil, util.AdminAuth())
		code := util.ExtractString(gen, "qr_code")
		if code == "" {
			return util.NewResult(false, "QR generation failed: "+gen)
		}
		for i := 0; i < 2; i++ {
			res := util.PostRequest(c.Logger, "/qr/add", util.H{"name": fmt.Sprintf("QR %s %d", track, i), "email": fmt.Sprintf("qr%d@example.com", i), "track": track, "code": code, "no_send": true}, util.DefaultAuth())
			if !util.IsOk(res) {
				return util.NewResult(false, "QR add failed: "+res)
			}
		}
	}
	if res := assertTrackCounts(c, "Track A", []int64{1, 1}); !res.Success {
		return res
	}
	if res := assertTrackCounts(c, "Track B", []int64{0, 2}); !res.Success {
		return res
	}
	// Nine additional manual registrations restore the full 9:2 ratio for A.
	for i := 0; i < 9; i++ {
		res := util.PostRequest(c.Logger, "/judge/new", util.H{"name": fmt.Sprintf("Manual A %d", i), "email": fmt.Sprintf("manual%d@example.com", i), "track": "Track A", "no_send": true}, util.AdminAuth())
		if !util.IsOk(res) {
			return util.NewResult(false, "manual add failed: "+res)
		}
	}
	return assertTrackCounts(c, "Track A", []int64{9, 2})
}

func TrackCSVAndReassignment(c *util.Context) util.Result {
	defer resetGroupOptions(c)
	if res := setupTrackGroups(c); !res.Success {
		return res
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("csv", "mixed-tracks.csv")
	if err != nil {
		return util.NewResult(false, err.Error())
	}
	// Interleave tracks to verify independent batch counts and original row mapping.
	fmt.Fprint(file, "A1,a1@example.com,Track A\nB1,b1@example.com,Track B\nG1,g1@example.com,\nA2,a2@example.com,Track A\nB2,b2@example.com,Track B\nG2,g2@example.com,\n")
	writer.WriteField("noSend", "true")
	writer.Close()
	req, err := http.NewRequest("POST", os.Getenv("API_URL")+"/judge/csv", &body)
	if err != nil {
		return util.NewResult(false, err.Error())
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", util.AdminAuth())
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return util.NewResult(false, err.Error())
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return util.NewResult(false, err.Error())
	}
	if response.StatusCode != 200 || !util.IsOk(string(data)) {
		return util.NewResult(false, "CSV failed: "+string(data))
	}
	for track, want := range map[string][]int64{"Track A": {1, 1}, "Track B": {0, 2}, "": {2, 0}} {
		if res := assertTrackCounts(c, track, want); !res.Success {
			return res
		}
	}
	// An inactive account must not consume the only judge slot in a room.
	if _, err := c.Db.Collection("judges").InsertOne(stdcontext.Background(), bson.M{
		"name": "Hidden track judge", "track": "Track A", "active": false, "group": 0,
	}); err != nil {
		return util.NewResult(false, err.Error())
	}
	// Deliberately unbalance all accounts, then verify reassignment repairs each pool.
	if _, err := c.Db.Collection("judges").UpdateMany(stdcontext.Background(), bson.M{}, bson.M{"$set": bson.M{"group": 0}}); err != nil {
		return util.NewResult(false, err.Error())
	}
	if res := reassignJudgeGroups(c); !res.Success {
		return res
	}
	for track, want := range map[string][]int64{"Track A": {1, 1}, "Track B": {0, 2}, "": {2, 0}} {
		if res := assertTrackCounts(c, track, want); !res.Success {
			return res
		}
	}
	return util.ResultOk()
}
