package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/codegangsta/cli"
)

/**
Conversation List: https://api.slack.com/methods/conversations.list:
	Example: https://slack.com/api/conversations.list
**/

var isVerbose = false
var saveCache = false

var apiKey = ""
var channel = ""

// Channel contains all the information of a channel
type Channel struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	IsChannel      bool     `json:"is_channel"`
	Created        int      `json:"created"`
	Creator        string   `json:"creator"`
	IsArchived     bool     `json:"is_archived"`
	IsGeneral      bool     `json:"is_general"`
	NameNormalized string   `json:"name_normalized"`
	IsShared       bool     `json:"is_shared"`
	IsOrgShared    bool     `json:"is_org_shared"`
	IsMember       bool     `json:"is_member"`
	IsPrivate      bool     `json:"is_private"`
	IsMpim         bool     `json:"is_mpim"`
	Members        []string `json:"members"`
	Topic          struct {
		Value   string `json:"value"`
		Creator string `json:"creator"`
		LastSet int    `json:"last_set"`
	} `json:"topic"`
	Purpose struct {
		Value   string `json:"value"`
		Creator string `json:"creator"`
		LastSet int    `json:"last_set"`
	} `json:"purpose"`
	PreviousNames []interface{} `json:"previous_names"`
	NumMembers    int           `json:"num_members"`
}

