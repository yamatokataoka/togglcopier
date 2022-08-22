package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	Token           = "Toggl Track Token"
	ApiUrl          = "https://api.track.toggl.com/api/v8"
	TimeEntriesPath = "time_entries"
	TimeZone        = "Time Zone"
)

type Client struct {
	host       string
	httpClient *http.Client
	token      string
}

func NewClient(host, token string) *Client {
	client := &http.Client{}
	return &Client{
		host:       host,
		httpClient: client,
		token:      token,
	}
}

func (c *Client) do(method, path string, params map[string]string, body []byte) (*http.Response, error) {
	baseURL := fmt.Sprintf("%s/%s", c.host, path)

	req, err := http.NewRequest(method, baseURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(c.token, "api_token")

	q := req.URL.Query()
	for key, val := range params {
		q.Set(key, val)
	}
	req.URL.RawQuery = q.Encode()

	return c.httpClient.Do(req)
}

func (c *Client) GetOneDayTimeEntries(oneDay time.Time) ([]map[string]interface{}, error) {
	params := map[string]string{
		"start_date": beginningOfDay(oneDay).Format(time.RFC3339),
		"end_date":   endOfDay(oneDay).Format(time.RFC3339),
	}

	res, err := c.do(http.MethodGet, TimeEntriesPath, params, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var oneDayTimeEntries []map[string]interface{}
	err = json.Unmarshal(body, &oneDayTimeEntries)
	if err != nil {
		return nil, err
	}

	return oneDayTimeEntries, nil
}

func (c *Client) CreateTimeEntries(timeEntries []map[string]interface{}) error {
	for _, timeEntry := range timeEntries {
		timeEntry["created_with"] = "api"

		startTime, err := time.Parse(time.RFC3339, timeEntry["start"].(string))
		if err != nil {
			return err
		}
		stopTime, err := time.Parse(time.RFC3339, timeEntry["stop"].(string))
		if err != nil {
			return err
		}

		timeEntry["start"] = nextDay(startTime)
		timeEntry["stop"] = nextDay(stopTime)

		timeEntryToSend := map[string]interface{}{
			"time_entry": timeEntry,
		}

		b, err := json.Marshal(timeEntryToSend)
		if err != nil {
			return err
		}
		res, err := c.do(http.MethodPost, TimeEntriesPath, nil, b)
		if err != nil {
			return err
		}
		defer res.Body.Close()
	}

	return nil
}

func beginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)
}

func nextDay(t time.Time) time.Time {
	return t.AddDate(0, 0, +1)
}

func cleanPastTimeEntries(pastTimeEntries []map[string]interface{}) []map[string]interface{} {
	for i := range pastTimeEntries {
		delete(pastTimeEntries[i], "guid")
		delete(pastTimeEntries[i], "uid")
		delete(pastTimeEntries[i], "id")
		delete(pastTimeEntries[i], "at")
	}
	return pastTimeEntries
}

func main() {
	var (
		dayToShift int
		err error
	)
	if len(os.Args) < 2 {
		dayToShift = 2
	} else if len(os.Args) == 2 {
		dayToShift, err = strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("Too many arguments.")
	}

	log.Printf("Copy all time entries at the day shifted %v days from today to the next day.", dayToShift)

	loc, err := time.LoadLocation(TimeZone)
	if err != nil {
		log.Fatal(err)
	}

	dayToCopy := time.Now().AddDate(0, 0, +dayToShift).In(loc)

	client := NewClient(ApiUrl, Token)

	existedTimeEntries, err := client.GetOneDayTimeEntries(dayToCopy)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Found %v time entries at %v", len(existedTimeEntries), dayToCopy.Format("January 02, 2006"))

	cleanedPastTimeEntries := cleanPastTimeEntries(existedTimeEntries)

	err = client.CreateTimeEntries(cleanedPastTimeEntries)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Copied all time entries")
}
