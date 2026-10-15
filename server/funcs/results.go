package funcs

import (
	"fmt"
	"server/models"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
)

// CreateFinalResultsWorkbook exports the aggregated dashboard totals without changing
// judging state. Track star standings are not deliberated winner selections.
func CreateFinalResultsWorkbook(projects []*models.Project, options *models.Options, exportedAt time.Time) ([]byte, error) {
	book := excelize.NewFile()
	defer book.Close()
	headerStyle, err := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"00A9D8"}, Pattern: 1},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	noteStyle, err := book.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	titleStyle, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 16}})
	if err != nil {
		return nil, err
	}
	if err := book.SetSheetName("Sheet1", "Overall"); err != nil {
		return nil, err
	}
	usedNames := map[string]bool{"overall": true}
	tracks := append([]string{""}, options.Tracks...)
	exportedTracks := map[string]bool{}
	for _, track := range tracks {
		if exportedTracks[track] {
			continue
		}
		exportedTracks[track] = true
		sheet, title, metric, views := "Overall", "Overall Results", "Score", "Overall Views"
		note := "Aggregated overall scores, highest first. Equal scores share a place; project number orders ties. Hidden projects are included and marked."
		if track != "" {
			sheet = resultsSheetName(track, usedNames)
			title, metric, views = track, "Stars", "Track Views"
			note = "Star standings only, not selected winners. Equal stars share a standing; project number orders ties. Hidden projects are included and marked."
			if _, err := book.NewSheet(sheet); err != nil {
				return nil, err
			}
		}
		rows := []*models.Project{}
		for _, project := range projects {
			if track == "" || contains(project.ChallengeList, track) {
				rows = append(rows, project)
			}
		}
		value := func(p *models.Project) int64 {
			if track == "" {
				return p.Score
			}
			return p.TrackStars[track]
		}
		sort.Slice(rows, func(i, j int) bool {
			a, b := rows[i], rows[j]
			if value(a) != value(b) {
				return value(a) > value(b)
			}
			if a.Location != b.Location {
				return a.Location < b.Location
			}
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			return a.Id.Hex() < b.Id.Hex()
		})
		heading := "Place"
		if track != "" {
			heading = "Star Standing"
		}
		content := [][]interface{}{
			{title},
			{"Exported at (UTC)", exportedAt.UTC().Format(time.RFC3339)},
			{note},
			{},
			{heading, "Project Number", "Project Name", metric, views, "Group Number", "Room", "Status", "Devpost URL"},
		}
		place := 0
		var previous int64
		for i, project := range rows {
			if i == 0 || value(project) != previous {
				place = i + 1
			}
			previous = value(project)
			seen := project.Seen
			if track != "" {
				seen = project.TrackSeen[track]
			}
			room, group := "", interface{}("")
			if options.MultiGroup {
				group = project.Group + 1
				room = fmt.Sprintf("Group %d", project.Group+1)
				if project.Group >= 0 && project.Group < int64(len(options.GroupNames)) && options.GroupNames[project.Group] != "" {
					room = options.GroupNames[project.Group]
				}
			}
			status := "Active"
			if !project.Active {
				status = "Hidden"
			}
			content = append(content, []interface{}{place, project.Location, project.Name, value(project), seen, group, room, status, project.Url})
		}
		for i, row := range content {
			cell, err := excelize.CoordinatesToCellName(1, i+1)
			if err != nil {
				return nil, err
			}
			// SetSheetRow stores project text as strings, never spreadsheet formulas.
			if err := book.SetSheetRow(sheet, cell, &row); err != nil {
				return nil, err
			}
		}
		for _, span := range []string{"A1:I1", "A3:I3"} {
			cells := strings.Split(span, ":")
			if err := book.MergeCell(sheet, cells[0], cells[1]); err != nil {
				return nil, err
			}
		}
		if err := book.SetCellStyle(sheet, "A1", "I1", titleStyle); err != nil {
			return nil, err
		}
		if err := book.SetRowHeight(sheet, 1, 28); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle(sheet, "A3", "I3", noteStyle); err != nil {
			return nil, err
		}
		if err := book.SetRowHeight(sheet, 3, 32); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle(sheet, "A5", "I5", headerStyle); err != nil {
			return nil, err
		}
		if err := book.SetColWidth(sheet, "A", "I", 18); err != nil {
			return nil, err
		}
		if err := book.SetColWidth(sheet, "C", "C", 45); err != nil {
			return nil, err
		}
		if err := book.SetColWidth(sheet, "G", "G", 25); err != nil {
			return nil, err
		}
		if err := book.SetColWidth(sheet, "I", "I", 55); err != nil {
			return nil, err
		}
		if err := book.SetRowHeight(sheet, 5, 32); err != nil {
			return nil, err
		}
		if err := book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 5, TopLeftCell: "A6", ActivePane: "bottomLeft"}); err != nil {
			return nil, err
		}
		if err := book.AutoFilter(sheet, fmt.Sprintf("A5:I%d", len(content)), nil); err != nil {
			return nil, err
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Excel tab labels have restricted characters, a 31 UTF-16-unit limit and
// case-insensitive uniqueness. The full category name remains in cell A1.
func resultsSheetName(track string, used map[string]bool) string {
	base := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		if strings.ContainsRune("[]:*?/\\", r) {
			return '-'
		}
		return r
	}, track)
	base = strings.Trim(strings.TrimSpace(base), "'")
	if base == "" {
		base = "Track"
	}
	if strings.EqualFold(base, "History") {
		base = "History Track"
	}
	trim := func(value string, limit int) string {
		runes, units := []rune{}, 0
		for _, r := range value {
			size := len(utf16.Encode([]rune{r}))
			if units+size > limit {
				break
			}
			runes, units = append(runes, r), units+size
		}
		return strings.TrimRight(string(runes), "'")
	}
	name := trim(base, 31)
	for n := 2; used[strings.ToLower(name)]; n++ {
		suffix := fmt.Sprintf(" (%d)", n)
		name = trim(base, 31-len(suffix)) + suffix
	}
	used[strings.ToLower(name)] = true
	return name
}
