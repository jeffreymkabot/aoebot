package aoebot

import (
	"encoding/json"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"regexp"
	"strings"
	// "sync"
)

type Driver interface {
	Actions(*Environment) []Action
}

type aoebotDriver struct {
	// mu    sync.Mutex
	dbURL string
	mongo *mgo.Session
}

func newAoebotDriver(dbURL string) (a *aoebotDriver) {
	a = &aoebotDriver{
		dbURL: dbURL,
	}
	return a
}

func (a *aoebotDriver) wakeup() (err error) {
	// a.mu.Lock()
	// defer a.mu.Unlock()
	a.mongo, err = mgo.Dial(a.dbURL)
	return
}

func (a *aoebotDriver) sleep() {
	a.mongo.Close()
}

func (a *aoebotDriver) Actions(env *Environment) []Action {
	actions := []Action{}
	conditions := []Condition{}
	coll := a.mongo.DB("aoebot").C("conditions")
	query := query(env)
	jsonQuery, _ := json.Marshal(query)
	log.Printf("Using query %s", jsonQuery)
	err := coll.Find(query).All(&conditions)
	if err != nil {
		log.Printf("Error in query %v", err)
	}
	for _, c := range conditions {
		if c.RegexPhrase != "" && env.TextMessage != nil {
			if regexp.MustCompile(c.RegexPhrase).MatchString(strings.ToLower(env.TextMessage.Content)) {
				actions = append(actions, c.Action.Action)
			}
		} else {
			actions = append(actions, c.Action.Action)
		}
	}
	log.Printf("Found actions %v", actions)
	return actions
}

func query(env *Environment) bson.M {
	and := []bson.M{
		bson.M{
			"enabled": true,
		},
	}
	if env.Guild != nil {
		and = append(and, emptyOrEqual("guild", env.Guild.ID))
	}
	if env.Author != nil {
		and = append(and, emptyOrEqual("user", env.Author.ID))
	}
	if env.TextChannel != nil {
		and = append(and, emptyOrEqual("textChannel", env.TextChannel.ID))
	}
	if env.VoiceChannel != nil {
		and = append(and, emptyOrEqual("textChannel", env.VoiceChannel.ID))
	}
	phrase := ""
	if env.TextMessage != nil {
		phrase = strings.ToLower(env.TextMessage.Content)
	}
	and = append(and, emptyOrEqual("phrase", phrase))
	return bson.M{
		"type": env.Type,
		"$and": and,
	}
}

func emptyOrEqual(field string, value interface{}) bson.M {
	return bson.M{
		field: bson.M{
			"$in": []interface{}{
				value,
				nil,
			},
		},
	}
}
