// Copyright (c) 2019 Cyro Dubeux. License MIT.

// Package tmdb is a wrapper for working with TMDb API.
package tmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// TMDb constants
const (
	baseURL           = "https://api.themoviedb.org/3"
	permissionURL     = "https://www.themoviedb.org/authenticate/"
	authenticationURL = "/authentication/"
	movieURL          = "/movie/"
	tvURL             = "/tv/"
	tvSeasonURL       = "/season/"
	tvEpisodeURL      = "/episode/"
	personURL         = "/person/"
	searchURL         = "/search/"
	collectionURL     = "/collection/"
	companyURL        = "/company/"
	configurationURL  = "/configuration/"
	creditURL         = "/credit/"
	discoverURL       = "/discover/"
	networkURL        = "/network/"
	keywordURL        = "/keyword/"
	genreURL          = "/genre/"
	guestSessionURL   = "/guest_session/"
	listURL           = "/list/"
	accountURL        = "/account/"
)

// Client type is a struct to instantiate this pkg.
type Client struct {
	// TMDb apiKey to use the client.
	apiKey string
	// sessionId to use the client.
	sessionID string
	// Auto retry flag to indicates if the client
	// should retry the previous operation.
	autoRetry bool
	// withContext flag enables the request with context.
	withContext bool
	// http.Client for custom configuration.
	http http.Client
}

// Error type represents an error returned by the TMDB API.
type Error struct {
	StatusMessage string `json:"status_message,omitempty"`
	Success       bool   `json:"success,omitempty"`
	StatusCode    int    `json:"status_code,omitempty"`
}

// Response type is a struct for http responses.
type Response struct {
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
}

// Init setups the Client with an apiKey.
func Init(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("APIKey is empty")
	}

	return &Client{apiKey: apiKey}, nil
}

// SetSessionID will set the session id.
func (c *Client) SetSessionID(sid string) error {
	if sid == "" {
		return errors.New("The SessionID is empty")
	}

	c.sessionID = sid

	return nil
}

// SetClientConfig sets a custom configuration for the http.Client.
func (c *Client) SetClientConfig(httpClient http.Client) {
	c.http = httpClient
}

// SetClientAutoRetry sets autoRetry flag to true.
func (c *Client) SetClientAutoRetry() {
	c.autoRetry = true
}

// SetClientWithContext enables the request with context.
func (c *Client) SetClientWithContext() {
	c.withContext = true
}

// Auto retry default duration.
const defaultRetryDuration = time.Second * 5

// retryDuration calculates the retry duration time.
func retryDuration(resp *http.Response) time.Duration {
	retryTime := resp.Header.Get("Retry-After")

	if retryTime == "" {
		return defaultRetryDuration
	}

	seconds, err := strconv.ParseInt(retryTime, 10, 32)

	if err != nil {
		return defaultRetryDuration
	}

	return time.Duration(seconds) * time.Second
}

// shouldRetry determines whether the status code indicates that the
// previous operation should be retried at a later time.
func shouldRetry(status int) bool {
	return status == http.StatusAccepted || status == http.StatusTooManyRequests
}

func (c *Client) get(url string, data interface{}) error {
	if url == "" {
		return errors.New("url field is empty")
	}

	if c.http.Timeout == 0 {
		c.http.Timeout = time.Second * 10
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if c.withContext {
		req, err = http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			url,
			nil,
		)
	}
	if err != nil {
		return fmt.Errorf("could not fetch the url: %s", err)
	}

	req.Header.Add("content-type", "application/json;charset=utf-8")

	for {
		res, err := c.http.Do(req)

		if err != nil {
			return err
		}

		defer res.Body.Close()

		if res.StatusCode == http.StatusTooManyRequests && c.autoRetry {
			time.Sleep(retryDuration(res))
			continue
		}

		if res.StatusCode == http.StatusNoContent {
			return nil
		}

		if res.StatusCode != http.StatusOK {
			return c.decodeError(res)
		}

		if err = json.NewDecoder(res.Body).Decode(data); err != nil {
			return fmt.Errorf("could not decode the data: %s", err)
		}

		break
	}

	return nil
}

func (c *Client) request(
	url string,
	body interface{},
	method string,
	data interface{},
) error {
	if url == "" {
		return errors.New("url field is empty")
	}

	if c.http.Timeout == 0 {
		c.http.Timeout = time.Second * 10
	}

	bodyBytes := new(bytes.Buffer)
	json.NewEncoder(bodyBytes).Encode(body)

	req, err := http.NewRequest(
		method,
		url,
		bytes.NewBuffer(bodyBytes.Bytes()),
	)
	if c.withContext {
		req, err = http.NewRequestWithContext(
			context.Background(),
			method,
			url,
			bytes.NewBuffer(bodyBytes.Bytes()),
		)
	}
	if err != nil {
		return fmt.Errorf("could not fetch the url: %s", err)
	}

	req.Header.Add("content-type", "application/json;charset=utf-8")

	for {
		res, err := c.http.Do(req)
		if err != nil {
			return errors.New(err.Error())
		}

		defer res.Body.Close()

		if c.autoRetry && shouldRetry(res.StatusCode) {
			time.Sleep(retryDuration(res))
			continue
		}

		// Checking if the response is greater or equal
		// to 300 or less than 200.
		if res.StatusCode >= http.StatusMultipleChoices ||
			res.StatusCode < http.StatusOK ||
			res.StatusCode == http.StatusNoContent {
			return c.decodeError(res)
		}

		if err = json.NewDecoder(res.Body).Decode(data); err != nil {
			return fmt.Errorf("could not decode the data: %s", err)
		}

		break
	}
	return nil
}

func (c *Client) fmtOptions(
	urlOptions map[string]string,
) string {
	options := ""

	if len(urlOptions) > 0 {
		for key, value := range urlOptions {
			options += fmt.Sprintf(
				"&%s=%s",
				key,
				url.QueryEscape(value),
			)
		}
	}

	return options
}

func (e Error) Error() string {
	return fmt.Sprintf(
		"code: %d | success: %t | message: %s",
		e.StatusCode,
		e.Success,
		e.StatusMessage,
	)
}

func (c *Client) decodeError(r *http.Response) error {
	resBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("could not read body response: %s", err)
	}

	if len(resBody) == 0 {
		return fmt.Errorf(
			"[%d]: empty body %s",
			r.StatusCode,
			http.StatusText(r.StatusCode),
		)
	}

	buf := bytes.NewBuffer(resBody)
	var clientError Error

	if err := json.NewDecoder(buf).Decode(&clientError); err != nil {
		return fmt.Errorf(
			"couldn't decode error: (%d) [%s]",
			len(resBody),
			resBody,
		)
	}

	return clientError
}
