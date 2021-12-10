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
User List: https://api.slack.com/methods/users.list:
	Example: https://slack.com/api/users.list


	Empty UserList: {"ok": true,"members": [],"cache_ts": 0}
**/

var isVerbose = false
var saveCache = false

var apiKey = ""
var channel = ""
var monitored = []string{}

// UserProfile contains all the information details of a given user
type UserProfile struct {
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	RealName           string `json:"real_name"`
	RealNameNormalized string `json:"real_name_normalized"`
	Email              string `json:"email"`
	Skype              string `json:"skype"`
	Phone              string `json:"phone"`
	Image24            string `json:"image_24"`
	Image32            string `json:"image_32"`
	Image48            string `json:"image_48"`
	Image72            string `json:"image_72"`
	Image192           string `json:"image_192"`
	ImageOriginal      string `json:"image_original"`
	Title              string `json:"title"`
	BotId              string `json:"bot_id"`
}

// User contains all the information of a user
type User struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Deleted           bool        `json:"deleted"`
	Color             string      `json:"color"`
	RealName          string      `json:"real_name"`
	TZ                string      `json:"tz,omitempty"`
	TZLabel           string      `json:"tz_label"`
	TZOffset          int         `json:"tz_offset"`
	Profile           UserProfile `json:"profile"`
	IsBot             bool        `json:"is_bot"`
	IsAdmin           bool        `json:"is_admin"`
	IsOwner           bool        `json:"is_owner"`
	IsPrimaryOwner    bool        `json:"is_primary_owner"`
	IsRestricted      bool        `json:"is_restricted"`
	IsUltraRestricted bool        `json:"is_ultra_restricted"`
	Has2FA            bool        `json:"has_2fa"`
	HasFiles          bool        `json:"has_files"`
	Presence          string      `json:"presence"`
}

// UserPresence contains details about a user online status
type UserPresence struct {
	Presence        string `json:"presence,omitempty"`
	Online          bool   `json:"online,omitempty"`
	AutoAway        bool   `json:"auto_away,omitempty"`
	ManualAway      bool   `json:"manual_away,omitempty"`
	ConnectionCount int    `json:"connection_count,omitempty"`
	//LastActivity    JSONTime `json:"last_activity,omitempty"`
}

type MemberList struct {
	Ok             bool    `json:"ok"`
	Members        []*User `json:"members,omitempty"`
	CacheTimestamp uint64  `json:"cache_ts"`
}

type SlackMessage struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func main() {
	app := cli.NewApp()
	app.Version = "0.0.4"
	//app.Name = "Slack Role Call"
	app.Usage = "Track a Slack team's membership changes"
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
			Value: "./userList.cache",
			Usage: "Optional, set cache file to use.",
		},
		cli.StringFlag{
			Name:  "updatecache, u",
			Value: "false",
			Usage: "Optional, saves all current members to cache",
		},
		cli.StringFlag{
			Name:  "channel, l",
			Value: "",
			Usage: "Optional, Slack channel to deliver results to. If not set a message will not be sent to Slack.",
		},
		cli.StringFlag{
			Name:  "monitor, m",
			Value: "",
			Usage: "Optional, A list of domains to monitor when a new user appears.",
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

		monitorString := c.String("monitor")
		if monitorString != "" {
			fmt.Printf("\nWill monitor the following domains:\n\t%s\n\n", monitorString)
			monitored = strings.Split(monitorString, ",")
		}

		channel = c.String("channel")

		dumpDelta(c.String("cache"))
	}
	app.Run(os.Args)
}

