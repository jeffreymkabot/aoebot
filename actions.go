package aoebot

import (
	"encoding/binary"
	"fmt"
	// "github.com/bwmarrin/discordgo"
	// "github.com/fatih/structs"
	"io"
	"os"
)

// Action can be performed given the environment of its trigger
type Action interface {
	performFunc(*Environment) func(*Bot) error
	kind() ActionType
}

// ActionType is used as a hint for unmarshalling actions from untyped languages e.g. JSON, BSON
type ActionType string

const (
	null  ActionType = "null"
	write ActionType = "write"
	say   ActionType = "say"
	react ActionType = "react"
)

// WriteAction specifies content that can be written to a text channel
type WriteAction struct {
	Content string
	TTS     bool
}

func (wa WriteAction) performFunc(env *Environment) func(*Bot) error {
	return func(b *Bot) error {
		return b.Write(env.TextChannel.ID, wa.Content, wa.TTS)
	}
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

func (ra ReactAction) performFunc(env *Environment) func(*Bot) error {
	return func(b *Bot) error {
		return b.React(env.TextChannel.ID, env.TextMessage.ID, ra.Emoji)
	}
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

func (sa SayAction) performFunc(env *Environment) func(*Bot) error {
	return func(b *Bot) error {
		// TODO cache result of sa.load
		buf, err := sa.load()
		if err != nil {
			return err
		}
		if env.VoiceChannel != nil {
			return b.Say(env.Guild.ID, env.VoiceChannel.ID, buf)
		}
		return b.sayToUserInGuild(env.Guild, env.Author.ID, buf)
	}
}

func (sa SayAction) load() (buf [][]byte, err error) {
	buf = make([][]byte, 0)
	file, err := os.Open(sa.File)
	if err != nil {
		return
	}
	defer file.Close()

	var opuslen int16

	for {
		err = binary.Read(file, binary.LittleEndian, &opuslen)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return buf, nil
		}
		if err != nil {
			return
		}

		inbuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &inbuf)

		if err != nil {
			return
		}

		buf = append(buf, inbuf)
	}
}

func (sa SayAction) kind() ActionType {
	return say
}

func (sa SayAction) String() string {
	return fmt.Sprintf("%v", sa.File)
}
