package yggdrasil

import (
	"encoding/json"
	"fmt"
	"github.com/imroc/req"
	"github.com/rs/zerolog/log"
	"net/url"
	"time"
)

type realServer struct {
	req       *req.Req
	apiPrefix string
	name      string
}

func (r realServer) Name() string {
	return r.name
}

func (r realServer) HasJoined(username string, serverID string) (*HasJoinedResponse, error) {
	u, err := url.Parse(r.apiPrefix + "/session/minecraft/hasJoined")
	if err != nil {
		return nil, fmt.Errorf("url parse: %w", err)
	}
	q := u.Query()
	q.Set("username", username)
	q.Set("serverId", serverID)
	u.RawQuery = q.Encode()
	url2 := u.String()
	log.Debug().Str("url", url2).Msg("hasJoined request")
	resp, err := r.req.Get(url2)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	respBytes := resp.Bytes()
	var resp2 HasJoinedResponse
	// decode JSON only if HTTP status code is OK
	if resp.Response().StatusCode == 200 {
		err := json.Unmarshal(respBytes, &resp2)
		if err != nil {
			log.Error().
				Str("body", string(respBytes)).
				Err(err).
				Msg("unmarshal response body JSON failed")
		}
	}
	resp2.StatusCode = resp.Response().StatusCode
	resp2.RawBody = respBytes
	resp2.ServerName = r.name
	log.Debug().
		Int("statusCode", resp2.StatusCode).
		Str("rawBody", string(respBytes)).
		Msg("hasJoined response")
	return &resp2, nil
}

func NewServer(apiPrefix string, opt ...NewServerOptions) (Server, error) {
	name := "<unnamed server>"
	r := req.New()
	if len(opt) > 0 {
		if p := opt[0].Proxy; p != "" {
			err := r.SetProxyUrl(p)
			if err != nil {
				return nil, fmt.Errorf("set proxy url: %w", err)
			}
		}
		if t := opt[0].Timeout; t > 0 {
			r.SetTimeout(t)
		}
		if n := opt[0].Name; n != "" {
			name = n
		}
	}
	return realServer{
		req:       r,
		apiPrefix: apiPrefix,
		name:      name,
	}, nil
}

type NewServerOptions struct {
	Name  string
	Proxy string
	// Timeout is HTTP API request timeout
	Timeout time.Duration
}
