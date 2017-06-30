/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"time"
)

const (
	mainChannelID  = "140142172979724288"
	memesChannelID = "305119943995686913"
	willowID       = "140136792849514496"
	shyronnieID    = "140898747264663552"
)

// Bot represents a discord bot
type Bot struct {
	token      string
	owner      string
	session    *discordgo.Session
	self       *discordgo.User
	unhandlers []func()
	voiceboxes map[string]*voicebox // TODO voiceboxes is vulnerable to concurrent read/write
	occupancy  map[string]string    // TODO occupancy is vulnerable to concurrent read/write
	aesthetic  bool
}

// NewBot initializes a bot
func NewBot(token string, owner string) *Bot {
	b := &Bot{
		token:      token,
		owner:      owner,
		unhandlers: []func(){},
		voiceboxes: make(map[string]*voicebox),
		occupancy:  make(map[string]string),
	}
	return b
}

// Wakeup initiates a new discord session
// Closes any session that may already exist
func (b *Bot) Wakeup() (err error) {
	if b.session != nil {
		b.Sleep()
	}
	b.session, err = discordgo.New("Bot " + b.token)
	if err != nil {
		return
	}

	b.self, err = b.session.User("@me")
	if err != nil {
		return
	}

	b.addHandlerOnce(onReady)
	// begin listen to discord websocket for events
	// invoking session.Open() triggers the discord ready event
	err = b.session.Open()

	return
}

// Sleep closes the discord session, removes event handlers, and stops all associated workers.
func (b *Bot) Sleep() {
	log.Printf("Closing session...")

	for _, f := range b.unhandlers {
		if f != nil {
			f()
		}
	}
	b.unhandlers = b.unhandlers[len(b.unhandlers):]

	for k, vb := range b.voiceboxes {
		if vb.quit != nil {
			close(vb.quit)
			vb.quit = nil
		}
		delete(b.voiceboxes, k)
	}

	// close the session after closing voice boxes since closing voiceboxes attempts disconnect
	b.session.Close()
	b.session = nil

	log.Printf("...closed session.")
}

// Die kills the bot gracefully
func (b *Bot) Die() {
	b.Sleep()
	log.Printf("Quit.")
	os.Exit(0)
}

// ForceDie kills the bot with a vengeance
func (b *Bot) ForceDie() {
	log.Printf("Force Quit.")
	os.Exit(1)
}

// Add an event handler to the discord session and retain a reference to the handler remover
func (b *Bot) addHandler(handler interface{}) {
	unhandler := b.session.AddHandler(handler)
	b.unhandlers = append(b.unhandlers, unhandler)
}

// Add a one-time event handler to the discord session and retain a reference to the handler remover
func (b *Bot) addHandlerOnce(handler interface{}) {
	unhandler := b.session.AddHandlerOnce(handler)
	b.unhandlers = append(b.unhandlers, unhandler)
}

// Write a message to a channel in a guild
func (b *Bot) Write(channelID string, message string, tts bool) (err error) {
	if tts {
		_, err = b.session.ChannelMessageSendTTS(channelID, message)
	} else {
		_, err = b.session.ChannelMessageSend(channelID, message)
	}
	return
}

// React with an emoji to a message in a channel in a guild
func (b *Bot) React(channelID string, messageID string, emoji string) (err error) {
	err = b.session.MessageReactionAdd(channelID, messageID, emoji)
	return
}

// Say some audio frames to a channel in a guild
// Say drops the payload when the voicebox for that guild queue is full
func (b *Bot) Say(guildID string, channelID string, audio [][]byte) (err error) {
	if vb, ok := b.voiceboxes[guildID]; ok && vb != nil && vb.queue != nil {
		vp := &voicePayload{
			buffer:    audio,
			channelID: channelID,
		}
		select {
		case vb.queue <- vp:
		default:
			err = fmt.Errorf("Full voice queue in guild %v", guildID)
		}
	} else {
		err = fmt.Errorf("No voicebox registered for guild %v", guildID)
	}
	return
}

// Listen to some audio frames in a guild
// TODO
func (b *Bot) Listen(guildID string, channelID string, duration time.Duration) (err error) {
	return
}

func onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Ready: %#v\n", r)
	for _, g := range r.Guilds {
		// exec independently per each guild g
		me.SpeakTo(g)
		for _, vs := range g.VoiceStates {
			me.occupancy[vs.UserID] = vs.ChannelID
		}
	}
	me.addHandler(onMessageCreate)
	me.addHandler(onVoiceStateUpdate)
}

// Create a context around a voice state when the bot sees a new text message
// Perform any actions that match that context
func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Message == nil {
		return
	}
	log.Printf("Saw a new message (%v) by user %v in channel %v", m.Message.Content, m.Message.Author.Username, m.Message.ChannelID)
	ctx, err := NewContext(m.Message)
	if err != nil {
		log.Printf("Error resolving message context: %v", err)
		return
	}
	if ctx.IsOwnContext() {
		return
	}

	actions := ctx.Actions()
	for _, a := range actions {
		// shadow a in the goroutine
		// as a iterates through for loop while goroutine would otherwise try to use it in closure asynchronously
		go func(a Action) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("Recovered from panic in perform %T on message create: %v", a, err)
				}
			}()
			log.Printf("Perform %T on message create: %v", a, a)
			err := a.perform(ctx)
			if err != nil {
				log.Printf("Error in perform %T on message create: %v", a, err)
			}
		}(a)
	}
}

// Create a context around a voice state when the bot sees someone's voice channel change
// Perform any actions that match that context
func onVoiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	userID := v.VoiceState.UserID
	channelID := v.VoiceState.ChannelID

	if me.occupancy[userID] != channelID {
		log.Printf("Saw user %v join the voice channel %v", userID, channelID)
		// concurrent write vulnerability here
		me.occupancy[userID] = channelID
		if channelID == "" {
			return
		}

		ctx, err := NewContext(v.VoiceState)
		if err != nil {
			log.Printf("Error resolving voice state context: %v", err)
			return
		}
		if ctx.IsOwnContext() {
			return
		}

		actions := ctx.Actions()
		for _, a := range actions {
			// shadow a in the goroutine
			// as a iterates through for loop while goroutine would otherwise try to use it in closure asynchronously
			go func(a Action) {
				defer func() {
					if err := recover(); err != nil {
						log.Printf("Recovered from panic in perform %T on voice state update: %v", a, err)
					}
				}()
				log.Printf("Perform %T on voice state update: %v", a, a)
				err := a.perform(ctx)
				if err != nil {
					log.Printf("Error in perform %T on voice state update: %v", a, err)
				}
			}(a)
		}
	}
	return
}

// func (g discordgo.Guild) String() string {
// 	return fmt.Sprintf("%v %v", g.Name, g.ID)
// }

// func (c discordgo.Channel) String() string {
// 	return fmt.Sprintf("%v %v", c.Name, c.ID)
// }
