package main

import (
	"encoding/binary"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/fatih/structs"
	"io"
	"os"
)

// ActionType is used as a hint for unmarshalling actions from untyped languages e.g. JSON, BSON
type ActionType string

const (
	write     ActionType = "write"
	say       ActionType = "say"
	react     ActionType = "react"
	stats     ActionType = "stats"
	reconnect ActionType = "reconnect"
	restart   ActionType = "restart"
	quit      ActionType = "quit"
)

// Action can be performed given the environment of its trigger
type Action interface {
	perform(env *Environment) error
	kind() ActionType
}

// WriteAction specifies content that can be written to a text channel
type WriteAction struct {
	Content string
	TTS     bool
}

// type something to the text channel of the original environment
func (wa WriteAction) perform(env *Environment) (err error) {
	err = me.Write(env.TextChannel.ID, wa.Content, wa.TTS)
	return
}

func (wa WriteAction) kind() ActionType {
	return write
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

func (ra ReactAction) perform(env *Environment) (err error) {
	err = me.React(env.TextChannel.ID, env.TextMessage.ID, ra.Emoji)
	return
}

func (ra ReactAction) kind() ActionType {
	return react
}

func (ra ReactAction) String() string {
	return fmt.Sprintf("%x", ra.Emoji)
}

// SayAction specifies content that can be said to a voice channel
type SayAction struct {
	File   string
	buffer [][]byte
}

// say something to the voice channel of the user in the original environment
func (sa SayAction) perform(env *Environment) (err error) {
	vcID := ""
	if env.VoiceChannel != nil {
		vcID = env.VoiceChannel.ID
	} else {
		vcID = getVoiceChannelIDByEnvironment(env)
	}
	if vcID == "" {
		return
	}

	// TODO cache file contents from load
	err = sa.load()
	if err != nil {
		return
	}
	err = me.Say(env.Guild.ID, vcID, sa.buffer)
	return
}

func getVoiceChannelIDByEnvironment(env *Environment) string {
	return getVoiceChannelIDByUser(env.Guild, env.Author)
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

func (sa SayAction) kind() ActionType {
	return say
}

func (sa SayAction) String() string {
	return fmt.Sprintf("%v", sa.File)
}

// StatsAction indicates that runtime information should be written to a text channel
type StatsAction struct {
}

func (sa StatsAction) perform(env *Environment) (err error) {
	me.Write(env.TextChannel.ID, me.Stats().String(), false)
	return
}

func (sa StatsAction) kind() ActionType {
	return stats
}

// ReconnectVoiceAction indicates that the bot should refresh its voice worker for a guild
type ReconnectVoiceAction struct {
	Content string
}

func (rva ReconnectVoiceAction) perform(env *Environment) (err error) {
	if rva.Content != "" {
		_ = me.Write(env.TextChannel.ID, rva.Content, false)
	}
	me.SpeakTo(env.Guild)
	return
}

func (rva ReconnectVoiceAction) kind() ActionType {
	return reconnect
}

func (rva ReconnectVoiceAction) String() string {
	return fmt.Sprintf("%v", rva.Content)
}

// RestartAction indicates that the bot should restart its discord session
type RestartAction struct {
	Content string
}

func (ra RestartAction) perform(env *Environment) (err error) {
	if ra.Content != "" {
		_ = me.Write(env.TextChannel.ID, ra.Content, false)
	}
	me.Sleep()
	me.Wakeup()
	return
}

func (ra RestartAction) kind() ActionType {
	return restart
}

func (ra RestartAction) String() string {
	return fmt.Sprintf("%v", ra.Content)
}

// QuitAction indicates that the bot should terminate execution
type QuitAction struct {
	Content string
	Force   bool
}

func (qa QuitAction) perform(env *Environment) (err error) {
	if qa.Content != "" {
		_ = me.Write(env.TextChannel.ID, qa.Content, false)
	}
	if qa.Force {
		me.ForceDie()
	} else {
		me.Die()
	}
	return
}

func (qa QuitAction) kind() ActionType {
	return quit
}

func (qa QuitAction) String() string {
	if qa.Force {
		return fmt.Sprintf("force %v", qa.Content)
	}
	return fmt.Sprintf("%v", qa.Content)
}

type CreateActionAction struct {
}

func (caa CreateActionAction) perform(env *Environment) error {
	return nil
}
