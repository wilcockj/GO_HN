package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
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

func main() {
	stories := fetchStories()

	ch := make(chan Item)
	var storycount = 0
	for _, id := range stories {
		storycount += 1
		go GetStoryInfo(ch, id)
	}

	var storyList []Item
	for i := 0; i < storycount; i++ {
		story := <-ch
		storyList = append(storyList, story)
	}
	fmt.Printf("Fetched %d stories\n", len(storyList))

	// order the list by descending score
	sort.Slice(storyList, func(i, j int) bool {
		return storyList[i].Score > storyList[j].Score
	})

	file, _ := json.MarshalIndent(storyList, "", " ")

	_ = os.WriteFile("test.json", file, 0644)

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		/*data := HNClientData{
			PageTitle: "HN Client",
			Todos: []Todo{
				{Title: "Task 1", Done: false},
				{Title: "Task 2", Done: true},
				{Title: "Task 3", Done: true},
			},
		}*/
        // Here so it can live update on refresh
        tmpl := template.Must(template.ParseFiles("layout.html"))
		start := time.Now()
		data := HNPageData{
			PageTitle: "Test HN Client",
            Items:     storyList[0:50],
		}

		tmpl.Execute(w, data)
		duration := time.Since(start)
		fmt.Println(duration)
	})

	fmt.Printf("Starting to serve\n")

	http.ListenAndServe(":9060", nil)
}

func fetchStories() []int {
	url := "https://hacker-news.firebaseio.com/v0/topstories.json?print=pretty"
	client := http.Client{
		Timeout: time.Second * 2,
	}

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
	url := "https://hacker-news.firebaseio.com/v0/item/" + strconv.Itoa(id) + ".json?print=pretty"

	//fmt.Println(url)
	client := http.Client{
		Timeout: time.Second * 2,
	}

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

	var story Item
	jsonErr := json.Unmarshal(body, &story)

	if jsonErr != nil {
		log.Fatal(jsonErr)
	}
	story.HN_Url = "https://news.ycombinator.com/item?id=" + strconv.Itoa(story.Id)
	ch <- story

}
