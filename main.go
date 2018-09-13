package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

var (
	usersListUrl       string = "https://slack.com/api/users.list"
	reactionsListUrl   string = "https://slack.com/api/reactions.list"
	channelsListUrl    string = "https://slack.com/api/channels.list"
	chatPostMessageUrl string = "https://slack.com/api/chat.postMessage"
	reactionList       []Reaction
	// cursor             string = "first cursor"
	token                  string = os.Getenv("SLACK_TOKEN")
	slack_channel          string = os.Getenv("SLACK_CHANNEL")
	currentClientMsgID     string = ""
	currentClientMsgIDList []string
)

type Response struct {
	ResponseMetadata ResponseMetadata `json:"response_metadata"`
	Items            []Item           `json:"items"`
}

type ResponseMetadata struct {
	NextCursor string `json:"next_cursor"`
}

type Item struct {
	Type    string  `json:"type"`
	Channel string  `json:"channel"`
	Message Message `json:"message"`
	File    File    `json:"file"`
}

type Message struct {
	Type        string     `json:"type"`
	ClientMsgID string     `json:"client_msg_id"`
	Reactions   []Reaction `json:"reactions"`
}

type File struct {
	ID        string     `json:"id"`
	Reactions []Reaction `json:"reactions"`
}

type Reaction struct {
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	UserIDs []string `json:"users"`
}

type ChannelListResponse struct {
	Channels []Channel `json:"channels"`
}

type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserListResponse struct {
	Users []User `json:"members"`
}

type User struct {
	ID string `json:"id"`
}

type Emoji struct {
	Key   string
	Value int
}

type EmojiList []Emoji

func (p EmojiList) Len() int           { return len(p) }
func (p EmojiList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p EmojiList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func main() {
	if token == "" {
		log.Fatal("SLACK_TOKEN environment variable should be set")
	}

	if slack_channel == "" {
		slack_channel = "general"
	}

	users := getUsers()
	nextCursor := "first"
	for _, user := range users {
		nextCursor = "first"
		currentClientMsgIDList = []string{}
		fmt.Println(user.ID)
		for {
			if nextCursor := getReactions(user, nextCursor); nextCursor == "" {
				break
			}
		}
	}

	reactions := map[string]int{}
	for _, reaction := range reactionList {
		count, ok := reactions[reaction.Name]
		if ok == false {
			reactions[reaction.Name] = 1
		} else {
			reactions[reaction.Name] = count + 1
		}
	}

	//fmt.Println(len(reactions))
	//for key, value := range reactions {
	//	fmt.Println(key + " : " + strconv.Itoa(value))
	//}

	emojiList := rankByEmojiCount(reactions)
	var builder strings.Builder
	for _, emoji := range emojiList {
		builder.WriteString(":" + emoji.Key + ":" + " : " + strconv.Itoa(emoji.Value) + "\n")
	}

	channelID := getChannelID()
	fmt.Println(builder.String())
	fmt.Println(channelID)

	postMessage(channelID, builder.String())
}

func rankByEmojiCount(reactions map[string]int) EmojiList {
	emojiList := make(EmojiList, len(reactions))
	i := 0
	for k, v := range reactions {
		emojiList[i] = Emoji{k, v}
		i++
	}
	sort.Sort(sort.Reverse(emojiList))
	return emojiList
}

func getUsers() []User {
	req, err := http.NewRequest("GET", usersListUrl, nil)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	q := req.URL.Query()
	q.Add("token", token)
	req.URL.RawQuery = q.Encode()
	fmt.Println(req.URL.String())

	resp, err := http.Get(req.URL.String())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer resp.Body.Close()

	fmt.Println(resp.Body)

	response := &UserListResponse{}
	err = json.NewDecoder(resp.Body).Decode(response)

	return response.Users
}

func getReactions(user User, nextCursor string) string {
	req, err := http.NewRequest("GET", reactionsListUrl, nil)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	q := req.URL.Query()
	q.Add("token", token)
	q.Add("user", user.ID)
	if nextCursor != "first" {
		q.Add("cursor", nextCursor)
	}
	req.URL.RawQuery = q.Encode()
	fmt.Println(req.URL.String())

	//resp, err := http.Get(reactionsListUrl + "?" + values.Encode())
	resp, err := http.Get(req.URL.String())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer resp.Body.Close()

	fmt.Println(resp.Body)

	response := &Response{}
	err = json.NewDecoder(resp.Body).Decode(response)

	for _, item := range response.Items {
		if item.Type == "message" {
			if isIncludeClientMsgID(item.Message.ClientMsgID) {
				continue
			}
			currentClientMsgIDList = append(currentClientMsgIDList, item.Message.ClientMsgID)
			for _, reaction := range item.Message.Reactions {
				if isIncludeUser(user, reaction) != true {
					continue
				}
				reactionList = append(reactionList, reaction)
			}
		} else if item.Type == "file" {
			fmt.Println("item type is file")
			for _, reaction := range item.File.Reactions {
				if isIncludeUser(user, reaction) != true {
					continue
				}
				reactionList = append(reactionList, reaction)
			}
		}
	}

	fmt.Println(len(response.Items))
	fmt.Println(len(reactionList))
	fmt.Println(response.ResponseMetadata.NextCursor)
	cursor := response.ResponseMetadata.NextCursor
	fmt.Println(cursor)
	return cursor
}

func getChannelID() string {
	req, err := http.NewRequest("GET", channelsListUrl, nil)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	q := req.URL.Query()
	q.Add("token", token)

	req.URL.RawQuery = q.Encode()
	fmt.Println(req.URL.String())

	//resp, err := http.Get(reactionsListUrl + "?" + values.Encode())
	resp, err := http.Get(req.URL.String())
	if err != nil {
		fmt.Println(err)
		return ""
	}

	defer resp.Body.Close()

	response := &ChannelListResponse{}
	err = json.NewDecoder(resp.Body).Decode(response)

	targetChannelID := ""
	for _, channel := range response.Channels {
		if channel.Name == slack_channel {
			targetChannelID = channel.ID
		}
	}

	return targetChannelID
}

func postMessage(channelID string, message string) {
	values := url.Values{}
	values.Set("token", token)
	values.Add("channel", channelID)
	values.Add("text", message)

	req, err := http.NewRequest(
		"POST",
		chatPostMessageUrl,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
}

func isIncludeUser(user User, reaction Reaction) bool {
	for _, userID := range reaction.UserIDs {
		fmt.Println("check user id")
		fmt.Println(userID)
		fmt.Println(user.ID)
		if userID == user.ID {
			return true
		}
	}
	return false
}

func isIncludeClientMsgID(clientMsgID string) bool {
	for _, id := range currentClientMsgIDList {
		if id == clientMsgID {
			return true
		}
	}
	return false
}
