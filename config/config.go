// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Config struct {
	Period  time.Duration `config:"period"`
	Address string        `config:"address"`
	Metrics []string      `config:"metrics"`
}

var DefaultConfig = Config{
	Period:  30 * time.Second,
	Address: "10.9.0.15:9003",
	Metrics: []string{"CPU", "memory", "interfaces"},
}
