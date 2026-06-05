package parkrun

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const DefaultBaseURL = "https://www.parkrun.com.au"

var httpClient = &http.Client{Timeout: 20 * time.Second}

func Lookup(ctx context.Context, baseURL string, athleteID int64, date string) (string, bool, error) {
	url := fmt.Sprintf("%s/parkrunner/%d/all/", strings.TrimRight(baseURL, "/"), athleteID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("parkrun returned status %d", resp.StatusCode)
	}

	return parse(resp.Body, date)
}

func parse(body io.Reader, date string) (string, bool, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return "", false, err
	}

	var result string
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
			for _, cell := range cells {
				if cell == date {
					if event == "" && len(cells) > 0 {
						event = cells[0]
					}
					result = normalize(event)
					found = true
					return
				}
			}
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
