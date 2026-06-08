package parkrun

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const DefaultBaseURL = "https://www.parkrun.com.au"

var httpClient = &http.Client{Timeout: 20 * time.Second}

type Data struct {
	Event      string
	RunNumber  int
	OverallPos int
	Time       time.Duration
	AgeGrade   float64
	PB         bool
}

func Lookup(ctx context.Context, baseURL string, athleteID int64, date string) (Data, bool, error) {
	url := fmt.Sprintf("%s/parkrunner/%d/all/", strings.TrimRight(baseURL, "/"), athleteID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Data{}, false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Data{}, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Data{}, false, fmt.Errorf("parkrun returned status %d", resp.StatusCode)
	}

	return parse(resp.Body, date)
}

func parse(body io.Reader, date string) (Data, bool, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return Data{}, false, err
	}

	var result Data
	var found bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found {
			return
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []string
			var event string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != html.ElementNode || c.Data != "td" {
					continue
				}
				cells = append(cells, strings.TrimSpace(nodeText(c)))
				if event == "" {
					if link := firstAnchorText(c); link != "" {
						event = link
					}
				}
			}

			if len(cells) < 6 || cells[1] != date {
				return
			}

			runNumber, _ := strconv.Atoi(cells[2])
			overallPos, _ := strconv.Atoi(cells[3])

			duration, _ := parseTime(cells[4])
			ageGrade := 0.0
			_, _ = fmt.Sscanf(cells[5], "%f%%", &ageGrade)

			result = Data{
				Event:      normalize(cells[0]),
				RunNumber:  runNumber,
				OverallPos: overallPos,
				Time:       duration,
				AgeGrade:   ageGrade,
				PB:         cells[6] != "",
			}
			found = true
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return result, found, nil
}

func normalize(event string) string {
	if event == "" || strings.HasSuffix(event, "parkrun") {
		return event
	}
	return event + " parkrun"
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func firstAnchorText(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "a" {
		return strings.TrimSpace(nodeText(n))
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if txt := firstAnchorText(c); txt != "" {
			return txt
		}
	}
	return ""
}

var timeRegexp = regexp.MustCompile(`^(\d+:)?(\d{2}):(\d{2})$`)

func parseTime(s string) (time.Duration, error) {
	m := timeRegexp.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("invalid time format: %q", s)
	}

	var hours int
	if m[1] != "" {
		var err error
		hours, err = strconv.Atoi(strings.TrimSuffix(m[1], ":"))
		if err != nil {
			return 0, err
		}
	}

	minutes, err := strconv.Atoi(m[2])
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.Atoi(m[3])
	if err != nil {
		return 0, err
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second, nil
}
