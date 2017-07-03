/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package main

import (
	"flag"
	"gopkg.in/mgo.v2/bson"
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

func exportConditions() {
	coll := me.mongo.DB("aoebot").C("conditions")
	info, err := coll.RemoveAll(bson.M{})
	log.Printf("Removed all in conditions %#v", info)
	if err != nil {
		log.Printf("Error in remove all %v", err)
	}
	for c := range conditions {
		log.Printf("Insert condition %#v", c)
		coll.Insert(c)
		if err != nil {
			log.Printf("Error in insert condition %v", err)
		}
	}
}

func main() {
	token := flag.String("t", "", "Auth Token")
	owner := flag.String("o", "", "Admin User ID")
	dbURL := flag.String("m", "", "MongoDB URL")
	doExport := flag.Bool("e", false, "Export conditions only")
	flag.Parse()
	if *token == "" || *owner == "" || *dbURL == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	err := prepare()
	if err != nil {
		log.Fatalf("Error in prepare: %#v\n", err)
	}

	me = NewBot(*token, *owner, *dbURL)

	err = me.Wakeup()
	if err != nil {
		log.Fatalf("Error in wakeup: %#v\n", err)
	}
	defer me.Die()

	if *doExport {
		log.Println("Do export")
		exportConditions()
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c // wait on ctrl-C to prop open main thread
}
