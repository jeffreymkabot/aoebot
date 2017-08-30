package main

import (
	"flag"
	"github.com/BurntSushi/toml"
	"github.com/jeffreymkabot/aoebot"
	"log"
	"os"
	"os/signal"
)

func main() {
	cfgFile := flag.String("cfg", "config.toml", "Config File Path")
	logFile := flag.String("log", "", "Log File Path")
	flag.Parse()
	if *cfgFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	var err error

	logDst := os.Stdout
	if *logFile != "" {
		logDst, err = os.OpenFile(*logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			log.Fatalf("Error opening log file: %v", err)
		}
	}

	log := log.New(logDst, "aoebot: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)

	var cfg aoebot.Config
	_, err = toml.DecodeFile(*cfgFile, &cfg)
	if err != nil {
		log.Fatalf("Error opening cfg file: %v", err)
	}

	bot, err := aoebot.NewFromConfig(cfg, log)
	if err != nil {
		log.Fatalf("%v", err)
	}

	err = bot.Start()
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer bot.Stop()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	for {
		select {
		case sig := <-c:
			switch sig {
			case os.Interrupt:
				return
			case os.Kill:
				os.Exit(1)
			}
		case <-bot.Killed():
			switch bot.Killer() {
			case aoebot.ErrQuit:
				return
			case aoebot.ErrForceQuit:
				os.Exit(1)
			}
		}
	}
}
