package main

import (
	"encoding/binary"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"io"
	"os"
)

// associate a response to trigger
type condition struct {
	// TODO isTriggeredBy better name?
	trigger  func(ctx context) bool
	response action
	name     string
}

// perform an action given the context (environment) of its trigger
type action interface {
	perform(ctx context) error
}

// TODO more generic to support capturing the context of more events
type context struct {
	session   *discordgo.Session
	guild     *discordgo.Guild
	channel   *discordgo.Channel
	author    *discordgo.User
	message   string
	messageId string
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

type voicePayload struct {
	buffer    [][]byte
	channelID string
	guild     *discordgo.Guild
}

// type something to the text channel of the original context
func (ta *textAction) perform(ctx context) error {
	var err error
	fmt.Printf("perform text action %#v\n", ta)
	if ta.tts {
		_, err = ctx.session.ChannelMessageSendTTS(ctx.channel.ID, ta.content)
	} else {
		_, err = ctx.session.ChannelMessageSend(ctx.channel.ID, ta.content)
	}
	return err
}

func (era *emojiReactionAction) perform(ctx context) error {
	var err error
	fmt.Printf("perform emoji action %#v\n", era)
	permissions, err := ctx.session.State.UserChannelPermissions(selfId, ctx.channel.ID)
	fmt.Printf("My channel permissions are %v\n", permissions)
	err = ctx.session.MessageReactionAdd(ctx.channel.ID, ctx.messageId, era.emoji)
	return err
}

// say something to the voice channel of the user in the original context
func (va *voiceAction) perform(ctx context) error {
	vcId := getVoiceChannelIdByContext(ctx)
	if vcId == "" {
		//ctx.session.ChannelMessageSend(ctx.channel.ID, "You should be in a voice channel!")
		return nil
	}
	fmt.Printf("perform voice action %#v\n", va.file)
	voiceQueues[ctx.guild] <- &voicePayload{
		buffer:    va.buffer,
		channelID: vcId,
		guild:     ctx.guild,
	}
	return nil
}

func getVoiceChannelIdByContext(ctx context) string {
	return getVoiceChannelIdByUser(ctx.guild, ctx.author)
}

func getVoiceChannelIdByUser(g *discordgo.Guild, u *discordgo.User) string {
	for _, vs := range g.VoiceStates {
		if vs.UserID == u.ID {
			return vs.ChannelID
		}
	}
	return ""
}

func getVoiceChannelIdByUserId(g *discordgo.Guild, uId string) string {
	for _, vs := range g.VoiceStates {
		if vs.UserID == uId {
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
