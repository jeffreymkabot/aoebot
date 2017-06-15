/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
)

var me *bot

func prepare() (err error) {
	// dynamically bind some voice actions
	err = createAoeChatCommands()
	if err != nil {
		return
	}
	log.Print("Registered aoe commands")

	err = loadVoiceActionFiles()
	if err != nil {
		return
	}
	log.Print("Loaded voice actions")

	return
}

func main() {
	var token string
	flag.StringVar(&token, "t", "", "Bot Auth Token")
	flag.Parse()
	if token == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	err := prepare()
	if err != nil {
		log.Fatalf("Error in prepare: %#v\n", err)
	}

	me = NewBot(token)

	err = me.wakeup()
	if err != nil {
		log.Fatalf("Error in wakeup: %#v\n", err)
	}
	defer me.die()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c // wait on ctrl-C to prop open main thread
}
