package main

import (
	"os"

	"github.com/thetherington/bravobeat/cmd"

	_ "github.com/thetherington/bravobeat/include"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