// ChannelList - TODO: comment this exported structure
type ChannelList struct {
	Ok               bool       `json:"ok"`
	Channels         []*Channel `json:"channels,omitempty"`
	CacheTimestamp   uint64     `json:"cache_ts"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

type SlackMessage struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func main() {
	app := cli.NewApp()
	app.Version = "0.0.1"
	//app.Name = "Slack Channel Monitor"
	app.Usage = "Track a Slack channel list"
	//app.UsageText = "TODO describe application usage"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "apikey, k",
			Value:  "",
			Usage:  "Required Slack API key",
			EnvVar: "SLACK_API_KEY",
		},
		cli.StringFlag{
			Name:  "verbose",
			Value: "false",
			Usage: "Dumps additional information to console",
		},
		cli.StringFlag{
			Name:  "cache, c",
			Value: "./channelList.cache",
			Usage: "Optional, set cache file to use.",
		},
		cli.StringFlag{
			Name:  "updatecache, u",
			Value: "false",
			Usage: "Optional, saves all current channels to cache",
		},
		cli.StringFlag{
			Name:  "channel, l",
			Value: "",
			Usage: "Optional, Slack channel to deliver results to. If not set a message will not be sent to Slack.",
		},
	}
	app.Action = func(c *cli.Context) {
		if c.String("apikey") == "" {
			fmt.Printf("\n\nError: Slack API key must be set\n\n")

			cli.ShowAppHelp(c)
			return
		}

		apiKey = c.String("apikey")

		isVerbose = false

		if c.String("verbose") == "true" {
			isVerbose = true
		}

		if c.String("updatecache") == "true" {
			saveCache = true
		}

		// monitorString := c.String("monitor")
		// if monitorString != "" {
		// 	fmt.Printf("\nWill monitor the following domains:\n\t%s\n\n", monitorString)
		// 	monitored = strings.Split(monitorString, ",")
		// }

		channel = c.String("channel")

		dumpDelta(c.String("cache"))
	}
	app.Run(os.Args)
}

func dumpDelta(fileName string) {
	var hasChanges = false
	var result = ""

	var channelList = loadChannelsFromFile(fileName)
	if channelList == nil {
		result = fmt.Sprintf("%sNo channel list cached. Will create one\n", result)
		channelList := loadChannelList()
		json, _ := json.Marshal(channelList)
		writeCache(fileName, json)

		return
	}

	var channelList2 = loadChannelList()

	result = fmt.Sprintf("%sSearching for missing channels\n", result)

	// Search for missing members
	for _, element := range channelList.Channels {
		channel := findChannel(element.ID, channelList2)
		if channel == nil {
			isTemp := strings.HasPrefix(element.Name, "z-")
			if !isTemp {
				hasChanges = true
				result = fmt.Sprintf("%s\t--- Missing Channel `%s`\n", result, element.Name)
			}
		} else if channel.IsArchived != element.IsArchived {
			isDelete := "no"
			//fmt.Printf("%s changed states from ", element.Name, channel.IsArchived, element.IsArchived)
			if channel.IsArchived {
				isDelete = "YES"
			}

			isTemp := strings.HasPrefix(element.Name, "z-")
			if !isTemp {
				hasChanges = true
				result = fmt.Sprintf("%s\t*** Channel Changed, %s, %s, isDelete: %s\n", result, element.Name, element.Purpose.Value, isDelete)
			}
		}
	}

	result = fmt.Sprintf("%sSearching for new channels\n", result)

	// Search for new members
	//hasMonitoredEntries := false
	//monitoredEntries := "Searching for monitored members @everyone WARNING possible bad actor(s) joined.\n*Please verify these users:*\n"

	for _, element := range channelList2.Channels {
		channel := findChannel(element.ID, channelList)
		if channel == nil {
			// Build a relaible name
			var name = element.ID

			// TODO: this is redundant *******
			if len(element.Name) > 0 {
				name = element.Name
			} else if len(element.Name) > 0 {
				name = element.Name
			}
			isTemp := strings.HasPrefix(name, "z-")

			if !isTemp {
				hasChanges = true
				result = fmt.Sprintf("%s\t+++ New Channel, #%s - `%s` \n", result, element.NameNormalized, element.Purpose.Value)
			}
		}
	}

	if saveCache {
		fmt.Println("Updating cache")
		json, _ := json.Marshal(channelList2)
		writeCache(fileName, json)
	}

	fmt.Println(result)

	if channel != "" && hasChanges {
		postMessage(channel, result)
	}
}

func loadChannelsFromFile(fileName string) *ChannelList {

	file, e := ioutil.ReadFile(fileName)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		return nil
	}

	var channels *ChannelList
	json.Unmarshal(file, &channels)

	return channels
}

func findChannel(id string, channels *ChannelList) *Channel {
	for _, element := range channels.Channels {
		if element.ID == id {
			return element
		}
	}

	return nil
}

func loadChannelListAsJSON(cursor string) []byte {
	url := "https://slack.com/api/conversations.list?exclude_archived=true&types=public_channel"

	if len(cursor) > 0 {
		url = url + "&cursor=" + cursor
	}
	fmt.Printf("UserList URL: %s\n", url)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+apiKey)
	response, err := client.Do(req)

	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}

		return contents
	}

	return nil
}

func loadChannelList() *ChannelList {
	var cursor = ""
	pageNum := 1
	contents := loadChannelListAsJSON(cursor)

	if contents == nil {
		fmt.Printf("Current List Failed: nil conversation list returned")
		os.Exit(1)
	}
	var channels *ChannelList
	json.Unmarshal(contents, &channels)

	if !channels.Ok {
		fmt.Printf("Current List Failed: \n%#v", channels)
		os.Exit(1)
	}

	cursor = channels.ResponseMetadata.NextCursor
	fmt.Printf("\tNext Cursor: %s\n", cursor)
	//cursor = ""
	for len(cursor) > 0 {
		pageNum = pageNum + 1
		var data = loadChannelListAsJSON(cursor)
		var nextPage *ChannelList
		json.Unmarshal(data, &nextPage)
		if nextPage == nil || !nextPage.Ok {
			fmt.Printf("Failed to load conversation list from server: \n%#v\n", nextPage)
			os.Exit(1)
		}

		channels.Channels = append(channels.Channels, nextPage.Channels...)
		cursor = nextPage.ResponseMetadata.NextCursor
		fmt.Printf("\t%d, Next Cursor: %s\n", pageNum, cursor)
	}

	return channels
}

func writeCache(filename string, byteArray []byte) {

	json := string(byteArray)

	fmt.Println("writing: " + filename)
	f, err := os.Create(filename)
	if err != nil {
		fmt.Println(err)
	}
	n, err := io.WriteString(f, json)
	if err != nil {
		fmt.Println(n, err)
	}
	f.Close()
}

func postMessage(channel string, message string) []byte {
	slackMessage := &SlackMessage{
		channel,
		message,
	}
	json, err := json.Marshal(slackMessage)
	//fmt.Printf("POST: %s\n\n", json)

	url := "https://slack.com/api/chat.postMessage"
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(json))
	req.Header.Add("Authorization", "Bearer "+apiKey)
	req.Header.Add("Content-type", "application/json; charset=utf-8")
	//req.Header.Add("Content-Type", "text/html; charset=utf-8")
	response, err := client.Do(req)

	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)

		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		} /*
			else {
				fmt.Println(string(contents))
			}
		*/

		return contents
	}

	return nil
}
