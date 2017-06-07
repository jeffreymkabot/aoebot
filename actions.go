package main

import (
	"encoding/binary"
	// "fmt"
	"github.com/bwmarrin/discordgo"
	"io"
	"log"
	"os"
	// "strings"
	// "time"
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
	// guild     *discordgo.Guild
}

type quitAction struct {
	content string
}

// type something to the text channel of the original context
func (ta *textAction) perform(ctx context) (err error) {
	log.Printf("perform text action %#v", ta)
	err = me.write(ta.content, ctx.channel.ID, ta.tts)
	return
}

func (era *emojiReactionAction) perform(ctx context) (err error) {
	log.Printf("perform emoji action %#v", era)
	// permissions, err := ctx.session.State.UserChannelPermissions(selfUser.ID, ctx.channel.ID)
	// fmt.Printf("My channel permissions are %v\n", permissions)
	err = ctx.session.MessageReactionAdd(ctx.channel.ID, ctx.messageId, era.emoji)
	return
}

// say something to the voice channel of the user in the original context
func (va *voiceAction) perform(ctx context) (err error) {
	vcId := getVoiceChannelIdByContext(ctx)
	if vcId == "" {
		//ctx.session.ChannelMessageSend(ctx.channel.ID, "You should be in a voice channel!")
		return
	}
	log.Printf("perform voice action %#v", va.file)
	vp := &voicePayload{
		buffer:    va.buffer,
		channelID: vcId,
	}
	err = me.say(vp, ctx.guild.ID)
	return
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

func (quit *quitAction) perform(ctx context) (err error) {
	// _, err := ctx.session.ChannelMessageSend(ctx.channel.ID, quit.content)
	err = (&textAction{
		content: quit.content,
		tts:     false,
	}).perform(ctx)
	me.quit <- struct{}{}
	return
}
