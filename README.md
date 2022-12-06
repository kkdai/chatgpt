chatgpt: ChatGPT SDK in Go
==============

[![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/kkdai/gptchat/master/LICENSE)  [![GoDoc](https://godoc.org/github.com/kkdai/cyk?status.svg)](https://godoc.org/github.com/kkdai/gptchat)   ![Go](https://github.com/kkdai/gptchat/workflows/Go/badge.svg) [![goreportcard.com](https://goreportcard.com/badge/github.com/kkdai/gptchat)](https://goreportcard.com/report/github.com/kkdai/gptchat)

Installation and Usage
=============

Install
---------------

    go get github.com/kkdai/gptchat

Usage
---------------

## Use

```go
package main

import (
    "github.com/kkdai/gptchat"
)

const (
 //Get consumer key and secret from https://dev.twitter.com/apps/new
 ConsumerKey string = ""
 ConsumerSecret string = ""
)
func main() {
 twitterClient = NewDesktopClient(ConsumerKey, ConsumerSecret)
 
 //Show a UI to display URL.
 //Please go to this URL to get code to continue
 twitterClient.DoAuth()
 
 //Get timeline only latest one
 timeline, byteData, err :=twitterClient.QueryTimeLine(1)
 
 if err == nil {
  fmt.Println("timeline struct=", timeline, " byteData=", string(byteData) )
 }
}

```

#### Use Server Mode OAuth in Twitter (Three-legged Authentication)

![](https://g.twimg.com/dev/sites/default/files/images_documentation/sign-in-oauth-1_0.png)

Refer to [twitter document](https://dev.twitter.com/web/sign-in/implementing), use server oauth mode to sign in Twitter

Please note, you might get error if you don't set `Callback_URL` in twitter setting.  You need input valid URL:

1. Could not be `localhost`, use other setting in `hosts` if you want to test it locally.
2. Must need input any URL otherwise your app will treat as `desktop app`.

```go

package main

import (
 "flag"
 "fmt"
 "net/http"
 "os"

 . "github.com/kkdai/twitter"
)

var ConsumerKey string
var ConsumerSecret string
var twitterClient *ServerClient

func init() {
 ConsumerKey = os.Getenv("ConsumerKey")
 ConsumerSecret = os.Getenv("ConsumerSecret")
}

const (
 //This URL need note as follow:
 // 1. Could not be localhost, change your hosts to a specific domain name
 // 2. This setting must be identical with your app setting on twitter Dev
 CallbackURL string = "http://YOURDOMAIN.com/maketoken"
)

func main() {

 if ConsumerKey == "" && ConsumerSecret == "" {
  fmt.Println("Please setup ConsumerKey and ConsumerSecret.")
  return
 }

 var port *int = flag.Int(
  "port",
  8888,
  "Port to listen on.")

 flag.Parse()

 fmt.Println("[app] Init server key=", ConsumerKey, " secret=", ConsumerSecret)
 twitterClient = NewServerClient(ConsumerKey, ConsumerSecret)
 http.HandleFunc("/maketoken", GetTwitterToken)
 http.HandleFunc("/request", RedirectUserToTwitter)
 http.HandleFunc("/follow", GetFollower)
 http.HandleFunc("/followids", GetFollowerIDs)
 http.HandleFunc("/time", GetTimeLine)
 http.HandleFunc("/user", GetUserDetail)
 http.HandleFunc("/", MainProcess)

 u := fmt.Sprintf(":%d", *port)
 fmt.Printf("Listening on '%s'\n", u)
 http.ListenAndServe(u, nil)
}

func MainProcess(w http.ResponseWriter, r *http.Request) {

 if !twitterClient.HasAuth() {
  fmt.Fprintf(w, "<BODY><CENTER><A HREF='/request'><IMG SRC='https://g.twimg.com/dev/sites/default/files/images_documentation/sign-in-with-twitter-gray.png'></A></CENTER></BODY>")
  return
 } else {
  //Logon, redirect to display time line
  timelineURL := fmt.Sprintf("http://%s/time", r.Host)
  http.Redirect(w, r, timelineURL, http.StatusTemporaryRedirect)
 }
}

func RedirectUserToTwitter(w http.ResponseWriter, r *http.Request) {
 fmt.Println("Enter redirect to twitter")
 fmt.Println("Token URL=", CallbackURL)
 requestUrl := twitterClient.GetAuthURL(CallbackURL)

 http.Redirect(w, r, requestUrl, http.StatusTemporaryRedirect)
 fmt.Println("Leave redirtect")
}

func GetTimeLine(w http.ResponseWriter, r *http.Request) {
 timeline, bits, _ := twitterClient.QueryTimeLine(1)
 fmt.Println("TimeLine=", timeline)
 fmt.Fprintf(w, "The item is: "+string(bits))

}
func GetTwitterToken(w http.ResponseWriter, r *http.Request) {
 fmt.Println("Enter Get twitter token")
 values := r.URL.Query()
 verificationCode := values.Get("oauth_verifier")
 tokenKey := values.Get("oauth_token")

 twitterClient.CompleteAuth(tokenKey, verificationCode)
 timelineURL := fmt.Sprintf("http://%s/time", r.Host)

 http.Redirect(w, r, timelineURL, http.StatusTemporaryRedirect)
}

func GetFollower(w http.ResponseWriter, r *http.Request) {
 followers, bits, _ := twitterClient.QueryFollower(10)
 fmt.Println("Followers=", followers)
 fmt.Fprintf(w, "The item is: "+string(bits))
}

func GetFollowerIDs(w http.ResponseWriter, r *http.Request) {
 followers, bits, _ := twitterClient.QueryFollowerIDs(10)
 fmt.Println("Follower IDs=", followers)
 fmt.Fprintf(w, "The item is: "+string(bits))
}
func GetUserDetail(w http.ResponseWriter, r *http.Request) {
 followers, bits, _ := twitterClient.QueryFollowerById(2244994945)
 fmt.Println("Follower Detail of =", followers)
 fmt.Fprintf(w, "The item is: "+string(bits))
}

```

Inspired By
=============

- [Twitter API authentication in Go](http://venkat.io/posts/twitter-api-auth-golang/)  
- [https://github.com/mrjones/oauth](https://github.com/mrjones/oauth)
- [Twitter:sign-in Doc](https://dev.twitter.com/web/sign-in)
- [Twitter: Browser sign in flow Overview](https://dev.twitter.com/web/sign-in/desktop-browser)

License
---------------

This package is licensed under MIT license. See [LICENSE](/LICENSE) for details.
