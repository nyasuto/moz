package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nyasuto/moz/internal/api"
)

func main() {
	var (
		port     = flag.String("port", "8080", "Port to run the server on")
		dataPath = flag.String("data", "moz.bin", "Path to the data file")
		help     = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		fmt.Println("moz-server - REST API server for Moz KV store")
		fmt.Println("\nUsage:")
		fmt.Println("  moz-server [options]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	server := api.NewServer(*dataPath, *port)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
