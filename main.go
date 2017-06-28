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

var me *Bot

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
	var owner string
	flag.StringVar(&token, "t", "", "Auth Token")
	flag.StringVar(&owner, "o", "", "Admin User ID")
	flag.Parse()
	if token == "" || owner == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	err := prepare()
	if err != nil {
		log.Fatalf("Error in prepare: %#v\n", err)
	}

	me = NewBot(token, owner)

	err = me.Wakeup()
	if err != nil {
		log.Fatalf("Error in wakeup: %#v\n", err)
	}
	defer me.Die()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c // wait on ctrl-C to prop open main thread
}
