package main

import (
	"os"

	"github.com/joho/godotenv"

	"github.com/rizbud/meta-ads-cli/cmd"
)

func main() {
	_ = godotenv.Load()
	if err := cmd.NewRoot(nil).Execute(); err != nil {
		os.Exit(1)
	}
}
