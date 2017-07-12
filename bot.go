/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package aoebot

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	// "gopkg.in/mgo.v2"
	"log"
	"math/rand"
	"sync"
	"time"
)

// Bot represents a discord bot
type Bot struct {
	mu sync.Mutex // TODO synchronize state and use of maps
	// kill       chan struct{}
	// err        error
	token string
	owner string
	// dbURL      string
	// mongo      *mgo.Session
	driver     *aoebotDriver
	session    *discordgo.Session
	self       *discordgo.User
	routines   map[*botroutine]struct{}
	unhandlers map[*func()]struct{}
	voiceboxes map[string]*voicebox // TODO voiceboxes is vulnerable to concurrent read/write
	occupancy  map[string]string    // TODO occupancy is vulnerable to concurrent read/write
	aesthetic  bool
}

// New initializes a bot
func New(token string, owner string, dbURL string) (b *Bot, err error) {
	b = &Bot{
		// kill:       make(chan struct{}),
		token: token,
		owner: owner,
		// dbURL:      dbURL,
		routines:   make(map[*botroutine]struct{}),
		unhandlers: make(map[*func()]struct{}),
		voiceboxes: make(map[string]*voicebox),
		occupancy:  make(map[string]string),
	}
	b.driver = newAoebotDriver(dbURL)
	b.session, err = discordgo.New("Bot " + b.token)
	if err != nil {
		return
	}
	b.session.LogLevel = discordgo.LogDebug
	return
}

// func (b *Bot) Kill() <-chan struct{} {
// 	return b.kill
// }

// Wakeup initiates a new database session and discord session
func (b *Bot) Wakeup() (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	err = b.driver.wakeup()
	if err != nil {
		return
	}

	b.self, err = b.session.User("@me")
	if err != nil {
		return
	}

	b.addHandlerOnce(b.onReady())
	// begin listen to discord websocket for events
	// invoking session.Open() triggers the discord ready event
	err = b.session.Open()
	if err != nil {
		return
	}
	return
}

// Sleep removes event handlers, stops all workers, closes the discord session, and closes the db session.
func (b *Bot) Sleep() {
	b.mu.Lock()
	defer b.mu.Unlock()
	log.Printf("Closing session...")

	for f := range b.unhandlers {
		if f != nil {
			(*f)()
		}
		delete(b.unhandlers, f)
	}

	for r := range b.routines {
		if r.quit != nil {
			close(r.quit)
			r.quit = nil
		}
		delete(b.routines, r)
	}

	for k, vb := range b.voiceboxes {
		if vb.quit != nil {
			close(vb.quit)
			vb.quit = nil
		}
		delete(b.voiceboxes, k)
	}

	// close the session after closing voice boxes since closing voiceboxes attempts graceful disconnect using discord session
	b.session.Close()

	b.driver.sleep()

	log.Printf("...closed session.")
}

// Add an event handler to the discord session and retain a reference to the handler remover
func (b *Bot) addHandler(handler interface{}) {
	unhandler := b.session.AddHandler(handler)
	b.unhandlers[&unhandler] = struct{}{}
}

// Add a one-time event handler to the discord session and retain a reference to the handler remover
func (b *Bot) addHandlerOnce(handler interface{}) {
	unhandler := b.session.AddHandlerOnce(handler)
	b.unhandlers[&unhandler] = struct{}{}
}

func (b *Bot) addRoutine(f func(<-chan struct{})) {
	quit := make(chan struct{})
	go f(quit)
	r := &botroutine{
		quit: quit,
	}
	b.routines[r] = struct{}{}
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

// helper func
func (b *Bot) sayToUserInGuild(guild *discordgo.Guild, userID string, audio [][]byte) (err error) {
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return b.Say(guild.ID, vs.ChannelID, audio)
		}
	}
	return fmt.Errorf("Couldn't find user %v in a voice channel in guild %v", userID, guild.ID)
}

// Listen to some audio frames in a guild
// TODO
func (b *Bot) Listen(guildID string, channelID string, duration time.Duration) (err error) {
	return
}

// IsOwnEnvironment is true when an environment references the bot's own actions/behavior
// This is useful to prevent the bot from reacting to itself
func (b *Bot) IsOwnEnvironment(env *Environment) bool {
	return env.Author != nil && env.Author.ID == b.self.ID
}

