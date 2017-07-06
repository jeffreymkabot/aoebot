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

	me := NewBot(*token, *owner, *dbURL)

	err := me.Wakeup()
	if err != nil {
		log.Fatalf("Error in wakeup: %#v\n", err)
	}
	defer me.Die()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c // wait on ctrl-C to prop open main thread
}
