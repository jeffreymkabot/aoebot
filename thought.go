package main

import (
	"io"
	"os"
	"encoding/binary"
	"github.com/bwmarrin/discordgo"
)

// associate a response to trigger
type condition struct {
	// TODO isTriggeredBy better name?
	trigger  func(ctx context) bool
	response action
}

// perform an action given the context (environment) of its trigger
type action interface {
	perform(ctx context) error
}

// type context interface{}
type context struct {
	session *discordgo.Session
	guild   *discordgo.Guild
	channel *discordgo.Channel
	author  *discordgo.User
	message string
}

type textAction struct {
	content string
	tts     bool
}

type voiceAction struct {
	file   string
	buffer [][]byte
}

// type something to the text channel of the original context
func (ta *textAction) perform(ctx context) error {
	var err error
	if ta.tts {
		_, err = ctx.session.ChannelMessageSendTTS(ctx.channel.ID, ta.content)
	} else {
		_, err = ctx.session.ChannelMessageSend(ctx.channel.ID, ta.content)
	}
	return err
}

// say something to the voice channel of the user in the original context
func (va *voiceAction) perform(ctx context) error{
	vcId := getVoiceChannelIdByContext(ctx)
	if vcId == "" {
		ctx.session.ChannelMessageSend(ctx.channel.ID, "You should be in a voice channel!")
		return nil
	}
	voiceQueue <- voicePayload{
		buffer: va.buffer,
		channelId: vcId,
		guildId: ctx.guild.ID,
	}
	return nil
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