func dumpDelta(fileName string) {
	var hasChanges = false
	var result = ""

	var previousList = loadMembersFromFile(fileName)
	if previousList == nil {
		result = fmt.Sprintf("%sNo member list cached. Will create one\n", result)
		previousList := loadMemberListAsJson()
		writeCache(fileName, previousList)

		return
	}

	var currentList = loadMemberList()

	if !currentList.Ok {
		fmt.Printf("Current List Failed: \n%#v", currentList)
		os.Exit(1)
	}

	result = fmt.Sprintf("%sSearching for MIA\n", result)

	// Search for members who were in previous list
	// and no longer exit in the current list
	// or the deleted flag has changed
	for _, previousRecord := range previousList.Members {
		currentRecord := findMember(previousRecord.ID, currentList)
		if currentRecord == nil {
			hasChanges = true
			result = fmt.Sprintf("%s\t--- Missing Member, %s, %s\n", result, previousRecord.RealName, previousRecord.Profile.Email)
		} else if currentRecord.Deleted != previousRecord.Deleted {
			isDelete := "No"

			if currentRecord.Deleted {
				isDelete = "Yes"
			}
			hasChanges = true
			result = fmt.Sprintf("%s\t--- Member, %s, %s, isDelete: %s\n", result, currentRecord.RealName, currentRecord.Profile.Email, isDelete)

			if previousRecord != nil {
				fmt.Printf("\n\nPrevious Record: %+v\n", previousRecord)
			}

			if currentRecord != nil {
				fmt.Printf("Current Record: %+v\n\n", currentRecord)
			}
		}
	}

	result = fmt.Sprintf("%sSearching for new members\n", result)

	// Search for new members
	hasMonitoredEntries := false
	monitoredEntries := "Searching for monitored members @everyone WARNING possible bad actor(s) joined.\n*Please verify these users:*\n"

	for _, element := range currentList.Members {
		previousRecord := findMember(element.ID, previousList)
		if previousRecord == nil {
			hasChanges = true

			// Build a reliable name
			var name = element.ID
			if len(element.RealName) > 0 {
				name = element.RealName
			} else if len(element.Name) > 0 {
				name = element.Name
			}

			var isBot = ""
			if element.IsBot {
				isBot = fmt.Sprintf(", isBot: YES, (%s)", element.Profile.BotId)
			}

			result = fmt.Sprintf("%s\t+++ New Member, %s, %s, %s - %s \n", result, name, element.Profile.Email, element.Profile.Title, isBot)

			if isMonitored(element.Profile.Email) {
				hasMonitoredEntries = true
				monitoredEntries = fmt.Sprintf("%s\t*** Suspect Member, %s, %s, %s - %s \n", monitoredEntries, name, element.Profile.Email, element.Profile.Title, isBot)
			}
		}
	}

	if hasMonitoredEntries {
		result = fmt.Sprintf("%s\n%s", result, monitoredEntries)
	}

	if saveCache {
		fmt.Println("Updating cache")
		newMemberList := loadMemberListAsJson()
		writeCache(fileName, newMemberList)
	}

	fmt.Println(result)

	if channel != "" && hasChanges {
		postMessage(channel, result)
	}
}

func loadMembersFromFile(fileName string) *MemberList {

	file, e := ioutil.ReadFile(fileName)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		return nil
	}

	var members *MemberList
	json.Unmarshal(file, &members)

	return members
}

func findMember(id string, members *MemberList) *User {
	for _, element := range members.Members {
		if element.ID == id {
			return element
		}
	}

	return nil
}

func isMonitored(id string) bool {
	fmt.Printf("Checking: %s\n", id)
	for _, element := range monitored {
		if caseInsensitiveContains(id, element) {
			fmt.Printf("%s matched monitored domain %s\n", id, element)
			return true
		}
	}
	return false
}

func caseInsensitiveContains(s, substr string) bool {
	s, substr = strings.ToUpper(s), strings.ToUpper(substr)
	return strings.Contains(s, substr)
}

func loadMemberListAsJson() []byte {
	url := "https://slack.com/api/users.list?pretty=1"

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

func loadMemberList() *MemberList {
	contents := loadMemberListAsJson()

	if contents != nil {
		var members *MemberList
		json.Unmarshal(contents, &members)
		return members
	}

	return nil
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
	fmt.Printf("POST: %s\n\n", json)

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
		} else {
			fmt.Println(string(contents))
		}

		return contents
	}

	return nil
}
