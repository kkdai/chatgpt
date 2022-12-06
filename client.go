package gptchat

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"
)

const (
	//Basic OAuth related URLs
	OAUTH_REQUES_TOKEN string = "https://api.twitter.com/oauth/request_token"
	OAUTH_AUTH_TOKEN   string = "https://api.twitter.com/oauth/authorize"
	OAUTH_ACCESS_TOKEN string = "https://api.twitter.com/oauth/access_token"

	//List API URLs
	API_BASE     string = "https://api.twitter.com/1.1/"
	API_TIMELINE string = API_BASE + "statuses/home_timeline.json"
)

type Client struct {
	HttpConn *http.Client
}

func (c *Client) HasAuth() bool {
	return c.HttpConn != nil
}

func (c *Client) BasicQuery(queryString string) ([]byte, error) {
	if c.HttpConn == nil {
		return nil, errors.New("No Client OAuth")
	}

	response, err := c.HttpConn.Get(queryString)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	bits, err := ioutil.ReadAll(response.Body)
	return bits, err
}

// User Timeline by UserID
// func (c *Client) QueryUserTimelineByUserID(user_id string) (UserTimeline, []byte, error) {
// 	requesURL := fmt.Sprintf("%s?user_id=%s", API_USER_TIMELINE, user_id)
// 	data, err := c.BasicQuery(requesURL)
// 	ret := UserTimeline{}
// 	err = json.Unmarshal(data, &ret)
// 	return ret, data, err
// }
