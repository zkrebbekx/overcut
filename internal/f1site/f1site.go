// Package f1site reads the official starting grid from the public results
// pages of the Formula 1 website. The grid page appears a few hours after
// qualifying with every penalty applied, which is the earliest public,
// machine-readable record of where each car starts.
package f1site

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BaseURL is the site root. A test can point it at a local server.
var BaseURL = "https://www.formula1.com"

// Race is one entry of the season results index.
type Race struct {
	ID   int
	Slug string
}

var (
	raceLink = regexp.MustCompile(`/en/results/(\d{4})/races/(\d+)/([a-z0-9-]+)/`)
	tableRe  = regexp.MustCompile(`(?s)<table.*?</table>`)
	rowRe    = regexp.MustCompile(`(?s)<tr.*?</tr>`)
	cellRe   = regexp.MustCompile(`(?s)<t[dh][^>]*>(.*?)</t[dh]>`)
	tagRe    = regexp.MustCompile(`<[^>]+>`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

func get(client *http.Client, url string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "overcut/1 (+https://github.com/zkrebbekx/overcut)")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("f1site: get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("f1site: get %s: status %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("f1site: read %s: %w", url, err)
	}
	return string(b), nil
}

// Index returns the season's races in page order. The slug is the site's
// country or circuit slug, such as "italy" or "great-britain".
func Index(client *http.Client, season int) ([]Race, error) {
	page, err := get(client, fmt.Sprintf("%s/en/results/%d/races", BaseURL, season))
	if err != nil {
		return nil, err
	}
	return ParseIndex(page, season), nil
}

// ParseIndex extracts the race ids and slugs from the index page.
func ParseIndex(page string, season int) []Race {
	seen := map[int]bool{}
	var out []Race
	for _, m := range raceLink.FindAllStringSubmatch(page, -1) {
		if m[1] != strconv.Itoa(season) {
			continue
		}
		id, _ := strconv.Atoi(m[2])
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Race{ID: id, Slug: m[3]})
	}
	return out
}

// slugAliases maps site slugs to a word that appears in the race name of
// the classification source. Slugs without an alias match on their first
// word.
var slugAliases = map[string]string{
	"australia":           "australian",
	"china":               "chinese",
	"japan":               "japanese",
	"canada":              "canadian",
	"barcelona-catalunya": "barcelona",
	"austria":             "austrian",
	"great-britain":       "british",
	"belgium":             "belgian",
	"hungary":             "hungarian",
	"netherlands":         "dutch",
	"italy":               "italian",
	"spain":               "spanish",
	"brazil":              "brazil",
	"emilia-romagna":      "emilia",
	"saudi-arabia":        "saudi",
	"united-states":       "united states",
	"las-vegas":           "las vegas",
	"abu-dhabi":           "abu dhabi",
}

// MatchRace finds the index entry for a race name such as "Italian Grand
// Prix". It returns false when no slug matches.
func MatchRace(index []Race, raceName string) (Race, bool) {
	name := strings.ToLower(raceName)
	for _, r := range index {
		key, ok := slugAliases[r.Slug]
		if !ok {
			key = strings.SplitN(r.Slug, "-", 2)[0]
		}
		if strings.Contains(name, key) {
			return r, true
		}
	}
	return Race{}, false
}

// GridRow is one car on the starting grid.
type GridRow struct {
	Position int
	Number   int
	Code     string // three-letter driver code
	Team     string
}

// StartingGrid fetches the official starting grid for a race id.
func StartingGrid(client *http.Client, season int, race Race) ([]GridRow, error) {
	page, err := get(client, fmt.Sprintf("%s/en/results/%d/races/%d/%s/starting-grid", BaseURL, season, race.ID, race.Slug))
	if err != nil {
		return nil, err
	}
	rows := ParseGrid(page)
	if len(rows) == 0 {
		return nil, fmt.Errorf("f1site: no grid rows on the page for race %d", race.ID)
	}
	return rows, nil
}

// ParseGrid extracts the grid rows from a starting-grid page. It returns
// nil when the page has no grid table yet.
func ParseGrid(page string) []GridRow {
	table := tableRe.FindString(page)
	if table == "" {
		return nil
	}
	var out []GridRow
	for _, tr := range rowRe.FindAllString(table, -1) {
		var cells []string
		for _, c := range cellRe.FindAllStringSubmatch(tr, -1) {
			text := html.UnescapeString(tagRe.ReplaceAllString(c[1], " "))
			cells = append(cells, strings.TrimSpace(spaceRe.ReplaceAllString(text, " ")))
		}
		if len(cells) < 4 {
			continue
		}
		pos, err := strconv.Atoi(cells[0])
		if err != nil {
			continue // header row
		}
		num, _ := strconv.Atoi(cells[1])
		words := strings.Fields(cells[2])
		if len(words) == 0 {
			continue
		}
		code := words[len(words)-1]
		if len(code) != 3 || strings.ToUpper(code) != code {
			continue
		}
		out = append(out, GridRow{Position: pos, Number: num, Code: code, Team: cells[3]})
	}
	return out
}
