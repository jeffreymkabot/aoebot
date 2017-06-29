package main

import (
	"encoding/binary"
	_ "encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/fatih/structs"
	"io"
	"os"
	"regexp"
	"strings"
)

// Condition defines a set of requirements an environment should meet for an action to be performed on that environment
type Condition struct {
	Name string
	// e.g. MessageContext, VoiceStateContext
	ContextType int
	Phrase      string
	IsRegex     bool
	GuildID     string
	ChannelID   string
	UserID      string
	// e.g. textAction, quitAction
	ActionType int
	Action     Action
}

const (
	write = iota
	say
	react
	reconnect
	restart
	quit
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
func (ctx *Context) Actions() []Action {
	actions := []Action{}
	for _, c := range conditions {
		if ctx.Satisfies(c) {
			actions = append(actions, c.Action)
		}
	}
	return actions
}

// Satisfies is true when the environment described in a Context meets the requirements defined in a Condition
// Some conditions are more specific than others
func (ctx *Context) Satisfies(c Condition) bool {
	typeMatch := ctx.Type == c.ContextType
	guildMatch := c.GuildID == "" || (ctx.Guild != nil && ctx.Guild.ID == c.GuildID)
	userMatch := c.UserID == "" || (ctx.Author != nil && ctx.Author.ID == c.UserID)
	// textChannelMatch := c.ChannelID == "" || (ctx.textChannel != nil && ctx.channel.ID == c.ChannelID)
	// voiceChannelMatch :=
	phraseMatch :=
		c.Phrase == "" ||
			(ctx.TextMessage != nil &&
				((!c.IsRegex && ctx.TextMessage.Content == c.Phrase) ||
					(c.IsRegex && regexp.MustCompile(c.Phrase).MatchString(ctx.TextMessage.Content))))
	return typeMatch && guildMatch && userMatch && phraseMatch
}

type textAction struct {
	content string
	tts     bool
}

type emojiReactionAction struct {
	emoji string
}

type voiceAction struct {
	file   string
	buffer [][]byte
}

type reconnectVoiceAction struct {
	content string
}

type restartAction struct {
	content string
}

type quitAction struct {
	force   bool
	content string
}

// type something to the text channel of the original context
func (ta textAction) perform(ctx *Context) (err error) {
	err = me.Write(ctx.TextChannel.ID, ta.content, ta.tts)
	return
}

func (ta textAction) String() string {
	if ta.tts {
		return fmt.Sprintf("/tts %v", ta.content)
	}
	return fmt.Sprintf("%v", ta.content)
}

func (era emojiReactionAction) perform(ctx *Context) (err error) {
	err = me.React(ctx.TextChannel.ID, ctx.TextMessage.ID, era.emoji)
	return
}

func (era emojiReactionAction) String() string {
	return fmt.Sprintf("%x", era.emoji)
}

// say something to the voice channel of the user in the original context
func (va *voiceAction) perform(ctx *Context) (err error) {
	vcID := ""
	if ctx.VoiceChannel != nil {
		vcID = ctx.VoiceChannel.ID
	} else {
		vcID = getVoiceChannelIDByContext(ctx)
	}

	if vcID == "" {
		return
	}
	vp := &voicePayload{
		buffer:    va.buffer,
		channelID: vcID,
	}
	err = me.Say(vp, ctx.Guild.ID)
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
func (va *voiceAction) load() error {
	va.buffer = make([][]byte, 0)
	file, err := os.Open(va.file)
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

		va.buffer = append(va.buffer, inbuf)
	}
}

func (va voiceAction) String() string {
	return fmt.Sprintf("%v", va.file)
}

func (rva reconnectVoiceAction) perform(ctx *Context) (err error) {
	if rva.content != "" {
		_ = me.Write(ctx.TextChannel.ID, rva.content, false)
	}
	me.SpeakTo(ctx.Guild)
	return
}

func (rva reconnectVoiceAction) String() string {
	return fmt.Sprintf("%v", rva.content)
}

func (ra restartAction) perform(ctx *Context) (err error) {
	if ra.content != "" {
		_ = me.Write(ctx.TextChannel.ID, ra.content, false)
	}
	me.Sleep()
	me.Wakeup()
	return
}

func (ra restartAction) String() string {
	return fmt.Sprintf("%v", ra.content)
}

func (qa quitAction) perform(ctx *Context) (err error) {
	if qa.content != "" {
		_ = me.Write(ctx.TextChannel.ID, qa.content, false)
	}
	if qa.force {
		me.ForceDie()
	} else {
		me.Die()
	}
	return
}

func (qa quitAction) String() string {
	if qa.force {
		return fmt.Sprintf("force %v", qa.content)
	}
	return fmt.Sprintf("%v", qa.content)
}
