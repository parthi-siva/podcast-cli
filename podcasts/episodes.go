package podcasts

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Episode struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Podcast     *Podcast  `json:"podcast"`
	Description string    `json:"description"`
	audioURL    string    `json:"audio-url"`
	ReleaseDate time.Time `json:"release-date"`
}

func (e *Episode) String() string {
	return fmt.Sprintf("title : %s, url: %s\n", e.Title, e.URL)
}

const episodesPerPage = 20

func (podcast *Podcast) Episodes(index int, count int) []*Episode {
	if podcast.episodes != nil && podcast.episodeIndex == index && podcast.episodesCount == count {
		return podcast.episodes
	}
	startPage := index/episodesPerPage + 1
	pagesToFetch := int(math.Ceil(float64(count+index%episodesPerPage) / episodesPerPage))

	var wg sync.WaitGroup
	episodes := make([]*Episode, 0)
	for i := 0; i < pagesToFetch; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			episodesPerPage, err := podcast.scrapEpisodesByPodcast(fmt.Sprintf("%s/-episodes?page=%d", podcast.URL, startPage+index))
			if err != nil {
				fmt.Println(err)
			} else {
				episodes = append(episodes, episodesPerPage...)
			}
		}(i)
	}
	wg.Wait()
	podcast.episodeIndex = index
	podcast.episodesCount = count
	// TODO: sort by release date
	if len(episodes) < count {
		sort.Sort(ByDate(episodes))
		podcast.episodes = episodes
	} else {
		sort.Sort(ByDate(episodes[:count]))
		podcast.episodes = episodes[:count]
	}
	return podcast.episodes
}

func (podcast *Podcast) scrapEpisodesByPodcast(URL string) ([]*Episode, error) {
	response, err := http.Get(URL)
	if err != nil || response.StatusCode != 200 {
		return nil, fmt.Errorf("Invalid podcast URL")
	}
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to get episodes list")
	}
	episodes := make([]*Episode, 0)
	doc.Find(".episode").Each(func(i int, s *goquery.Selection) {
		title := s.Find(".header > .title > a").Text()
		url, success := s.Find(".header > .title > a").Attr("href")
		if !success {
			return
		}
		description := strings.Trim(s.Find(".description").Text(), " \n")
		releaseDateString := strings.Trim(s.Find(".header > .released").Text(), " \n")
		releaseDate := parseReleaseDate(releaseDateString)
		episodes = append(episodes, &Episode{
			URL: "https://gpodder.net" + url, Title: title, Description: description, ReleaseDate: releaseDate, Podcast: podcast,
		})
	})

	return episodes, nil
}

func (e *Episode) AudioURL() (string, error) {
	if e.audioURL != "" {
		fmt.Println("hey there")
		return e.audioURL, nil
	}
	response, err := http.Get(e.URL)
	if err != nil || response.StatusCode != 200 {
		fmt.Println(err)
		return "", fmt.Errorf("Invalid episode URL")
	}

	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return "", err
	}

	link, success := doc.Find(".description > a:nth-child(2)").Attr("href")
	if !success {
		return "", fmt.Errorf("Failed to find audi link")
	}
	e.audioURL = link
	return link, nil
}

func parseReleaseDate(date string) (releaseDate time.Time) {
	formats := []string{
		"Jan. 2, 2006", "January 2, 2006",
	}
	date = strings.Replace(date, "Sept.", "September", -1)
	for _, format := range formats {
		releaseDate, err := time.Parse(format, date) //Sept. 30, 2019
		if err == nil {
			return releaseDate
		}
	}
	return releaseDate
}

func SearchEpisode(episodes []*Episode, query string) []*Episode {
	episodesMatched := make([]*Episode, 0)
	queryRegex := fmt.Sprintf(`(?i)%s`, query)
	for _, episode := range episodes {
		tb, _ := regexp.Match(queryRegex, []byte(episode.Title))
		db, _ := regexp.Match(queryRegex, []byte(episode.Description))
		if tb || db {
			episodesMatched = append(episodesMatched, episode)
		}
	}
	sort.Sort(ByDate(episodesMatched))
	return episodesMatched
}

type ByDate []*Episode

func (a ByDate) Len() int           { return len(a) }
func (a ByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByDate) Less(i, j int) bool { return a[i].ReleaseDate.After(a[j].ReleaseDate) }
