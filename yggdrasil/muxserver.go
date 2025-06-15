package yggdrasil

import (
	"fmt"
	"github.com/avast/retry-go"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/sourcegraph/conc"
	"go.uber.org/multierr"
	"runtime/debug"
	"strings"
	"sync"
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

func (m muxServer) GetMinecraftProfiles(usernames []string) (GetMinecraftProfilesResponse, error) {
	wg := &sync.WaitGroup{}
	errs := make(chan error, len(m.subServers))
	results := make(chan GetMinecraftProfilesResponse, len(m.subServers))
	for _, ss := range m.subServers {
		ss := ss
		wg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errs <- fmt.Errorf("panic while querying subServer %v: %v, trace: %v",
						ss.Name(), r, string(debug.Stack()))
				}
				wg.Done()
			}()
			ret, err := ss.GetMinecraftProfiles(usernames)
			if err != nil {
				errs <- fmt.Errorf("error querying subServer %v: %v", ss.Name(), err)
				return
			}
			results <- ret
		}()
	}
	wg.Wait()
	close(errs)
	close(results)
	if err := multierr.Combine(lo.ChannelToSlice(errs)...); err != nil {
		return nil, err
	}
	return lo.Reduce(lo.ChannelToSlice(results),
		func(agg GetMinecraftProfilesResponse, item GetMinecraftProfilesResponse, _ int) GetMinecraftProfilesResponse {
			return append(agg, item...)
		}, nil), nil
}

func NewMuxServer(servers ...Server) Server {
	return muxServer{
		subServers: servers,
	}
}
