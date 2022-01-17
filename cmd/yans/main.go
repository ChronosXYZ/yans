package main

import (
	"flag"
	"github.com/ChronosX88/yans/internal/config"
	"github.com/ChronosX88/yans/internal/server"
	"log"
	"os"
	"os/signal"
)

func main() {

	configPath := flag.String("config", "", "Path to config")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("No config provided!")
	}

	cfg, err := config.ParseConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	log.Println("Starting YANS...")
	ns, err := server.NewNNTPServer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if err := ns.Start(); err != nil {
		log.Fatal(err)
	}
	log.Println("YANS has been successfully started!")

	for range c {
		log.Println("Stopping YANS...")
		ns.Stop()
		break
	}
}
