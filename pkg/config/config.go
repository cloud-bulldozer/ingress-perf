package config

import (
	"os"

	yaml "gopkg.in/yaml.v3"
)

var Cfg []Config

func Load(cfg string) error {
	f, err := os.Open(cfg)
	if err != nil {
		return err
	}
	data := yaml.NewDecoder(f)
	err = data.Decode(&Cfg)
	return err
}
