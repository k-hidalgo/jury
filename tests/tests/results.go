package tests

import (
	"archive/zip"
	"bytes"
	stdcontext "context"
	"encoding/xml"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"tests/util"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Exercise the actual admin download, including judge-derived totals rather
// than legacy project totals, track isolation, and read-only behavior.
func FinalResultsExport(c *util.Context) util.Result {
	defer resetGroupOptions(c)
	if res := setupGroups(c, []int64{2, 1}, 3, false); !res.Success {
		return res
	}
	ctx := stdcontext.Background()
	_, err := c.Db.Collection("options").UpdateOne(ctx, bson.M{}, bson.M{"$set": bson.M{"tracks": []string{"Hardware", "Empty"}, "group_names": []string{"Grand", "Rion"}}})
	if err != nil {
		return util.NewResult(false, err.Error())
	}
	defer c.Db.Collection("options").UpdateOne(ctx, bson.M{}, bson.M{"$set": bson.M{"tracks": []string{}}})
	ids := []primitive.ObjectID{}
	for i := 1; i <= 3; i++ {
		var p struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := c.Db.Collection("projects").FindOne(ctx, bson.M{"location": i}).Decode(&p); err != nil {
			return util.NewResult(false, err.Error())
		}
		ids = append(ids, p.ID)
		tracks := []string{"Hardware"}
		if i == 3 {
			tracks = []string{}
		}
		_, err := c.Db.Collection("projects").UpdateOne(ctx, bson.M{"_id": p.ID}, bson.M{"$set": bson.M{"score": 999, "track_stars": bson.M{"Hardware": 99}, "challenge_list": tracks, "seen": i, "track_seen": bson.M{"Hardware": i + 2}}})
		if err != nil {
			return util.NewResult(false, err.Error())
		}
	}
	_, err = c.Db.Collection("judges").InsertMany(ctx, []interface{}{
		bson.M{"name": "Overall Export Judge", "track": "", "token": "export-test-token", "active": true, "rankings_agg": []bson.M{{"project_id": ids[0], "score": -2}, {"project_id": ids[1], "score": 2}, {"project_id": ids[2], "score": 0}}},
		bson.M{"name": "Track Export Judge", "track": "Hardware", "seen_projects": []bson.M{{"project_id": ids[0], "starred": true}}},
	})
	if err != nil {
		return util.NewResult(false, err.Error())
	}
	for _, auth := range []string{util.DefaultAuth(), util.JudgeAuth("export-test-token")} {
		status, _ := util.GetRequestWithStatus(c.Logger, "/admin/export/final-results", auth)
		if status < 400 {
			return util.NewResult(false, "non-admin could export results")
		}
	}
	before := util.GetRequest(c.Logger, "/project/list", util.AdminAuth())
	status, body := util.GetRequestWithStatus(c.Logger, "/admin/export/final-results", util.AdminAuth())
	if status != 200 {
		return util.NewResult(false, fmt.Sprintf("export returned %d", status))
	}
	data := []byte(body)
	workbook, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return util.NewResult(false, "download is not an XLSX: "+err.Error())
	}
	var shared struct {
		Items []struct {
			Text string `xml:"t"`
		} `xml:"si"`
	}
	sheets := map[string][]byte{}
	for _, file := range workbook.File {
		reader, err := file.Open()
		if err != nil {
			return util.NewResult(false, err.Error())
		}
		b, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return util.NewResult(false, err.Error())
		}
		sheets[file.Name] = b
	}
	if err := xml.Unmarshal(sheets["xl/sharedStrings.xml"], &shared); err != nil {
		return util.NewResult(false, err.Error())
	}
	var definition struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(sheets["xl/workbook.xml"], &definition); err != nil {
		return util.NewResult(false, err.Error())
	}
	names := []string{}
	for _, sheet := range definition.Sheets {
		names = append(names, sheet.Name)
	}
	if !reflect.DeepEqual(names, []string{"Overall", "Hardware", "Empty"}) {
		return util.NewResult(false, fmt.Sprint("unexpected tabs: ", names))
	}
	for part, expected := range map[string]map[string]string{
		"xl/worksheets/sheet1.xml": {"B6": "2", "D6": "2", "D7": "0", "D8": "-2", "E6": "2", "G6": "Grand"},
		"xl/worksheets/sheet2.xml": {"B6": "1", "D6": "1", "D7": "0", "E6": "3"},
	} {
		var sheet struct {
			Cells []struct {
				Ref   string `xml:"r,attr"`
				Type  string `xml:"t,attr"`
				Value string `xml:"v"`
			} `xml:"sheetData>row>c"`
		}
		if err := xml.Unmarshal(sheets[part], &sheet); err != nil {
			return util.NewResult(false, err.Error())
		}
		for _, cell := range sheet.Cells {
			value := cell.Value
			if cell.Type == "s" {
				n, e := strconv.Atoi(value)
				if e != nil || n < 0 || n >= len(shared.Items) {
					return util.NewResult(false, "invalid string index")
				}
				value = shared.Items[n].Text
			}
			if want, ok := expected[cell.Ref]; ok {
				if want != value {
					return util.NewResult(false, fmt.Sprintf("%s %s: got %q want %q", part, cell.Ref, value, want))
				}
				delete(expected, cell.Ref)
			}
		}
		if len(expected) != 0 {
			return util.NewResult(false, fmt.Sprint("missing cells: ", expected))
		}
	}
	after := util.GetRequest(c.Logger, "/project/list", util.AdminAuth())
	if before != after {
		return util.NewResult(false, "export changed project results")
	}
	return util.ResultOk()
}
