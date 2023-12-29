// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import (
	"time"
)

type Node struct {
	Period  time.Duration `config:"period"`
	Address string        `config:"address"`
	Metrics []string      `config:"metrics"`
}

type Config struct {
	Nodes []Node `config:"nodes"`
}

var DefaultConfig = Config{
	Nodes: []Node{
		{
			30 * time.Second,
			"127.0.0.1:9003",
			[]string{"CPU", "memory"},
		},
	},
}
