package main

import (
	"encoding/binary"
	_ "encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/fatih/structs"
	// "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io"
	"os"
	"regexp"
	"strings"
)

// Action can be performed given the context (environment) of its trigger
type Action interface {
	perform(ctx *Context) error
}

const (
	// MessageContext is an environment around a text message
	MessageContext = iota
	// VoiceStateContext is an environment around a voice state
	VoiceStateContext
	//
	adHocContext
)

// Context captures an environment that can elicit bot actions
// TODO more generic to support capturing the Context of more events
type Context struct {
	Guild        *discordgo.Guild
	TextChannel  *discordgo.Channel
	TextMessage  *discordgo.Message
	VoiceChannel *discordgo.Channel
	Author       *discordgo.User
	Type         int
}

// NewContext creates a new environment based on a seed event/trigger
func NewContext(seed interface{}) (ctx *Context, err error) {
	ctx = &Context{}
	switch s := seed.(type) {
	case *discordgo.Message:
		ctx.Type = MessageContext
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
		ctx.Type = VoiceStateContext
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

// Actions returns a list of actions matching a context
func (ctx Context) Actions() []Action {
	actions := []Action{}
	for _, c := range conditions {
		if ctx.Satisfies(c) {
			actions = append(actions, c.Action)
		}
	}
	return actions
}

func (ctx Context) ActionsQuery() []Action {
	actions := []Action{}
	coll := me.mongo.DB("aoebot").C("conditions")
	coll.Find(ctx.Query()).All(&actions)
	return actions
}

// TODO
func (ctx Context) Query() bson.M {
	return bson.M{}
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
	phraseMatch := c.Phrase == "" ||
		((!c.IsRegex && ctxPhrase == c.Phrase) || (c.IsRegex && regexp.MustCompile(c.Phrase).MatchString(ctxPhrase)))
	return typeMatch && guildMatch && userMatch && textChannelMatch && voiceChannelMatch && phraseMatch
}

// WriteAction specifies content that can be written to a text channel
type WriteAction struct {
	Content string
	TTS     bool
}

// type something to the text channel of the original context
func (wa WriteAction) perform(ctx *Context) (err error) {
	err = me.Write(ctx.TextChannel.ID, wa.Content, wa.TTS)
	return
}

func (wa WriteAction) String() string {
	if wa.TTS {
		return fmt.Sprintf("/tts %v", wa.Content)
	}
	return fmt.Sprintf("%v", wa.Content)
}

// ReactAction specifies an emoji that can be used to react to a message
type ReactAction struct {
	Emoji string
}

func (ra ReactAction) perform(ctx *Context) (err error) {
	err = me.React(ctx.TextChannel.ID, ctx.TextMessage.ID, ra.Emoji)
	return
}

func (ra ReactAction) String() string {
	return fmt.Sprintf("%x", ra.Emoji)
}

// SayAction specifies content that can be said to a voice channel
type SayAction struct {
	File   string
	buffer [][]byte
}

// say something to the voice channel of the user in the original context
func (sa SayAction) perform(ctx *Context) (err error) {
	vcID := ""
	if ctx.VoiceChannel != nil {
		vcID = ctx.VoiceChannel.ID
	} else {
		vcID = getVoiceChannelIDByContext(ctx)
	}
	if vcID == "" {
		return
	}

	err = me.Say(ctx.Guild.ID, vcID, sa.buffer)
	return
}

func getVoiceChannelIDByContext(ctx *Context) string {
	return getVoiceChannelIDByUser(ctx.Guild, ctx.Author)
}

func getVoiceChannelIDByUser(g *discordgo.Guild, u *discordgo.User) string {
	for _, vs := range g.VoiceStates {
		if vs.UserID == u.ID {
			return vs.ChannelID
		}
	}
	return ""
}

func getVoiceChannelIDByUserID(g *discordgo.Guild, uID string) string {
	for _, vs := range g.VoiceStates {
		if vs.UserID == uID {
			return vs.ChannelID
		}
	}
	return ""
}

// need to use pointer receiver so the load method can modify the voiceAction's internal byte buffer
func (sa *SayAction) load() error {
	sa.buffer = make([][]byte, 0)
	file, err := os.Open(sa.File)
	if err != nil {
		return err
	}
	defer file.Close()

	var opuslen int16

	for {
		err = binary.Read(file, binary.LittleEndian, &opuslen)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}

		inbuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &inbuf)

		if err != nil {
			return err
		}

		sa.buffer = append(sa.buffer, inbuf)
	}
}

func (sa SayAction) String() string {
	return fmt.Sprintf("%v", sa.File)
}

// StatsAction indicates that runtime information should be written to a text channel
type StatsAction struct {
}

func (sa StatsAction) perform(ctx *Context) (err error) {
	me.Write(ctx.TextChannel.ID, me.Stats().String(), false)
	return
}

// ReconnectVoiceAction indicates that the bot should refresh its voice worker for a guild
type ReconnectVoiceAction struct {
	Content string
}

func (rva ReconnectVoiceAction) perform(ctx *Context) (err error) {
	if rva.Content != "" {
		_ = me.Write(ctx.TextChannel.ID, rva.Content, false)
	}
	me.SpeakTo(ctx.Guild)
	return
}

func (rva ReconnectVoiceAction) String() string {
	return fmt.Sprintf("%v", rva.Content)
}

// RestartAction indicates that the bot should restart its discord session
type RestartAction struct {
	Content string
}

func (ra RestartAction) perform(ctx *Context) (err error) {
	if ra.Content != "" {
		_ = me.Write(ctx.TextChannel.ID, ra.Content, false)
	}
	me.Sleep()
	me.Wakeup()
	return
}

func (ra RestartAction) String() string {
	return fmt.Sprintf("%v", ra.Content)
}

// QuitAction indicates that the bot should terminate
type QuitAction struct {
	Content string
	Force   bool
}

func (qa QuitAction) perform(ctx *Context) (err error) {
	if qa.Content != "" {
		_ = me.Write(ctx.TextChannel.ID, qa.Content, false)
	}
	if qa.Force {
		me.ForceDie()
	} else {
		me.Die()
	}
	return
}

func (qa QuitAction) String() string {
	if qa.Force {
		return fmt.Sprintf("force %v", qa.Content)
	}
	return fmt.Sprintf("%v", qa.Content)
}
