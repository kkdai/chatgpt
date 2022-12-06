package gptchat

import (
	"log"

	"github.com/mrjones/oauth"
)

func NewServerClient(consumerKey, consumerSecret string) *ServerClient {
	//newClient := NewClient(consumerKey, consumerKey)

	newServer := new(ServerClient)

	newServer.OAuthConsumer = oauth.NewConsumer(
		consumerKey,
		consumerSecret,
		oauth.ServiceProvider{
			RequestTokenUrl:   OAUTH_REQUES_TOKEN,
			AuthorizeTokenUrl: OAUTH_AUTH_TOKEN,
			AccessTokenUrl:    OAUTH_ACCESS_TOKEN,
		},
	)

	//Enable debug info
	newServer.OAuthConsumer.Debug(false)

	// newServer.Client = *newClient
	newServer.OAuthTokens = make(map[string]*oauth.RequestToken)
	return newServer
}

type ServerClient struct {
	Client
	OAuthConsumer *oauth.Consumer
	OAuthTokens   map[string]*oauth.RequestToken
}

func (s *ServerClient) GetAuthURL(tokenUrl string) (string, error) {
	token, requestUrl, err := s.OAuthConsumer.GetRequestTokenAndUrl(tokenUrl)

	if err != nil {
		log.Println(err)
		return "", nil
	}
	// Make sure to save the token, we'll need it for AuthorizeToken()
	s.OAuthTokens[token.Token] = token
	return requestUrl, nil
}

func (s *ServerClient) CompleteAuth(tokenKey, verificationCode string) error {
	accessToken, err := s.OAuthConsumer.AuthorizeToken(s.OAuthTokens[tokenKey], verificationCode)
	if err != nil {
		log.Println(err)
		return err
	}

	s.HttpConn, err = s.OAuthConsumer.MakeHttpClient(accessToken)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
