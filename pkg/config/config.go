package config

import (
	"os"

	yaml "gopkg.in/yaml.v3"
)

var Cfg []Config

// UnmarshalYAML implements YAML unmarshalleer to set default values in the config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type ConfigDefaulted Config
	defaultCfg := ConfigDefaulted{
		Warmup: false, // Disable warmup by default
	}
	if err := unmarshal(&defaultCfg); err != nil {
		return err
	}
	*c = Config(defaultCfg)
	return nil
}

func Load(cfg string) error {
	f, err := os.Open(cfg)
	if err != nil {
		return err
	}
	data := yaml.NewDecoder(f)
	data.KnownFields(true)
	err = data.Decode(&Cfg)
	return err
}
