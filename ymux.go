package main

import (
	"fmt"
	"github.com/akamensky/argparse"
	"github.com/gin-gonic/gin"
	"github.com/keuin/ymux-go/config"
	"github.com/keuin/ymux-go/instrument"
	"github.com/keuin/ymux-go/yggdrasil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"regexp"
	"strings"
	"time"
)

var processStartTime = time.Now()

const applicationJson = "application/json"

func main() {
	p := argparse.NewParser("ymux-go", "Minecraft Yggdrasil server mux")
	configFile := p.String("c", "config", &argparse.Options{
		Required: true,
		Help:     "path to the config file",
	})
	err := p.Parse(os.Args)
	if err != nil {
		fmt.Print(p.Usage(err))
		return
	}

	cfg, err := config.Read(*configFile)
	if err != nil {
		panic(fmt.Errorf("error reading config file: %w", err))
	}
	err = cfg.Validate()
	if err != nil {
		panic(fmt.Errorf("config validation failed: %w", err))
	}
	ss, err := createServers(cfg)
	if err != nil {
		panic(err)
	}
	s := yggdrasil.NewMuxServer(ss...)

	if cfg.Debug {
		log.Logger = log.Logger.Level(zerolog.DebugLevel)
	} else {
		log.Logger = log.Logger.Level(zerolog.InfoLevel)
		gin.SetMode(gin.ReleaseMode)
	}
	log.Debug().Msg("debug mode is enabled")
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/", func(c *gin.Context) {
		// servers with authlib-injector will call this API on boot
		// we need this to make them happy
		c.Data(200, applicationJson, []byte(`{}`))
	})
	handlers := []gin.HandlerFunc{func(c *gin.Context) {
		var args struct {
			Username string `form:"username"`
			ServerID string `form:"serverId"`
		}
		err := c.ShouldBindQuery(&args)
		if err != nil {
			_ = c.AbortWithError(400, err)
			instrument.SetInstrument(c, instrument.RequestInfo{
				Success:  false,
				Username: args.Username,
				ServerID: args.ServerID,
			})
			return
		}
		r, err := s.HasJoined(args.Username, args.ServerID)
		if err != nil {
			log.Error().Err(err).Msg("ymux hasJoined API failed")
			_ = c.AbortWithError(500, err)
			instrument.SetInstrument(c, instrument.RequestInfo{
				Success:  false,
				Username: args.Username,
				ServerID: args.ServerID,
			})
			return
		}
		if r == nil {
			log.Error().Msg("ymux hasJoined API returns nil response")
			_ = c.AbortWithError(500, fmt.Errorf("ymux hasJoined API returns nil response"))
			instrument.SetInstrument(c, instrument.RequestInfo{
				Success:  false,
				Username: args.Username,
				ServerID: args.ServerID,
			})
			return
		}
		log.Info().
			Str("username", args.Username).
			Str("serverId", args.ServerID).
			Str("yggdrasilServer", r.ServerName).
			Bool("hasJoined", r.HasJoined()).
			Msg("ymux hasJoined API OK")
		instrument.SetInstrument(c, instrument.RequestInfo{
			Success:  true,
			Username: args.Username,
			ServerID: args.ServerID,
			LoggedIn: r.HasJoined(),
		})
		if r.HasJoined() {
			c.Data(200, applicationJson, r.RawBody)
			return
		}
		c.Status(204)
	}}

	// setup prometheus metrics exporter
	if cfg.Metrics.Enabled {
		reg := prometheus.NewRegistry()
		reg.MustRegister(
			collectors.NewBuildInfoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			collectors.NewGoCollector(
				collectors.WithGoCollectorRuntimeMetrics(
					collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")},
				),
			),
		)
		r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{
			ErrorLog:            promZeroLogger{},
			ErrorHandling:       promhttp.HTTPErrorOnError,
			Registry:            nil,
			DisableCompression:  true,
			MaxRequestsInFlight: 0,
			Timeout:             0,
			EnableOpenMetrics:   true,
			ProcessStartTime:    processStartTime,
		})))
		ex := instrument.NewExporter(reg)
		handlers = append([]gin.HandlerFunc{ex.Instrument}, handlers...)
		log.Info().Msg("prometheus metrics exporter enabled")
	}
	r.GET("/sessionserver/session/minecraft/hasJoined", handlers...)

	err = r.Run(cfg.Listen)
	if err != nil {
		panic(fmt.Errorf("error running http server: %w", err))
	}
}

type promZeroLogger struct {
}

func (p promZeroLogger) Println(v ...interface{}) {
	log.Error().Str("msg", fmt.Sprintln(v...)).Msg("prometheus metrics exporter error")
}

func createServers(cfg *config.Config) ([]yggdrasil.Server, error) {
	var servers []yggdrasil.Server
	for _, s := range cfg.Servers {
		s.Prefix = strings.TrimRight(s.Prefix, "/")
		ys, err := yggdrasil.NewServer(s.Prefix, yggdrasil.NewServerOptions{
			Name:  s.Name,
			Proxy: s.Proxy,
		})
		if err != nil {
			return nil, fmt.Errorf("parse server `%v`: %w", s.Name, err)
		}
		servers = append(servers, ys)
	}
	return servers, nil
}
