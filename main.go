// github trend等の 時間をおいてかぶる
// github awesomelist dotfile更新ウォッチ
// change favicon
// white list black list
// rssのブランチを分ける
package main

import (
	"context"
	"encoding/json"
	"html"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

const (
	TITLE = "test Aggregated Feed"
	LINK  = "https://a.example.com"
)

type FeedInfo struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Group    string `json:"group"`
	Priority int    `json:"priority"`
}

type Items []*gofeed.Item

func (a Items) Len() int           { return len(a) }
func (a Items) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Items) Less(i, j int) bool { return !(*a[i].PublishedParsed).Before(*a[j].PublishedParsed) }

func main() {
	parentCtx := context.Background()

	// Read feeds.json
	file, err := ioutil.ReadFile("feeds.json")
	if err != nil {
		log.Fatalf("failed to read feeds.json: %s", err)
	}

	var feedsInfo []FeedInfo
	err = json.Unmarshal(file, &feedsInfo)
	if err != nil {
		log.Fatalf("failed to unmarshal JSON: %s", err)
	}

	// fetch latest info from feeds
	groupedItems := make(map[string]Items)
	seenUrls := make(map[string]struct{})
	// Sort by Priority in descending order
	sort.Slice(feedsInfo, func(i, j int) bool {
		return feedsInfo[i].Priority > feedsInfo[j].Priority
	})

	for _, feedInfo := range feedsInfo {
		log.Printf("checking: %s @ %s\n", feedInfo.Title, feedInfo.URL)
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Second)
		fp := gofeed.NewParser()
		feed, err := fp.ParseURLWithContext(feedInfo.URL, ctx)
		if err != nil {
			log.Fatalf("failed to parse feed (%s): %s", feedInfo.URL, err)
		}
		cancel()

		for i := range feed.Items {
			if _, exists := seenUrls[feed.Items[i].Link]; !exists {
				feed.Items[i].Title = strconv.Itoa(feedInfo.Priority) + ":" + feedInfo.Title + ": " + feed.Items[i].Title
				groupedItems[feedInfo.Group] = append(groupedItems[feedInfo.Group], feed.Items[i])
				seenUrls[feed.Items[i].Link] = struct{}{} // Mark this URL as seen
			}
		}
	}

	// create new feeds for each group and write to files
	for group, items := range groupedItems {
		feed := &feeds.Feed{
			Title:       TITLE + " - " + group,
			Subtitle:    group,
			Author:      &feeds.Author{Name: "triggerwaffle", Email: "johndoe@example.com"},
			Description: "rss of " + group,
			Link:        &feeds.Link{Href: LINK},
			Updated:     time.Now(),
			Copyright:   "triggerwaffle",
		}
		for i := range items {
			feed.Items = append(feed.Items, &feeds.Item{
				Title:       items[i].Title,
				Link:        &feeds.Link{Href: items[i].Link},
				Description: "",
				Created:     *items[i].PublishedParsed,
				Id:          items[i].GUID,
			})
		}

		data, err := feed.ToAtom()
		if err != nil {
			log.Fatalf("failed to generate atom feed for group %s: %s", group, err)
		}

		data = strings.Replace(data, `<?xml version="1.0" encoding="UTF-8"?>`, "", 1)

		filename := group + ".xml"
		f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			log.Fatalf("failed to open file %s: %s", filename, err)
		}
		if _, err := f.Write([]byte(html.UnescapeString(data))); err != nil {
			log.Fatalf("failed to write to file %s: %s", filename, err)
		}
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}

}
