package aoebot

import (
	// "encoding/binary"
	"fmt"
	// "github.com/fatih/structs"
	"io"
	"os"
	"github.com/jonas747/dca"
)

// Action can be performed given the environment of its trigger
type Action interface {
	Perform(*Environment) error
	kind() ActionType
}

// ActionType is used as a hint for unmarshalling actions from untyped languages e.g. JSON, BSON
type ActionType string

const (
	write ActionType = "write"
	voice ActionType = "say"
	react ActionType = "react"
)

// WriteAction specifies content that can be written to a text channel
type WriteAction struct {
	Content string
	TTS     bool
}

func (wa WriteAction) Perform(env *Environment) error {
	return env.Bot.Write(env.TextChannel.ID, wa.Content, wa.TTS)
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

func (ra ReactAction) Perform(env *Environment) error {
	return env.Bot.React(env.TextChannel.ID, env.TextMessage.ID, ra.Emoji)
}

func (ra ReactAction) kind() ActionType {
	return react
}

func (ra ReactAction) String() string {
	return fmt.Sprintf("%s", ra.Emoji)
}

// VoiceAction specifies audio that can be said to a voice channel
type VoiceAction struct {
	File   string
	buffer [][]byte
}

func (va VoiceAction) Perform(env *Environment) error {
	// TODO cache result of sa.load
	buf, err := va.load()
	if err != nil {
		return err
	}
	if env.VoiceChannel != nil {
		return env.Bot.Say(env.Guild.ID, env.VoiceChannel.ID, buf)
	}
	return env.Bot.sayToUserInGuild(env.Guild, env.Author.ID, buf)
}

func (va VoiceAction) load() (buf [][]byte, err error) {
	buf = make([][]byte, 0)
	file, err := os.Open(va.File)
	if err != nil {
		return
	}
	defer file.Close()

	decoder := dca.NewDecoder(file)

	var frame []byte
	for {
		frame, err = decoder.OpusFrame()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		buf = append(buf, frame)
	}
}

func (va VoiceAction) kind() ActionType {
	return voice
}

func (va VoiceAction) String() string {
	return fmt.Sprintf("%v", va.File)
}
