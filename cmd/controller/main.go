package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	version = "dev"
)

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.Parse()

	if showVersion {
		fmt.Printf("kube-booster controller version: %s\n", version)
		os.Exit(0)
	}

	fmt.Println("kube-booster controller starting...")
	// TODO: Implement controller logic
}
