package yggdrasil

import (
	"fmt"
	"github.com/avast/retry-go"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/sourcegraph/conc"
	"strings"
	"time"
)

const (
	retryInterval = 100 * time.Millisecond
	maxRetryTimes = 3
)

type muxServer struct {
	subServers []Server
}

func (m muxServer) Name() string {
	return "muxServer[" + strings.Join(lo.Map(m.subServers, func(item Server, _ int) string {
		return item.Name()
	}), ", ") + "]"
}

func (m muxServer) HasJoined(username string, serverID string) (*HasJoinedResponse, error) {
	var wg conc.WaitGroup
	type Ret = *HasJoinedResponse
	results := make(chan mo.Result[Ret], len(m.subServers))
	for _, s := range m.subServers {
		s := s
		wg.Go(func() {
			var resp *HasJoinedResponse
			err := retry.Do(
				func() error {
					var err error
					resp, err = s.HasJoined(username, serverID)
					return err
				},
				retry.Delay(retryInterval),
				retry.Attempts(maxRetryTimes),
			)
			if err != nil {
				results <- mo.Err[Ret](fmt.Errorf("call hasJoined on server `%v` failed: %w",
					s.Name(), err))
			} else {
				results <- mo.Ok[Ret](resp)
			}
		})
	}

	chPanic := make(chan error, 1)
	go func() {
		// wait for all async tasks to finish in another async goroutine
		// to allow the main request return as early as possible
		r := wg.WaitAndRecover()
		close(results)
		if r != nil {
			chPanic <- r.AsError()
		}
		close(chPanic)
	}()

	var last *Ret
	for r := range results {
		r, err := r.Get()
		if err != nil {
			log.Error().Err(err).Msg("hasJoined failed")
			continue
		}
		last = &r
		// return the first positive result
		if r.HasJoined() {
			return r, nil
		}
	}
	if last == nil {
		// no data generated, all async tasks panicked
		err := <-chPanic
		log.Error().Err(err).Msg("all hasJoined async query panicked")
		return nil, err
	} else {
		return *last, nil
	}
}

func NewMuxServer(servers ...Server) Server {
	return muxServer{
		subServers: servers,
	}
}
