// tweetlib - A fully oauth-authenticated Go Twitter library
//
// Copyright 2011 The Tweetlib Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tweetlib

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"reflect"
)

var _ = reflect.TypeOf

const (
	// URL to post tweets
	postURL = "http://api.twitter.com/1.1/statuses/update.json"
	// General URL for API calls
	apiURL = "https://api.twitter.com/1.1"
)

// twitterError represents an error generated by
// the Twitter API
type twitterError struct {
	Message string `json:message` // Error message
	Code    int    `json:code`    // Error code
}

// twitterErrorReply: contains a list of errors returned
// for a request to the Twitter API
type twitterErrorReply struct {
	Errors []twitterError `json:errors`
}

// Twitter error responses can actually contain
// multiple errors. This function preps them
// for a nice display
func (ter *twitterErrorReply) String() string {
	buf := bytes.NewBufferString("")
	for i := range ter.Errors {
		fmt.Fprintf(buf, "%s (%d)\n", ter.Errors[i].Message, ter.Errors[i].Code)
	}
	return buf.String()
}

// Checks whether the response is an error
func checkResponse(res *http.Response) (err error) {
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return nil
	}
	slurp, err := ioutil.ReadAll(res.Body)
	fmt.Printf("%s\n", slurp)
	if err != nil {
		return err
	}
	var jerr twitterErrorReply
	if err = json.Unmarshal(slurp, &jerr); err != nil {
		return
	}
	return errors.New(jerr.String())
}

// Creates a new twitter client
func New(transport *Transport) (*Client, error) {
	if transport.Client() == nil {
		return nil, errors.New("client is nil")
	}
	c := &Client{client: transport.Client()}
	return c, nil
}

// Client: Twitter API client provides access to the various
// API services
type Client struct {
	client *http.Client
}

