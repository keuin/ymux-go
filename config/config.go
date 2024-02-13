package config

import (
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"net/url"
	"os"
)

type Config struct {
	Servers []struct {
		Name   string `toml:"name"`
		Prefix string `toml:"prefix"`
		Proxy  string `toml:"proxy"`
	} `toml:"servers"`
	Debug   bool   `toml:"debug"`
	Listen  string `toml:"listen"`
	Metrics struct {
		Enabled bool `toml:"enabled"`
	} `toml:"metrics"`
}

func (c Config) Validate() error {
	if len(c.Servers) == 0 {
		return errors.New("no yggdrasil server specified")
	}
	for _, s := range c.Servers {
		if s.Prefix == "" {
			return fmt.Errorf("missing prefix for yggdrasil server `%v`", s.Name)
		}
		_, err := url.Parse(s.Prefix)
		if err != nil {
			return fmt.Errorf("invalid prefix for yggdrasil server `%v`: %w", s.Name, err)
		}
	}
	return nil
}

func Read(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	dec := toml.NewDecoder(f)
	var c Config
	_, err = dec.Decode(&c)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &c, nil
}
