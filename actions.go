package aoebot

import (
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
	voice ActionType = "voice"
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
		return "/tts" + wa.Content
	}
	return wa.Content
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
	return ra.Emoji
}

// VoiceAction specifies audio that can be said to a voice channel
type VoiceAction struct {
	File  string `bson:"file,omitempty"`
	Alias string `bson:"alias,omitempty"`
}

func (va VoiceAction) Perform(env *Environment) error {
	open := func() (io.ReadCloser, error) {
		return os.Open(va.File)
	}
	if env.VoiceChannel != nil {
		return env.Bot.Voice(env.Guild.ID, env.VoiceChannel.ID, open)
	}
	return env.Bot.voiceToUserInGuild(env.Guild, env.Author.ID, open)
}

func (va VoiceAction) kind() ActionType {
	return voice
}

func (va VoiceAction) String() string {
	if va.Alias != "" {
		return va.Alias
	}
	return va.File
}