// Performs an arbitrary API call and returns the response JSON if successful.
// This is generally used internally by other functions but it can also
// be used to perform API calls not directly supported by tweetlib.
//
// For example
//
//   opts := NewOptionals()
//   opts.Add("status", "Hello, world")
//   rawJSON, _ := client.CallJSON("POST", "statuses/update_status", opts)
//   var tweet Tweet
//   err := json.Unmarshal(rawJSON, &tweet)
//
// is the same as
//
//   tweet, err := client.UpdateStatus("Hello, world", nil)
func (c *Client) CallJSON(method, endpoint string, opts *Optionals) (rawJSON []byte, err error) {
	if method != "GET" && method != "POST" {
		err = fmt.Errorf("Invalid method '%s'. Must be either GET or POST.", method)
		return
	}
	if opts == nil {
		opts = NewOptionals()
	}
	endpoint = fmt.Sprintf("%s/%s.json?%s", apiURL, endpoint, opts.Values.Encode())
	fmt.Println(endpoint)
	var req *http.Request
	if method == "POST" {
		body := bytes.NewBuffer([]byte(opts.Values.Encode()))
		req, _ = http.NewRequest(method, endpoint, body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, _ = http.NewRequest(method, endpoint, nil)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return
	}
	if err = checkResponse(res); err != nil {
		return
	}
	rawJSON, err = ioutil.ReadAll(res.Body)
	return
}

// Performs an arbitrary API call and tries to unmarshal the result into
// 'resp' on success. This is generally used internally by the other functions
// but it could be used to perform unsupported API calls.
//
// Example usage:
//
//     var tweet Tweet
//     opts := NewOptionals()
//     opts.Add("status", "Hello, world")
//     err := client.Call("POST", "statuses/update_status", opts, &tweet)
//
// is equivalent to
//
//     tweet, err := client.UpdateStatus("Hello, world", nil)
func (c *Client) Call(method, endpoint string, opts *Optionals, resp interface{}) (err error) {
	rawJSON, err := c.CallJSON(method, endpoint, opts)
	if err != nil {
		return
	}
	if resp != nil {
		if err = json.Unmarshal(rawJSON, resp); err != nil &&
			reflect.TypeOf(err) != reflect.TypeOf(&json.UnmarshalTypeError{}) {
			return err
		}
	}
	return nil
}

// Returns the 20 (by default) most recent tweets containing a users's
// @screen_name for the authenticating user.
// THis method can only return up to 800 tweets (via the "count" optional
// parameter.
// See https://dev.twitter.com/docs/api/1.1/get/statuses/mentions_timeline
func (c *Client) Mentions(opts *Optionals) (tweets *TweetList, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	tweets = &TweetList{}
	err = c.Call("GET", "statuses/mentions_timeline", opts, tweets)
	return
}

// Returns a collection of the most recent Tweets posted by the user indicated
// by the screen_name.
// See https://dev.twitter.com/docs/api/1.1/get/statuses/user_timeline
func (c *Client) UserTimeline(screenname string, opts *Optionals) (tweets *TweetList, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("screen_name", screenname)
	tweets = new(TweetList)
	err = c.Call("GET", "statuses/user_timeline", opts, tweets)
	return
}

// Returns a collection of the most recent Tweets and retweets posted by
// the authenticating user and the users they follow.
// See https://dev.twitter.com/docs/api/1.1/get/statuses/home_timeline
func (c *Client) HomeTimeline(opts *Optionals) (tweets *TweetList, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	tweets = new(TweetList)
	err = c.Call("GET", "statuses/home_timeline", opts, tweets)
	return
}

// Returns a collection of the  most recent tweets authored by the
// authenticating user that have been retweeted by others.
// See https://dev.twitter.com/docs/api/1.1/get/statuses/retweets_of_me
func (c *Client) RetweetsOfMe(opts *Optionals) (tweets *TweetList, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	tweets = new(TweetList)
	err = c.Call("GET", "statuses/retweets_of_me", opts, tweets)
	return
}

// Update: posts a status update to Twitter
// See https://dev.twitter.com/docs/api/1.1/post/statuses/update
func (c *Client) UpdateStatus(status string, opts *Optionals) (tweet *Tweet, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("status", status)
	tweet = &Tweet{}
	err = c.Call("POST", "statuses/update", opts, tweet)
	return tweet, err
}

// Returns up to 100 of the first retweets of a given tweet Id
func (c *Client) Retweets(id int64, opts *Optionals) (tweets *TweetList, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	tweets = &TweetList{}
	err = c.Call("GET", fmt.Sprintf("statuses/retweets/%d", id), opts, tweets)
	return
}

// Returns a single Tweet, specified by the id parameter.
// The Tweet's author will also be embedded within the tweet.
func (c *Client) GetStatus(id int64, opts *Optionals) (tweet *Tweet, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("id", id)
	tweet = &Tweet{}
	err = c.Call("GET", "statuses/show", opts, tweet)
	return
}

// Destroys the status specified by the required ID parameter.
// The authenticating user must be the author of the specified
// status. returns the destroyed tweet if successful
func (c *Client) DestroyStatus(id int64, opts *Optionals) (tweet *Tweet, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("id", id)
	tweet = &Tweet{}
	err = c.Call("POST", fmt.Sprintf("statuses/destroy/%d", id), opts, tweet)
	return tweet, err
}

// Retweets a tweet. Returns the original tweet with retweet details embedded.
func (c *Client) Retweet(id int64, opts *Optionals) (tweet *Tweet, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("id", id)
	tweet = &Tweet{}
	err = c.Call("POST", fmt.Sprintf("statuses/retweet/%d", id), opts, tweet)
	return tweet, err
}

// Updates the authenticating user's current status and attaches media for
// upload. In other words, it creates a Tweet with a picture attached.
func (c *Client) UpdateStatusWithMedia(status string, media *TweetMedia, opts *Optionals) (tweet *Tweet, err error) {
	if opts == nil {
		opts = NewOptionals()
	}

	body := bytes.NewBufferString("")
	mp := multipart.NewWriter(body)
	mp.WriteField("status", status)
	for n, v := range opts.Values {
		mp.WriteField(n, v[0])
	}
	writer, err := mp.CreateFormFile("media[]", media.Filename)
	if err != nil {
		return nil, err
	}
	writer.Write(media.Data)
	header := fmt.Sprintf("multipart/form-data;boundary=%v", mp.Boundary())
	mp.Close()

	endpoint := fmt.Sprintf("%s/statuses/update_with_media.json?%s", apiURL, opts.Values.Encode())
	req, _ := http.NewRequest("POST", endpoint, body)
	req.Header.Set("Content-Type", header)
	res, err := c.client.Do(req)
	if err != nil {
		return
	}
	if err = checkResponse(res); err != nil {
		return
	}
	if err = json.NewDecoder(res.Body).Decode(tweet); err != nil {
		return
	}
	return

}

// Returns the current configuration used by Twitter including twitter.com
// slugs which are not usernames, maximum photo resolutions, and t.co URL
// lengths.
// See https://dev.twitter.com/docs/api/1.1/get/help/configuration
func (c *Client) Configuration() (configuration *Configuration, err error) {
	configuration = &Configuration{}
	err = c.Call("GET", "help/configuration", nil, configuration)
	return
}

// Returns Twitter's Privacy Policy
// https://dev.twitter.com/docs/api/1.1/get/help/privacy
func (c *Client) PrivacyPolicy() (privacyPolicy string, err error) {
	type pp struct {
		Text string `json:"privacy"`
	}
	ret := &pp{}
	err = c.Call("GET", "help/privacy", nil, ret)
	privacyPolicy = ret.Text
	return
}

// Returns Twitter's terms of service
// https://dev.twitter.com/docs/api/1.1/get/help/tos
func (c *Client) Tos() (string, error) {
	type tos struct {
		Text string `json:"tos"`
	}
	ret := &tos{}
	err := c.Call("GET", "help/tos", nil, ret)
	return ret.Text, err
}

// Returns Twitter's terms of service
// https://dev.twitter.com/docs/api/1.1/get/help/tos
func (c *Client) Limits() (limits *Limits, err error) {
	limits = &Limits{}
	err = c.Call("GET", "application/rate_limit_status", nil, limits)
	return
}

// Returns the 20 most recent direct messages sent to the authenticating user.
// Includes detailed information about the sender and recipient user. You can
// request up to 200 direct messages per call, up to a maximum of 800 incoming DMs
// See https://dev.twitter.com/docs/api/1.1/get/direct_messages
func (c *Client) DMList(opts *Optionals) (messages *MessageList, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	messages = &MessageList{}
	err = c.Call("GET", "direct_messages", opts, messages)
	return
}

// Returns the 20 most recent direct messages sent by the authenticating user.
// Includes detailed information about the sender and recipient user. You can
// request up to 200 direct messages per call, up to a maximum of 800 outgoing DMs.
// See https://dev.twitter.com/docs/api/1.1/get/direct_messages/sent
func (c *Client) DMSent(opts *Optionals) (messages *MessageList, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	messages = &MessageList{}
	err = c.Call("GET", "direct_messages/sent", opts, messages)
	return
}

// Returns a single direct message, specified by an id parameter.
// See https://dev.twitter.com/docs/api/1.1/get/direct_messages/show
func (c *Client) DM(id int64, opts *Optionals) (message *Message, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("id", id)
	message = &Message{}
	err = c.Call("GET", "direct_messages/show", opts, message)
	return
}

// Destroys the direct message specified in the required ID parameter.
// The authenticating user must be the recipient of the specified direct
// message.
// See https://dev.twitter.com/docs/api/1.1/post/direct_messages/destroy
func (c *Client) DMDestroy(id int64, opts *Optionals) (message *Message, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("id", id)
	message = &Message{}
	err = c.Call("POST", "direct_messages/show", opts, message)
	return
}

// Sends a new direct message to the specified user from the authenticating user.
// See https://dev.twitter.com/docs/api/1.1/post/direct_messages/new
func (c *Client) DMSend(screenname, text string, opts *Optionals) (message *Message, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("screen_name", screenname)
	opts.Add("text", text)
	message = &Message{}
	err = c.Call("POST", "direct_messages/new", opts, message)
	return
}

// Provides a simple, relevance-based search interface to public user accounts
// on Twitter. Try querying by topical interest, full name, company name,
// location, or other criteria. Exact match searches are not supported.
// See https://dev.twitter.com/docs/api/1.1/get/users/search
func (c *Client) Search(q string, opts *Optionals) (tweets *TweetList, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	opts.Add("q", q)
	tweets = &TweetList{}
	err = c.Call("GET", "users/search", opts, tweets)
	return
}

// Returns settings (including current trend, geo and sleep time information)
// for the authenticating user
// See https://dev.twitter.com/docs/api/1.1/get/account/settings
func (c *Client) AccountSettings() (settings *AccountSettings, err error) {
	settings = &AccountSettings{}
	err = c.Call("GET", "account/settings", nil, settings)
	return
}

// Helper function to verify if credentials are valid. Returns the
// user object if they are.
// See https://dev.twitter.com/docs/api/1.1/get/account/verify_credentials
func (c *Client) VerifyCredentials(opts *Optionals) (user *User, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	user = &User{}
	err = c.Call("GET", "account/verify_credentials", opts, user)
	return
}

// Update authenticating user's settings.
// See https://dev.twitter.com/docs/api/1.1/post/account/settings
func (c *Client) UpdateSettings(opts *Optionals) (newSettings *AccountSettings, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	newSettings = &AccountSettings{}
	err = c.Call("POST", "account/settings", opts, newSettings)
	return
}

// Enables/disables SMS delivery
// See https://dev.twitter.com/docs/api/1.1/post/account/update_delivery_device
func (c *Client) EnableSMS(enable bool) (err error) {
	opts := NewOptionals()
	if enable {
		opts.Add("device", "sms")
	} else {
		opts.Add("device", "none")
	}
	err = c.Call("POST", "account/update_delivery_device", opts, nil)
	return
}

// Sets values that users are able to set under the "Account" tab of their
// settings page. Only the parameters specified will be updated.
// See https://dev.twitter.com/docs/api/1.1/post/account/update_profile
func (c *Client) UpdateProfile(opts *Optionals) (user *User, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	user = &User{}
	err = c.Call("POST", "account/update_profile", opts, user)
	return
}

// Updates the authenticating user's profile background image.
// Passing an empty []byte as image will disable the current
// background image.
// https://dev.twitter.com/docs/api/1.1/post/account/update_profile_background_image
func (c *Client) UpdateProfileBackgroundImage(image []byte, opts *Optionals) (user *User, err error) {
	if opts == nil {
		opts = NewOptionals()
	}
	if len(image) > 0 {
		opts.Add("image", base64.StdEncoding.EncodeToString(image))
		opts.Add("use", true)
	} else {
		opts.Add("use", false)
	}
	user = &User{}
	err = c.Call("POST", "account/update_profile_background_image", opts, user)
	return

}
