package main

import (
	"encoding/binary"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"io"
	"log"
	"os"
	// "strings"
	"time"
)

type bot struct {
	token      string
	session    *discordgo.Session
	self       *discordgo.User
	quit       chan struct{}
	voiceboxes map[string]*voicebox // TODO voiceboxes is vulnerable to concurrent read/write
	occupancy  map[string]string    // TODO occupancy is vulnerable to concurrent read/write
	aesthetic  bool
}

type voicebox struct {
	queue chan<- *voicePayload
	quit  chan<- struct{}
}

func (b *bot) wakeup() (err error) {
	b.session, err = discordgo.New("Bot " + b.token)
	if err != nil {
		return
	}

	b.self, err = b.session.User("@me")
	if err != nil {
		return
	}

	b.session.AddHandler(onReady)
	// listen to discord websocket for events
	// this function triggers the ready event on success
	err = b.session.Open()
	if err != nil {
		return
	}
	b.quit = make(chan struct{})
	go func() {
		log.Printf("Listening for session quit...")
		<-b.quit
		log.Printf("...Got a session quit")
		b.sleep()
	}()
	return
}

func (b *bot) sleep() {
	for _, vb := range b.voiceboxes {
		vb.quit <- struct{}{}
	}
	b.session.Close()
	log.Printf("Closed session")
	os.Exit(0)
}

// dispatch voice data to a particular discord guild
// listen to a queue of voicePayloads for that guild
// voicePayloads provide data meant for a voice channel in a discord guild
// we can remain connected to the same channel while we process a relatively contiguous stream of voicePayloads
// for that channel
func (b *bot) speakTo(g *discordgo.Guild) *voicebox {
	var vc *discordgo.VoiceConnection
	var err error

	// afk after a certain amount of time not talking
	var afkTimer *time.Timer
	// disconnect voice after a certain amount of time afk
	var dcTimer *time.Timer

	// disconnect() and goAfk() get invoked as the function arg in time.AfterFunc()
	// need to use closures so they can manipulate same VoiceConnection vc used in speakTo()
	disconnect := func() {
		if vc != nil {
			log.Printf("Disconnect voice in guild %v %v", g.Name, g.ID)
			_ = vc.Speaking(false)
			_ = vc.Disconnect()
			vc = nil
		}
	}
	goAfk := func() {
		log.Printf("Join afk channel %v in guild %v %v", g.AfkChannelID, g.Name, g.ID)
		vc, err = b.session.ChannelVoiceJoin(g.ID, g.AfkChannelID, true, true)
		if err != nil {
			log.Printf("Error join afk: %#v", err)
			disconnect()
		} else {
			dcTimer = time.AfterFunc(5*time.Minute, disconnect)
		}
	}
	defer goAfk()
	queue := make(chan *voicePayload)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case vp := <-queue:
				if afkTimer != nil {
					afkTimer.Stop()
				}
				if dcTimer != nil {
					dcTimer.Stop()
				}
				log.Printf("Speak to channel %v in guild %v %v", vp.channelID, g.Name, g.ID)
				vc, err = b.session.ChannelVoiceJoin(g.ID, vp.channelID, false, true)
				if err != nil {
					log.Printf("Error join channel: %#v\n", err)
					break
				}
				_ = vc.Speaking(true)
				time.Sleep(100 * time.Millisecond)
				for _, sample := range vp.buffer {
					vc.OpusSend <- sample
				}
				time.Sleep(100 * time.Millisecond)
				_ = vc.Speaking(false)
				afkTimer = time.AfterFunc(300*time.Millisecond, goAfk)
			case <-quit:
				if afkTimer != nil {
					afkTimer.Stop()
				}
				if dcTimer != nil {
					dcTimer.Stop()
				}
				log.Printf("Quit voice in guild %v %v", g.Name, g.ID)
				disconnect()
				return
			}
		}
	}()

	return &voicebox{
		queue: queue,
		quit:  quit,
	}
}

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
	// permissions, err := ctx.session.State.UserChannelPermissions(selfUser.ID, ctx.channel.ID)
	// fmt.Printf("My channel permissions are %v\n", permissions)
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
	me.voiceboxes[ctx.guild.ID].queue <- &voicePayload{
		buffer:    va.buffer,
		channelID: vcId,
		// guild:     ctx.guild,
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

func (quit *quitAction) perform(ctx context) error {
	// _, err := ctx.session.ChannelMessageSend(ctx.channel.ID, quit.content)
	err := (&textAction{
		content: quit.content,
		tts:     false,
	}).perform(ctx)
	me.quit <- struct{}{}
	return err
}
