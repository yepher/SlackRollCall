package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/codegangsta/cli"
)

/**
User List: https://api.slack.com/methods/users.list:
	Example: https://slack.com/api/users.list?token=[YOUR_SLACK_API_TOKEN]
**/

var isVerbose = false
var saveCache = false

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
	app.Version = "0.0.1"
	app.Name = "Slack Role Call"
	app.Usage = "Slack Role Call"
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
			Usage: "Set cache file to use.",
		},
		cli.StringFlag{
			Name:  "updatecache, u",
			Value: "false",
			Usage: "Saves all current members to cache",
		},
	}
	app.Action = func(c *cli.Context) {

		if c.String("apikey") == "" {
			fmt.Printf("Error: SparkPost API key must be set\n")
			return
		}

		isVerbose = false

		if c.String("verbose") == "true" {
			isVerbose = true
		}

		if c.String("updatecache") == "true" {
			saveCache = true
		}

		dumpDelta(c.String("cache"))
	}
	app.Run(os.Args)
}

func dumpDelta(fileName string) {
	var memberList = loadMembersFromFile(fileName)
	if memberList == nil {
		fmt.Printf("No member list cached. Will create one\n")
		memberList := loadMemberListAsJson()
		writeCache(fileName, memberList)

		return
	}
	//fmt.Printf("Members List: %s\n\n", memberList)

	//var memberList2 = loadMembers("./userList2.json")
	var memberList2 = loadMemberList()

	fmt.Println("Searching for MIA")
	// Search for new members
	for _, element := range memberList.Members {
		member := findMember(element.ID, memberList2)
		if member == nil {
			fmt.Printf("\t--- Missing Member, %s, %s\n", element.RealName, element.Profile.Email)
		} else if member.Deleted != element.Deleted {
			isDelete := "no"

			if member.Deleted {
				isDelete = "YES"
			}

			fmt.Printf("\t--- Member, %s, %s, isDelete: %s\n", element.RealName, element.Profile.Email, isDelete)

		}
	}

	fmt.Println("Searching for new members")
	// Search for missing members
	for _, element := range memberList2.Members {
		member := findMember(element.ID, memberList)
		if member == nil {
			fmt.Printf("\t+++ New Member, %s, %s - %s\n", element.RealName, element.Profile.Email, element.Profile.Title)
		}
	}

	if saveCache {
		fmt.Println("Updating cache")
		newMemberList := loadMemberListAsJson()
		writeCache(fileName, newMemberList)
	}

	//fmt.Printf("Results: %s\n", members)
}

func loadMembersFromFile(fileName string) *MemberList {
	//fmt.Printf("Loading: %s", fileName)

	file, e := ioutil.ReadFile(fileName)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		return nil
	}
	//fmt.Printf("Will parse: %s", file)

	var members *MemberList
	json.Unmarshal(file, &members)

	return members
}

func findMember(id string, members *MemberList) *User {
	//fmt.Printf("\tLooking up: %s\n", id)
	for _, element := range members.Members {
		if element.ID == id {
			return element
		}
	}

	return nil
}

func loadMemberListAsJson() []byte {
	url := "https://slack.com/api/users.list?token=" + os.Getenv("SLACK_API_KEY")
	//fmt.Printf("URL: %s\n", url)

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
