/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gopkg.in/mgo.v2"
	"log"
	"math/rand"
	"os"
	"time"
)

const (
	// globals are only for convienience while experiment
	// values will eventually be encoded with action conditions in a database
	openmicChannelID = "322881248366428161"
)

// Bot represents a discord bot
type Bot struct {
	token      string
	owner      string
	dbURL      string
	mongo      *mgo.Session
	session    *discordgo.Session
	self       *discordgo.User
	routines   []*botroutine
	unhandlers []func()
	voiceboxes map[string]*voicebox // TODO voiceboxes is vulnerable to concurrent read/write
	occupancy  map[string]string    // TODO occupancy is vulnerable to concurrent read/write
	aesthetic  bool
}

// NewBot initializes a bot
func NewBot(token string, owner string, dbURL string) *Bot {
	b := &Bot{
		token:      token,
		owner:      owner,
		dbURL:      dbURL,
		routines:   []*botroutine{},
		unhandlers: []func(){},
		voiceboxes: make(map[string]*voicebox),
		occupancy:  make(map[string]string),
	}
	return b
}

// Wakeup initiates a new db session and discord session
func (b *Bot) Wakeup() (err error) {
	b.mongo, err = mgo.Dial(b.dbURL)
	if err != nil {
		return
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

// Sleep removes event handlers, stops all workers, closes the discord session, and closes the db session.
func (b *Bot) Sleep() {
	log.Printf("Closing session...")

	for _, f := range b.unhandlers {
		if f != nil {
			f()
		}
	}
	b.unhandlers = b.unhandlers[len(b.unhandlers):]

	for _, r := range b.routines {
		if r.quit != nil {
			close(r.quit)
			r.quit = nil
		}
	}
	b.routines = b.routines[len(b.routines):]

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

	b.mongo.Close()
	b.mongo = nil

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

func (b *Bot) addRoutine(f func(<-chan struct{})) {
	quit := make(chan struct{})
	go f(quit)
	r := &botroutine{
		quit: quit,
	}
	b.routines = append(b.routines, r)
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
	me.addRoutine(randomVoiceInOpenMic)
}

// Create a context around a voice state when the bot sees a new text message
// Perform any actions that match that context
func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Message == nil {
		return
	}
	log.Printf("Saw a new message (%v) by user %v in channel %v", m.Message.Content, m.Message.Author, m.Message.ChannelID)
	env, err := NewEnvironment(m.Message)
	if err != nil {
		log.Printf("Error resolving message context: %v", err)
		return
	}
	if env.IsOwnEnvironment() {
		return
	}

	actions := env.Actions()
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
			err := a.perform(env)
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

		env, err := NewEnvironment(v.VoiceState)
		if err != nil {
			log.Printf("Error resolving voice state context: %v", err)
			return
		}
		if env.IsOwnEnvironment() {
			return
		}

		actions := env.Actions()
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
				err := a.perform(env)
				if err != nil {
					log.Printf("Error in perform %T on voice state update: %v", a, err)
				}
			}(a)
		}
	}
	return
}

type botroutine struct {
	quit chan<- struct{}
}

// hardcoded experiment for now
func randomVoiceInOpenMic(quit <-chan struct{}) {
	var err error
	var wait time.Duration
	log.Printf("Begin random voice routine")
	defer log.Printf("End random voice routine")
	for {
		select {
		case <-quit:
			return
		default:
		}
		wait = randomNormalWait(420, 90)
		log.Printf("Next random voice in %f seconds", wait.Seconds())
		select {
		case <-quit:
			return
		case <-time.After(wait):
			env := &Environment{}
			env.Type = adhoc
			env.VoiceChannel, err = me.session.State.Channel(openmicChannelID)
			if err != nil {
				log.Printf("Error resolve open mic channel %v", err)
				continue
			}
			env.Guild, err = me.session.State.Guild(env.VoiceChannel.GuildID)
			if err != nil {
				log.Printf("Error resolve open mic guild %v", err)
				continue
			}
			if env.IsOwnEnvironment() {
				continue
			}
			actions := env.Actions()
			// randomly perform just one of the actions
			a := actions[rand.Intn(len(actions))]
			go func(a Action) {
				defer func() {
					if err := recover(); err != nil {
						log.Printf("Recovered from panic in perform %T on random voice: %v", a, err)
					}
				}()
				log.Printf("Perform %T on random voice: %v", a, a)
				err := a.perform(env)
				if err != nil {
					log.Printf("Error in perform %T on random voice: %v", a, err)
				}
			}(a)
		}
	}
}

// func (g discordgo.Guild) String() string {
// 	return fmt.Sprintf("%v %v", g.Name, g.ID)
// }

// func (c discordgo.Channel) String() string {
// 	return fmt.Sprintf("%v %v", c.Name, c.ID)
// }
