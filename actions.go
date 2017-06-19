package main

import (
	"encoding/binary"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"io"
	// "log"
	"os"
	// "strings"
	// "time"
)

// associate a response to trigger
type condition struct {
	// TODO isTriggeredBy better name?
	trigger  func(ctx *Context) bool
	response action
	name     string
}

// perform an action given the context (environment) of its trigger
type action interface {
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
	guild        *discordgo.Guild
	textChannel  *discordgo.Channel
	textMessage  *discordgo.Message
	voiceChannel *discordgo.Channel
	author       *discordgo.User
	Type         int
}

// NewContext creates a new environment based on a seed event/trigger
func NewContext(seed interface{}) (ctx *Context, err error) {
	ctx = &Context{}
	switch s := seed.(type) {
	case *discordgo.Message:
		ctx.Type = MessageContext
		ctx.textMessage = s
		ctx.author = s.Author
		ctx.textChannel, err = me.session.State.Channel(s.ChannelID)
		if err != nil {
			return
		}
		ctx.guild, err = me.session.State.Guild(ctx.textChannel.GuildID)
		if err != nil {
			return
		}
	case *discordgo.VoiceState:
		ctx.Type = VoiceStateContext
		ctx.author, err = me.session.User(s.UserID)
		if err != nil {
			return
		}
		ctx.voiceChannel, err = me.session.State.Channel(s.ChannelID)
		if err != nil {
			return
		}
		ctx.guild, err = me.session.State.Guild(ctx.voiceChannel.GuildID)
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
	return ctx.author != nil && ctx.author.ID == me.self.ID
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
	content string
}

// type something to the text channel of the original context
func (ta textAction) perform(ctx *Context) (err error) {
	err = me.Write(ctx.textChannel.ID, ta.content, ta.tts)
	return
}

func (ta textAction) String() string {
	if ta.tts {
		return fmt.Sprintf("/tts %v", ta.content)
	}
	return fmt.Sprintf("%v", ta.content)
}

func (era emojiReactionAction) perform(ctx *Context) (err error) {
	err = me.React(ctx.textChannel.ID, ctx.textMessage.ID, era.emoji)
	return
}

func (era emojiReactionAction) String() string {
	return fmt.Sprintf("%x", era.emoji)
}

// say something to the voice channel of the user in the original context
func (va *voiceAction) perform(ctx *Context) (err error) {
	vcID := ""
	if ctx.voiceChannel != nil {
		vcID = ctx.voiceChannel.ID
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
	err = me.Say(vp, ctx.guild.ID)
	return
}

func getVoiceChannelIDByContext(ctx *Context) string {
	return getVoiceChannelIDByUser(ctx.guild, ctx.author)
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
		_ = me.Write(ctx.textChannel.ID, rva.content, false)
	}
	me.reconnectVoicebox(ctx.guild)
	return
}

func (rva reconnectVoiceAction) String() string {
	return fmt.Sprintf("%v", rva.content)
}

func (ra restartAction) perform(ctx *Context) (err error) {
	if ra.content != "" {
		_ = me.Write(ctx.textChannel.ID, ra.content, false)
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
		_ = me.Write(ctx.textChannel.ID, qa.content, false)
	}
	me.Die()
	return
}

func (qa quitAction) String() string {
	return fmt.Sprintf("%v", qa.content)
}
