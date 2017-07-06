package main

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gopkg.in/mgo.v2/bson"
	"log"
	"regexp"
	"strings"
)

// EnvironmentType indicates the source of a context
type EnvironmentType int

const (
	message EnvironmentType = iota
	voicestate
	adhoc
)

// Environment captures an environment that can elicit bot actions
// TODO more generic to support capturing the Environment of more events
type Environment struct {
	Type         EnvironmentType
	Guild        *discordgo.Guild
	TextChannel  *discordgo.Channel
	TextMessage  *discordgo.Message
	VoiceChannel *discordgo.Channel
	Author       *discordgo.User
}

// NewEnvironment creates a new environment based on a seed event/trigger
func NewEnvironment(seed interface{}) (env *Environment, err error) {
	env = &Environment{}
	switch s := seed.(type) {
	case *discordgo.Message:
		env.Type = message
		env.TextMessage = s
		env.Author = s.Author
		env.TextChannel, err = me.session.State.Channel(s.ChannelID)
		if err != nil {
			return
		}
		env.Guild, err = me.session.State.Guild(env.TextChannel.GuildID)
		if err != nil {
			return
		}
	case *discordgo.VoiceState:
		env.Type = voicestate
		env.Author, err = me.session.User(s.UserID)
		if err != nil {
			return
		}
		env.VoiceChannel, err = me.session.State.Channel(s.ChannelID)
		if err != nil {
			return
		}
		env.Guild, err = me.session.State.Guild(env.VoiceChannel.GuildID)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("Unsupported type %T for context seed", s)
		return
	}
	return
}

// IsOwnEnvironment is true when an environment references the bot's own actions/behavior
// This is useful to prevent the bot from reacting to itself
func (env Environment) IsOwnEnvironment() bool {
	return env.Author != nil && env.Author.ID == me.self.ID
}

// Actions retrieves the list of Actions whose conditions the environment satisfies
func (env Environment) Actions() []Action {
	actions := []Action{}
	conditions := []Condition{}
	coll := me.mongo.DB("aoebot").C("conditions")
	query := env.query()
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

func (env Environment) query() bson.M {
	and := []bson.M{}
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
	// regex := bson.M{
	// 	"$or": []bson.M{
	// 		bson.M{
	// 			"regex": bson.M{"$exists": false},
	// 		},
	// 		bson.M{},
	// 	},
	// }
	// and = append(and, regex)
	return bson.M{
		"type": env.Type,
		"$and": and,
	}
}

func emptyOrEqual(field string, value interface{}) bson.M {
	return bson.M{
		"$or": []bson.M{
			bson.M{
				field: bson.M{"$exists": false},
			},
			bson.M{
				field: value,
			},
		},
	}
}

// Satisfies is true when an environment meets the requirements defined in a Condition
// Some conditions are more specific than others
func (env Environment) Satisfies(c Condition) bool {
	typeMatch := env.Type == c.EnvironmentType
	guildMatch := c.GuildID == "" || (env.Guild != nil && env.Guild.ID == c.GuildID)
	userMatch := c.UserID == "" || (env.Author != nil && env.Author.ID == c.UserID)
	textChannelMatch := c.TextChannelID == "" || (env.TextChannel != nil && env.TextChannel.ID == c.TextChannelID)
	voiceChannelMatch := c.VoiceChannelID == "" || (env.VoiceChannel != nil && env.VoiceChannel.ID == c.VoiceChannelID)
	envPhrase := ""
	if env.TextMessage != nil {
		envPhrase = strings.ToLower(env.TextMessage.Content)
	}
	phraseMatch := c.Phrase == "" || envPhrase == c.Phrase
	regexMatch := c.RegexPhrase == "" || regexp.MustCompile(c.RegexPhrase).MatchString(envPhrase)
	return typeMatch && guildMatch && userMatch && textChannelMatch && voiceChannelMatch && phraseMatch && regexMatch
}
