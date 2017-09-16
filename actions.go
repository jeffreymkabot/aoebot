package aoebot

import (
	"bytes"
	"fmt"
	"io"
	"os"
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
	_, err := env.Bot.React(env.TextChannel.ID, env.TextMessage.ID, ra.Emoji)
	return err
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
	// TODO could cache result of sa.load
	r, err := va.load()
	if err != nil {
		return err
	}
	if env.VoiceChannel != nil {
		return env.Bot.Say(env.Guild.ID, env.VoiceChannel.ID, r)
	}
	return env.Bot.sayToUserInGuild(env.Guild, env.Author.ID, r)
}

func (va VoiceAction) load() (io.Reader, error) {
	file, err := os.Open(va.File)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := bytes.NewBuffer([]byte{})
	_, err = buf.ReadFrom(file)
	return buf, err
}

func (va VoiceAction) kind() ActionType {
	return voice
}

func (va VoiceAction) String() string {
	return fmt.Sprintf("%v", va.File)
}
