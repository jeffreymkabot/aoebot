package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/BurntSushi/toml"
	"github.com/jeffreymkabot/aoebot"
	"github.com/jeffreymkabot/aoebot/commands"
)

func main() {
	cfgFile := flag.String("cfg", "config.toml", "Config File Path")
	flag.Parse()
	if *cfgFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	var cfg struct {
		Token string
		Mongo string
		Owner string
		Bot   aoebot.Config
	}
	_, err := toml.DecodeFile(*cfgFile, &cfg)
	if err != nil {
		log.Fatalf("failed to open cfg file: %v", err)
	}

	signalCh := make(chan os.Signal, 2)
	signal.Notify(signalCh, os.Interrupt, os.Kill)

	bot, err := aoebot.New(cfg.Token, cfg.Mongo, cfg.Owner, signalCh)
	if err != nil {
		log.Fatalf("failed to initialize %v", err)
	}
	bot.WithConfig(cfg.Bot)
	bot.AddCommand(
		&commands.Aoe2{},
		&commands.Memes{},
		&commands.AddChannel{},
		&commands.AddReact{},
		&commands.DelReact{},
		&commands.AddWrite{},
		&commands.DelWrite{},
		&commands.AddVoice{},
		&commands.DelVoice{},
		&commands.AddGame{},
		&commands.ListGame{},
		&commands.IPlay{},
		&commands.IDontPlay{},
		&commands.Roll{},
		&commands.Source{},
		&commands.TestAction{},
	)

	if err := bot.Start(); err != nil {
		log.Fatalf("failed to start %v", err)
	}
	// bot.Stop() will not be executed if the program exits with os.Exit()
	defer bot.Stop()

	// block on a signal raised by os or internally (i.e. via aoebot.Shutdown.Run)
	// handle SIGKILL by exiting immediately without executing deferred statements
	sig := <-signalCh
	switch sig {
	case os.Interrupt:
		return
	case os.Kill:
		os.Exit(1)
	}
}
