package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/codegangsta/cli"
)

/**
User List: https://api.slack.com/methods/users.list:
	Example: https://slack.com/api/users.list?token=[YOUR_SLACK_API_TOKEN]


	Empty UserList: {"ok": true,"members": [],"cache_ts": 0}
**/

var isVerbose = false
var saveCache = false

var apiKey = ""
var channel = ""

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

func main() {
	app := cli.NewApp()
	app.Version = "0.0.3"
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

		channel = c.String("channel")

		dumpDelta(c.String("cache"))
	}
	app.Run(os.Args)
}

func dumpDelta(fileName string) {
	var hasChanges = false
	var result = ""

	var memberList = loadMembersFromFile(fileName)
	if memberList == nil {
		result = fmt.Sprintf("%sNo member list cached. Will create one\n", result)
		memberList := loadMemberListAsJson()
		writeCache(fileName, memberList)

		return
	}

	var memberList2 = loadMemberList()

	result = fmt.Sprintf("%sSearching for MIA\n", result)

	// Search for new members
	for _, element := range memberList.Members {
		member := findMember(element.ID, memberList2)
		if member == nil {
			hasChanges = true
			result = fmt.Sprintf("%s\t--- Missing Member, %s, %s\n", result, element.RealName, element.Profile.Email)
		} else if member.Deleted != element.Deleted {
			isDelete := "no"

			if member.Deleted {
				isDelete = "YES"
			}
			hasChanges = true
			result = fmt.Sprintf("%s\t--- Member, %s, %s, isDelete: %s\n", result, element.RealName, element.Profile.Email, isDelete)
		}
	}

	result = fmt.Sprintf("%sSearching for new members\n", result)

	// Search for missing members
	for _, element := range memberList2.Members {
		member := findMember(element.ID, memberList)
		if member == nil {
			hasChanges = true

			// Build a relaible name
			var name =  element.ID
			if len(element.RealName) > 0 {
				name = element.RealName
			} else if (len(element.Name) > 0) {
				name = element.Name
			} 

			var isBot = ""
			if element.IsBot {
				isBot = fmt.Sprintf(", isBot: YES, (%s)", element.Profile.BotId );
			}

			result = fmt.Sprintf("%s\t+++ New Member, %s, %s, %s - %s \n", result, name, element.Profile.Email, element.Profile.Title, isBot)
		}
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

func loadMemberListAsJson() []byte {
	url := "https://slack.com/api/users.list?token=" + apiKey

	response, err := http.Get(url)
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
	message = url.QueryEscape(message)
	channel = url.QueryEscape(channel)

	url := "https://slack.com/api/chat.postMessage?token=" + apiKey + "&channel=" + channel + "&text=" + message

	response, err := http.Get(url)
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
