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

// ContextType indicates the source of a context
type ContextType int

const (
	message ContextType = iota
	voicestate
	adhoc
)

// Context captures an environment that can elicit bot actions
// TODO more generic to support capturing the Context of more events
type Context struct {
	Type         ContextType
	Guild        *discordgo.Guild
	TextChannel  *discordgo.Channel
	TextMessage  *discordgo.Message
	VoiceChannel *discordgo.Channel
	Author       *discordgo.User
}

// NewContext creates a new environment based on a seed event/trigger
func NewContext(seed interface{}) (ctx *Context, err error) {
	ctx = &Context{}
	switch s := seed.(type) {
	case *discordgo.Message:
		ctx.Type = message
		ctx.TextMessage = s
		ctx.Author = s.Author
		ctx.TextChannel, err = me.session.State.Channel(s.ChannelID)
		if err != nil {
			return
		}
		ctx.Guild, err = me.session.State.Guild(ctx.TextChannel.GuildID)
		if err != nil {
			return
		}
	case *discordgo.VoiceState:
		ctx.Type = voicestate
		ctx.Author, err = me.session.User(s.UserID)
		if err != nil {
			return
		}
		ctx.VoiceChannel, err = me.session.State.Channel(s.ChannelID)
		if err != nil {
			return
		}
		ctx.Guild, err = me.session.State.Guild(ctx.VoiceChannel.GuildID)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("Unsupported type %T for context seed", s)
		return
	}
	return
}

// IsOwnContext is true when a context references the bot's own actions/behavior
// This is useful to prevent the bot from reacting to itself
func (ctx Context) IsOwnContext() bool {
	return ctx.Author != nil && ctx.Author.ID == me.self.ID
}

// Actions retrieves the list of Actions whose Condition the context satisfies
func (ctx Context) Actions() []Action {
	actions := []Action{}
	conditions := []Condition{}
	coll := me.mongo.DB("aoebot").C("conditions")
	query := ctx.query()
	jsonQuery, _ := json.Marshal(query)
	log.Printf("Using query %s", jsonQuery)
	err := coll.Find(query).All(&conditions)
	if err != nil {
		log.Printf("Error in query %v", err)
	}
	for _, c := range conditions {
		if c.RegexPhrase != "" && ctx.TextMessage != nil {
			if regexp.MustCompile(c.RegexPhrase).MatchString(strings.ToLower(ctx.TextMessage.Content)) {
				actions = append(actions, c.Action.Action)
			}
		} else {
			actions = append(actions, c.Action.Action)
		}
	}
	log.Printf("Found actions %v", actions)
	return actions
}

func (ctx Context) query() bson.M {
	and := []bson.M{}
	if ctx.Guild != nil {
		and = append(and, emptyOrEqual("guild", ctx.Guild.ID))
	}
	if ctx.Author != nil {
		and = append(and, emptyOrEqual("user", ctx.Author.ID))
	}
	if ctx.TextChannel != nil {
		and = append(and, emptyOrEqual("textChannel", ctx.TextChannel.ID))
	}
	if ctx.VoiceChannel != nil {
		and = append(and, emptyOrEqual("textChannel", ctx.VoiceChannel.ID))
	}
	phrase := ""
	if ctx.TextMessage != nil {
		phrase = strings.ToLower(ctx.TextMessage.Content)
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
		"type": ctx.Type,
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

// Satisfies is true when the environment described in a Context meets the requirements defined in a Condition
// Some conditions are more specific than others
func (ctx Context) Satisfies(c Condition) bool {
	typeMatch := ctx.Type == c.ContextType
	guildMatch := c.GuildID == "" || (ctx.Guild != nil && ctx.Guild.ID == c.GuildID)
	userMatch := c.UserID == "" || (ctx.Author != nil && ctx.Author.ID == c.UserID)
	textChannelMatch := c.TextChannelID == "" || (ctx.TextChannel != nil && ctx.TextChannel.ID == c.TextChannelID)
	voiceChannelMatch := c.VoiceChannelID == "" || (ctx.VoiceChannel != nil && ctx.VoiceChannel.ID == c.VoiceChannelID)
	ctxPhrase := ""
	if ctx.TextMessage != nil {
		ctxPhrase = strings.ToLower(ctx.TextMessage.Content)
	}
	phraseMatch := c.Phrase == "" || ctxPhrase == c.Phrase
	regexMatch := c.RegexPhrase == "" || regexp.MustCompile(c.RegexPhrase).MatchString(ctxPhrase)
	return typeMatch && guildMatch && userMatch && textChannelMatch && voiceChannelMatch && phraseMatch && regexMatch
}
