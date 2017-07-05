package main

import (
	"encoding/binary"
	_ "encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/fatih/structs"
	// "gopkg.in/mgo.v2"
	"io"
	"os"
)

// Action can be performed given the context (environment) of its trigger
type Action interface {
	perform(ctx *Context) error
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

	// TODO cache file contents from load
	err = sa.load()
	if err != nil {
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