func (b *Bot) onReady() func(s *discordgo.Session, r *discordgo.Ready) {
	// Access b Bot through a closure
	return func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Ready: %#v\n", r)
		for _, g := range r.Guilds {
			// exec independently per each guild g
			b.SpeakTo(g)
			for _, vs := range g.VoiceStates {
				b.occupancy[vs.UserID] = vs.ChannelID
			}
		}
		b.addHandler(b.onMessageCreate())
		b.addHandler(b.onVoiceStateUpdate())
		b.addRoutine(b.randomVoiceInOpenMic())
	}
}

func (b *Bot) onMessageCreate() func(*discordgo.Session, *discordgo.MessageCreate) {
	// Create a context around a voice state when the bot sees a new text message
	// Perform any actions that match that contex
	// Access b Bot through a closure
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Message == nil {
			return
		}
		log.Printf("Saw a new message (%v) by %s in channel %v", m.Message.Content, m.Message.Author, m.Message.ChannelID)

		// %

		env, err := NewEnvironment(s, m.Message)
		if err != nil {
			log.Printf("Error resolving message context: %v", err)
			return
		}
		if b.IsOwnEnvironment(env) {
			return
		}

		// %

		actions := b.driver.Actions(env)
		b.dispatch(env, actions...)
	}
}

func (b *Bot) onVoiceStateUpdate() func(*discordgo.Session, *discordgo.VoiceStateUpdate) {
	// Create a context around a voice state when the bot sees someone's voice channel change
	// Perform any actions that match that contex
	// Access b Bot through a closure
	return func(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
		userID := v.VoiceState.UserID
		channelID := v.VoiceState.ChannelID

		occupancy := b.occupancy[userID]
		if occupancy != channelID {
			log.Printf("Saw user %v join the voice channel %v", userID, channelID)
			b.occupancy[userID] = channelID
			if channelID == "" {
				return
			}

			// %

			env, err := NewEnvironment(s, v.VoiceState)
			if err != nil {
				log.Printf("Error resolving voice state context: %v", err)
				return
			}
			if b.IsOwnEnvironment(env) {
				return
			}

			// %

			actions := b.driver.Actions(env)
			b.dispatch(env, actions...)
		}
	}
}

type botroutine struct {
	quit chan<- struct{}
}

// hardcoded experiment for now
func (b *Bot) randomVoiceInOpenMic() func(<-chan struct{}) {
	// Access b Bot through a closure
	return func(quit <-chan struct{}) {
		var err error
		var wait time.Duration

		openmicChannelID := "322881248366428161"

		log.Printf("Begin random voice routine.")
		defer log.Printf("End random voice routine.")
		for {
			select {
			case <-quit:
				return
			default:
			}
			// TODO this doesn't work on 32-bit OS
			wait = randomNormalWait(420, 90)
			log.Printf("Next random voice in %f seconds", wait.Seconds())
			select {
			case <-quit:
				return
			case <-time.After(wait):

				// %

				env := &Environment{}
				env.Type = adhoc
				env.VoiceChannel, err = b.session.State.Channel(openmicChannelID)
				if err != nil {
					log.Printf("Error resolve open mic channel %v", err)
					continue
				}
				env.Guild, err = b.session.State.Guild(env.VoiceChannel.GuildID)
				if err != nil {
					log.Printf("Error resolve open mic guild %v", err)
					continue
				}
				if b.IsOwnEnvironment(env) {
					continue
				}

				// %

				actions := b.driver.Actions(env)
				if len(actions) < 1 {
					continue
				}
				// randomly perform just one of the actions
				a := actions[rand.Intn(len(actions))]
				b.dispatch(env, a)
			}
		}
	}
}

func (b *Bot) dispatch(env *Environment, actions ...Action) {
	for _, a := range actions {
		// shadow a in the goroutine
		// a iterates through for loop goroutine would otherwise try to use it in closure asynchronously
		go func(a Action) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("Recovered from panic in perform %T on %v: %v", a, env.Type, err)
				}
			}()
			log.Printf("Perform %T on %v: %v", a, env.Type, a)
			err := a.performFunc(env)(b)
			if err != nil {
				log.Printf("Error in perform %T on %v: %v", a, env.Type, err)
			}
		}(a)
	}
}
