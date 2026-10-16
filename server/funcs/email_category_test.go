package funcs

import (
	"html"
	"mime"
	"os"
	"strings"
	"testing"
)

func TestJudgeEmailSubjectCategory(t *testing.T) {
	for _, tc := range []struct{ track, want string }{
		{"", "SwampHacks XII — Judging: Overall Judging"},
		{"Best Integration of Hardware", "SwampHacks XII — Judging: Best Integration of Hardware"},
		{"Tender for Lawyers Sponsored By Morgan & Morgan", "SwampHacks XII — Judging: Tender for Lawyers Sponsored By Morgan & Morgan"},
	} {
		subject := judgeEmailSubject("SwampHacks XII", tc.track)
		if subject != tc.want {
			t.Fatalf("got %q, want %q", subject, tc.want)
		}
		encoded := mime.QEncoding.Encode("UTF-8", subject)
		decoded, err := new(mime.WordDecoder).DecodeHeader(encoded)
		if err != nil || decoded != subject {
			t.Fatal("SMTP subject did not round trip", decoded, err)
		}
	}
	if subject := judgeEmailSubject("Event\r\nBcc: example", "Track\nName"); strings.ContainsAny(subject, "\r\n") {
		t.Fatal("subject contains header line break")
	}
}

// Render the actual email file without invoking SMTP or SendGrid.
func TestJudgeEmailTemplateCategory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	for _, track := range []string{"", "Best Integration of Hardware", "Tender for Lawyers Sponsored By Morgan & Morgan", strings.Repeat("Long Category ", 20), "<script>example</script>"} {
		body, err := FillTemplate("Demo Judge", "https://example.com", "DEMO1234", "SwampHacks XII", track)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if !strings.Contains(text, "Your judging category: "+html.EscapeString(judgingCategory(track))) {
			t.Fatalf("missing category for %q", track)
		}
		if !strings.Contains(text, "Start Judging") || strings.Contains(text, "Start Judging —") {
			t.Fatal("button label changed")
		}
		if !strings.Contains(text, "https://example.com/judge/login?code=DEMO1234") {
			t.Fatal("login link changed")
		}
		if !strings.Contains(text, "use its separate invitation") {
			t.Fatal("missing category-specific code explanation")
		}
		if strings.Contains(text, "<script>example</script>") {
			t.Fatal("category was not HTML escaped")
		}
	}
}
