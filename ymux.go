package main

import (
	"fmt"
	"github.com/akamensky/argparse"
	"github.com/gin-gonic/gin"
	"github.com/keuin/ymux-go/config"
	"github.com/keuin/ymux-go/yggdrasil"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
)

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
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		// servers with authlib-injector will call this API on boot
		// we need this to make them happy
		c.Data(200, applicationJson, []byte(`{}`))
	})
	r.GET("/sessionserver/session/minecraft/hasJoined", func(c *gin.Context) {
		var args struct {
			Username string `form:"username"`
			ServerID string `form:"serverId"`
		}
		err := c.ShouldBindQuery(&args)
		if err != nil {
			_ = c.AbortWithError(400, err)
			return
		}
		r, err := s.HasJoined(args.Username, args.ServerID)
		if err != nil {
			log.Error().Err(err).Msg("ymux hasJoined API failed")
			_ = c.AbortWithError(500, err)
			return
		}
		log.Info().
			Str("username", args.Username).
			Str("serverId", args.ServerID).
			Str("yggdrasilServer", r.ServerName).
			Bool("hasJoined", r.HasJoined()).
			Msg("ymux hasJoined API OK")
		if r.HasJoined() {
			c.Data(200, applicationJson, r.RawBody)
			return
		}
		c.Status(204)
	})
	err = r.Run(cfg.Listen)
	if err != nil {
		panic(fmt.Errorf("error running http server: %w", err))
	}
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
