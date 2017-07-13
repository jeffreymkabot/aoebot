package main

import (
	"flag"
	"github.com/jeffreymkabot/aoebot"
	"log"
	"os"
	"os/signal"
)

func main() {
	token := flag.String("t", "", "Auth Token")
	owner := flag.String("o", "", "Admin User ID")
	dbURL := flag.String("m", "", "MongoDB URL")
	flag.Parse()
	if *token == "" || *owner == "" || *dbURL == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	driver, err := aoebot.NewDefaultDriver(*dbURL)
	if err != nil {
		log.Fatalf("Error in start default driver: %v", err)
	}
	defer driver.Close()

	bot, err := aoebot.New(*token, *owner, driver)
	if err != nil {
		log.Fatalf("Error in create bot: %v", err)
	}
	bot.SetDriver(driver)

	err = bot.Wakeup()
	if err != nil {
		log.Fatalf("Error in wakeup: %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	select {
	case signal := <-c:
		if signal != os.Kill {
			bot.Sleep()
		}
	case <-bot.Killed():
		// treat force quit like SIGKILL
		if bot.Killer() != aoebot.ErrForceQuit {
			bot.Sleep()
		}
	}
}
