package funcs

import (
	"bytes"
	"reflect"
	"server/models"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
)

func openResults(t *testing.T, projects []*models.Project, options *models.Options) *excelize.File {
	t.Helper()
	data, err := CreateFinalResultsWorkbook(projects, options, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { book.Close() })
	return book
}

func cell(t *testing.T, book *excelize.File, sheet, address, expected string) {
	t.Helper()
	value, err := book.GetCellValue(sheet, address)
	if err != nil || value != expected {
		t.Fatalf("%s!%s: got %q, %v; want %q", sheet, address, value, err, expected)
	}
}

func TestResultsStandingsAndTrackIsolation(t *testing.T) {
	options := models.NewOptions()
	options.Tracks = []string{"Hardware", "Other", "Empty"}
	options.MultiGroup = true
	options.GroupNames = []string{"Grand Ballroom", "Rion Ballroom"}
	a := models.NewProject("Alpha", 2, 0, "", "https://devpost.com/a", "", "", []string{"Hardware", "Other"})
	b := models.NewProject("Beta", 1, 1, "", "https://devpost.com/b", "", "", []string{"Hardware"})
	c := models.NewProject("Gamma", 3, 0, "", "", "", "", []string{"Other"})
	a.Score, b.Score, c.Score = 5, 5, -2
	a.Seen, b.Seen, c.Seen = 3, 2, 1
	a.TrackStars = map[string]int64{"Hardware": 1, "Other": 99}
	b.TrackStars = map[string]int64{"Hardware": 2}
	a.TrackSeen = map[string]int64{"Hardware": 4, "Other": 88}
	b.TrackSeen = map[string]int64{"Hardware": 2}
	c.TrackStars, c.TrackSeen = nil, nil
	b.Active = false
	projects := []*models.Project{a, c, b}
	book := openResults(t, projects, options)
	if !reflect.DeepEqual(book.GetSheetList(), []string{"Overall", "Hardware", "Other", "Empty"}) {
		t.Fatal(book.GetSheetList())
	}
	cell(t, book, "Overall", "C6", "Beta")
	cell(t, book, "Overall", "A6", "1")
	cell(t, book, "Overall", "A7", "1")
	cell(t, book, "Overall", "A8", "3")
	cell(t, book, "Overall", "D8", "-2")
	cell(t, book, "Overall", "E6", "2")
	cell(t, book, "Overall", "F6", "2")
	cell(t, book, "Overall", "G6", "Rion Ballroom")
	cell(t, book, "Overall", "H6", "Hidden")
	cell(t, book, "Overall", "I7", "https://devpost.com/a")
	cell(t, book, "Overall", "B2", "2026-10-18T12:00:00Z")
	cell(t, book, "Hardware", "D5", "Stars")
	cell(t, book, "Hardware", "C6", "Beta")
	cell(t, book, "Hardware", "D6", "2")
	cell(t, book, "Hardware", "D7", "1")
	cell(t, book, "Hardware", "E7", "4")
	cell(t, book, "Other", "D6", "99")
	cell(t, book, "Other", "D7", "0")
	cell(t, book, "Other", "E7", "0")
	cell(t, book, "Empty", "A6", "")
	if projects[0] != a || projects[1] != c || a.TrackStars["Other"] != 99 {
		t.Fatal("export mutated input")
	}
}

func TestResultsNamesEmptyWorkbookAndLiteralText(t *testing.T) {
	long := strings.Repeat("Long category ", 5)
	options := models.NewOptions()
	options.Tracks = []string{"Overall", "overall", "A/B", "A:B", long, long + "x", "'Quoted'", "History", "[]:*?/\\", strings.Repeat("🎉", 25), "A/B"}
	book := openResults(t, nil, options)
	names := book.GetSheetList()
	if len(names) != 11 {
		t.Fatalf("got %d tabs: %v", len(names), names)
	}
	seen := map[string]bool{}
	for _, name := range names {
		if len(utf16.Encode([]rune(name))) > 31 || strings.ContainsAny(name, "[]:*?/\\") || seen[strings.ToLower(name)] {
			t.Fatalf("invalid/duplicate tab: %q", name)
		}
		seen[strings.ToLower(name)] = true
	}
	cell(t, book, names[5], "A1", long)
	cell(t, book, "Overall", "A5", "Place")
	p := models.NewProject("=HYPERLINK(\"https://example.com\")", 1, 0, "", "+text", "", "", nil)
	literal := openResults(t, []*models.Project{p}, models.NewOptions())
	cell(t, literal, "Overall", "C6", p.Name)
	cell(t, literal, "Overall", "I6", p.Url)
	formula, err := literal.GetCellFormula("Overall", "C6")
	if err != nil || formula != "" {
		t.Fatal("project text became a formula", formula, err)
	}
	cell(t, literal, "Overall", "F6", "")
	cell(t, literal, "Overall", "G6", "")
}
