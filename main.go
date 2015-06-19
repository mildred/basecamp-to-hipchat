package main

import (
	"encoding/json"
	"fmt"
	"github.com/andybons/hipchat"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"flag"
)

type APIClient struct {
	http     http.Client
	Username string
	Password string
}

type (
	Account struct {
		Id      int    `json:"id"`
		Name    string `json:"name"`
		Href    string `json:"href"`
		Product string `json:"product"`
	}

	Event struct {
		Id        int          `json:"id"`
		Action    string       `json:"action"`
		Summary   string       `json:"summary"`
		CreatedAt time.Time    `json:"created_at"`
		UpdatedAt time.Time    `json:"updated_at"`
		Bucket    EventBucket  `json:"bucket"`
		HTMLUrl   string       `json:"html_url"`
		Excerpt   string       `json:"excerpt"`
		Creator   EventCreator `json:"creator"`
	}

	EventBucket struct {
		Name string `json:"name"`
	}

	EventCreator struct {
		Name string `json:"name"`
	}

	Person struct {
		Id    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email_address"`
		Admin bool   `json:"admin"`
	}

	Project struct {
		Id          int    `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Archived    bool   `json:"archived"`
		Starred     bool   `json:"starred"`
	}

	Todo struct {
		Id      int    `json:"id"`
		Content string `json:"content"`
		DueAt   string `json:"due_at"`
	}

	TodoList struct {
		Id             int    `json:"id"`
		Name           string `json:"name"`
		Description    string `json:"description"`
		Completed      bool   `json:"completed"`
		CompletedCount int    `json:"completed_count"`
		RemainingCount int    `json:"remaining_count"`
		ProjectId      int    `json:"project_id"`

		Bucket struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		}

		Todos struct {
			Remaining []*Todo `json:"remaining"`
			Completed []*Todo `json:"completed"`
		}
	}
)

func (api *APIClient) newRequest(account int, method, path string) (*http.Request, error) {
	url := fmt.Sprintf("https://basecamp.com/%d/api/v1%s", account, path)
	//log.Println(url)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(api.Username, api.Password)
	req.Header.Set("User-Agent", "basecamp-to-hipchat (shanti+basecamp@sogilis.com)")
	return req, nil
}

func (api *APIClient) projects(account int) ([]*Project, error) {
	req, err := api.newRequest(account, "GET", "/projects.json")
	if err != nil {
		return nil, err
	}

	res, err := api.http.Do(req)
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var result []*Project
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (api *APIClient) allEventsSincePage(account int, since time.Time, page int) ([]*Event, error) {
	vals := url.Values{}
	vals.Add("page", fmt.Sprintf("%d", page))
	vals.Add("since", since.Format(time.RFC3339))

	req, err := api.newRequest(account, "GET", "/events.json?"+vals.Encode())
	if err != nil {
		return nil, err
	}

	res, err := api.http.Do(req)
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var result []*Event
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}

	//log.Printf("events.json since %v page %d: %d events", since, page, len(result))

	return result, nil
}

func (api *APIClient) allEventsSince(account int, since time.Time) ([]*Event, error) {
	var result []*Event
	page := 1

	lastResult, err := api.allEventsSincePage(account, since, page)
	if err != nil {
		return result, err
	}

	for len(lastResult) == 50 {
		for _, ev := range lastResult {
			result = append(result, ev)
		}
		page = page + 1

		lastResult, err = api.allEventsSincePage(account, since, page)
		if err != nil {
			return result, err
		}
	}

	for _, ev := range lastResult {
		result = append(result, ev)
	}

	return result, nil
}

func (api *APIClient) monitorEvents(account int, sleepTime time.Duration, since time.Time) <-chan interface{} {
	c := make(chan interface{})
	go func() {
		for {
			time.Sleep(sleepTime)
			events, err := api.allEventsSince(account, since)
			if err != nil {
				c <- err
				//log.Println(err)
				continue
			}
			//since = time.Now()
			for _, ev := range events {
				//log.Println(ev)
				c <- ev
				if ev.CreatedAt.After(since) {
					since = ev.CreatedAt
				}
			}
		}
	}()
	return c
}

func getRoom(basecampProject string, rooms []hipchat.Room) *hipchat.Room {
	var defaultRoom hipchat.Room
	hasDefault := false
	for _, room := range rooms {
		if basecampProject == room.Name || (room.Topic != "" && strings.Contains(room.Topic, basecampProject)) {
			//log.Printf("Project %s: Choose room %s (topic: %s)", basecampProject, room.Name, room.Topic);
			return &room
		} else if strings.Index(room.Topic, "Basecamp:*") >= 0 {
			defaultRoom = room
			hasDefault = true
			//log.Printf("Project %s: Choose default room %s (topic: %s) %d", basecampProject, defaultRoom.Name, defaultRoom.Topic, strings.Index(defaultRoom.Topic, "Basecamp:*"));
		}
	}
	if !hasDefault {
		return nil
	}
	return &defaultRoom
}

func run(basecampUser, basecampPass, hipchatAPIKey string) error {
	api := &APIClient{
		Username: basecampUser,
		Password: basecampPass,
	}

	hipchatClient := hipchat.NewClient(hipchatAPIKey)

	/*
	projects, err := api.projects(1788133)
	if err != nil {
		return err
	}

	rooms, err := hipchatClient.RoomList()
	if err != nil {
		return err
	}

	for _, project := range projects {
		r := getRoom(project.Name, rooms)
		log.Printf("Project %s -> room %s (%s)", project.Name, r.Name, r.Topic)
		for _, room := range rooms {
			if project.Name == room.Name || (room.Topic != "" && project.Description != "" && (strings.Contains(room.Topic, project.Name) || strings.Contains(project.Description, room.Topic))) {
				log.Printf("  room %v: %v", room.Name, room.Topic)
			} else if strings.Contains(room.Topic, "Basecamp:*") {
				log.Printf("  default room: %v", room.Name)
			}
		}
	}

	for _, room := range rooms {
		log.Printf("Room %v: %v", room.Name, room.Topic)
		for _, project := range projects {
			if project.Name == room.Name {
				log.Printf("  project %v (%v)", project.Name, project.Description)
			} else if room.Topic != "" && project.Description != "" && strings.Contains(room.Topic, project.Name) {
				log.Printf("  project %v (%v)", project.Name, project.Description)
			} else if room.Topic != "" && project.Description != "" && strings.Contains(project.Description, room.Topic) {
				log.Printf("  project %v (%v)", project.Name, project.Description)
			}
		}
	}
	*/

	var c <-chan interface{} = api.monitorEvents(1788133, 1*time.Second, time.Now())
	for val := range c {
		if ev, ok := val.(*Event); ok {
			//log.Printf("%v: %v", ev.Bucket.Name, ev.Summary)
			rooms, err := hipchatClient.RoomList()
			if err != nil {
				log.Println(err)
			} else if room := getRoom(ev.Bucket.Name, rooms); room != nil {
				req := hipchat.MessageRequest{
					RoomId:        fmt.Sprintf("%d", room.Id),
					From:          ev.Creator.Name,
					Message:       fmt.Sprintf(`<a href="%s"><strong>%s</strong></a><br/>%s`, ev.HTMLUrl, ev.Summary, ev.Excerpt),
					Color:         hipchat.ColorPurple,
					MessageFormat: hipchat.FormatHTML,
					Notify:        true,
				}
				if err := hipchatClient.PostMessage(req); err != nil {
					log.Println(err);
				} else {
					//log.Printf("Message sent to room %s", room.Name)
				}
			} else {
				log.Printf("Cannot find a room for %s", ev.Bucket.Name)
			}
		} else {
			log.Println(val)
		}
	}
	/*
		req := hipchat.MessageRequest{
			RoomId:        "Sogilis",
			From:          "basecamp",
			Message:       "Bad news: Combustible lemons failed.",
			Color:         hipchat.ColorPurple,
			MessageFormat: hipchat.FormatText,
			Notify:        true,
		}

		if err := c.PostMessage(req); err != nil {
			return err
		}
	*/
	return nil
}

func main() {
	var basecampUser  = flag.String("basecamp-user", "", "Username of special basecamp account that can access all projects")
	var basecampPass  = flag.String("basecamp-pass", "", "Password of special basecamp account that can access all projects")
	var HipchatAPIKey = flag.String("hipchat-api-key", "", "API Key for Hipchat")
	
	flag.Parse();
	
	err := run(*basecampUser, *basecampPass, *HipchatAPIKey)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
}
