package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	//	"os"
	"math/rand"
	"sort"
	"strconv"
	"time"
)

type Stories struct {
	StoryIDs []int
}

/*
Field 	Description
id 	The item's unique id.
deleted 	true if the item is deleted.
type 	The type of item. One of "job", "story", "comment", "poll", or "pollopt".
by 	The username of the item's author.
time 	Creation date of the item, in Unix Time.
text 	The comment, story or poll text. HTML.
dead 	true if the item is dead.
parent 	The comment's parent: either another comment or the relevant story.
poll 	The pollopt's associated poll.
kids 	The ids of the item's comments, in ranked display order.
url 	The URL of the story.
score 	The story's score, or the votes for a pollopt.
title 	The title of the story, poll or job. HTML.
parts 	A list of related pollopts, in display order.
descendants 	In the case of stories or polls, the total comment count.
*/

type PostType string

var client http.Client

const (
	Job     PostType = "job"
	Story            = "story"
	Comment          = "comment"
	Poll             = "poll"
	PollOpt          = "pollopt"
)

type Item struct {
	Id          int
	Deleted     bool
	Type        PostType
	Time        int64
	Text        string
	Dead        bool
	Parent      int
	Poll        int
	Kids        []int
	Url         string
	Score       int
	Title       string
	Parts       []int
	Descendants int
	HN_Url      string
}

type HNPageData struct {
	PageTitle string
	Items     []Item
}

func isFromLastDay(story Item) bool {
	// Get the current time
	currentTime := time.Now()

	// Convert current time to Unix timestamp
	currentTimestamp := currentTime.Unix()

	// Calculate the timestamp for the same time on the previous day
	oneDayAgoTimestamp := currentTimestamp - 24*60*60 // 24 hours * 60 minutes/hour * 60 seconds/minute
	if story.Time >= oneDayAgoTimestamp && story.Time <= currentTimestamp {
		return true
	} else {
		return false
	}
}

func main() {
	// to change the flags on the default logger
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	client = http.Client{
		Timeout: time.Second * 10,
	}

	start := time.Now()
	stories := fetchStories()

	sort.Slice(stories, func(i, j int) bool {
		return stories[i] > stories[j]
	})

	fmt.Printf("Fetching story ids took %s\n", time.Since(start))

	storyList := getStoryItemInfo(&stories)

	fmt.Printf("Fetched %d stories\n", len(storyList))

	duration := time.Since(start)

	storyList = sortStoriesAndReturnTopN(storyList, 50)

	fmt.Printf("Fetching stories took %s\n", duration)

	updateChannel := make(chan []Item)
	go fetchStoriesPeriodically(updateChannel)

	go func() {
		for newStories := range updateChannel {
			// Update the global storyList variable with new stories.
			// You need to ensure that updates to storyList are safe and do not cause race conditions.
			// You might need another mechanism for safely updating storyList, like a mutex or another channel.
			storyList = updateStories(newStories, storyList)
		}
	}()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	tmpl := template.Must(template.ParseFiles("layout.html"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// Here so it can live update on refresh
		start := time.Now()
		data := HNPageData{
			PageTitle: "Test HN Client",
			Items:     storyList,
		}

		tmpl.Execute(w, data)
		duration := time.Since(start)
		fmt.Println(duration)
	})

	fmt.Printf("Starting to serve\n")

	log.Fatal(http.ListenAndServe(":9060", nil))

}

func updateStories(newStories, oldStories []Item) []Item {
	// This function should handle merging, sorting, and filtering of stories.
	// For simplicity, let's assume you just replace old stories with new ones.
	// In a real application, you'd probably want to merge and sort the lists in some way.
	return newStories
}

func fetchStoriesPeriodically(updateChannel chan []Item) {
	ticker := time.NewTicker(20 * time.Second) // adjust the interval as necessary
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Fetching new stories")
			newStories := fetchStories()
			updateChannel <- sortStoriesAndReturnTopN(getStoryItemInfo(&newStories), 50)
		}
	}
}

func sortStoriesAndReturnTopN(storyList []Item, top_n int) []Item {
	// order the list by descending score
	sort.Slice(storyList, func(i, j int) bool {
		return storyList[i].Score > storyList[j].Score
	})
	storyList = storyList[0:top_n]
	return storyList
}

func getNewStoryIds(stories []int) []int {
	newStories := fetchStories()

	sort.Slice(newStories, func(i, j int) bool {
		return stories[i] > stories[j]
	})

	var unseenStories []int
	var found_in_list bool
	for _, id := range newStories {
		found_in_list = false
		for _, val := range stories {
			if val == id {
				found_in_list = true
				break
			}
		}
		if !found_in_list {
			unseenStories = append(unseenStories, id)
		}

	}
	return unseenStories
}

func getStoryItemInfo(stories *[]int) []Item {
	ch := make(chan Item)
	var storycount = 0
	for _, id := range *stories {
		storycount += 1
		go GetStoryInfo(ch, id)
	}

	var storyList []Item
	for i := 0; i < storycount; i++ {
		story := <-ch
		if isFromLastDay(story) {
			storyList = append(storyList, story)
		}
	}
	return storyList
}

func fetchStories() []int {
	url := "https://hacker-news.firebaseio.com/v0/topstories.json?print=pretty"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", "John")

	res, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var storyids []int
	jsonErr := json.Unmarshal(body, &storyids)

	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	return storyids
}

func GetStoryInfo(ch chan Item, id int) {
	//fmt.Printf("Fetching story %d\n", id)
	randtime := time.Duration(rand.Intn(5))
	time.Sleep(randtime * time.Second)
	url := "https://hacker-news.firebaseio.com/v0/item/" + strconv.Itoa(id) + ".json?print=pretty"

	//fmt.Println(url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", "John")

	res, getErr := client.Do(req)

	if getErr != nil {
		for getErr != nil {
			res, getErr = client.Do(req)
			// wait a random amount of time to
			// retry the request
			fmt.Println("Trying to get story", id)
			randtime := time.Duration(rand.Intn(5))
			time.Sleep(randtime * time.Second)
		}
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var story Item
	jsonErr := json.Unmarshal(body, &story)

	if jsonErr != nil {
		log.Fatal(jsonErr)
	}
	story.HN_Url = "https://news.ycombinator.com/item?id=" + strconv.Itoa(story.Id)
	ch <- story

}
