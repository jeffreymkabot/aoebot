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
	// Maximum number of voice payloads that can wait to be processed for a particular guild
	MaxVoiceQueue = 100
	// Initial size of the list of event handler removers for a bot session
	InitUnhandlers = 10
	mainChannelID   = "140142172979724288"
	memesChannelID  = "305119943995686913"
	willowID        = "140136792849514496"
	shyronnieID     = "140898747264663552"
)

type bot struct {
	token      string
	session    *discordgo.Session
	self       *discordgo.User
	unhandlers []func()
	voiceboxes map[string]*voicebox // TODO voiceboxes is vulnerable to concurrent read/write
	occupancy  map[string]string    // TODO occupancy is vulnerable to concurrent read/write
	aesthetic  bool
}

type voicebox struct {
	guild *discordgo.Guild
	queue chan<- *voicePayload
	quit  chan<- struct{}
}

func NewBot(token string) *bot {
	b := &bot{
		token:      token,
		unhandlers: make([]func(), InitUnhandlers),
		voiceboxes: make(map[string]*voicebox),
		occupancy:  make(map[string]string),
	}
	return b
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

	b.addHandlerOnce(onReady)
	// listen to discord websocket for events
	// invoking this function triggers the discord ready event
	err = b.session.Open()
	if err != nil {
		return
	}
	return
}

func (b *bot) sleep() {
	log.Printf("Closing session...")
	for _, f := range b.unhandlers {
		if f != nil {
			f()
		}
	}
	b.unhandlers = b.unhandlers[len(b.unhandlers):]
	for k, vb := range b.voiceboxes {
		vb.quit <- struct{}{}
		delete(b.voiceboxes, k)
	}
	b.session.Close()
	b.session = nil
	log.Printf("...Closed session")
}

func (b *bot) die() {
	b.sleep()
	log.Printf("Quit.")
	os.Exit(0)
}

func (b *bot) addHandler(handler interface{}) {
	unhandler := b.session.AddHandler(handler)
	b.unhandlers = append(b.unhandlers, unhandler)
}

func (b *bot) addHandlerOnce(handler interface{}) {
	unhandler := b.session.AddHandlerOnce(handler)
	b.unhandlers = append(b.unhandlers, unhandler)
}

func onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Ready: %#v\n", r)
	time.Sleep(100 * time.Millisecond)
	for _, g := range r.Guilds {
		// exec independent per each guild g
		me.voiceboxes[g.ID] = me.connectVoicebox(g)
		for _, vs := range g.VoiceStates {
			me.occupancy[vs.UserID] = vs.ChannelID
		}
	}
	me.addHandler(onMessageCreate)
	me.addHandler(onVoiceStateUpdate)
}

// dispatch voice data to a particular discord guild
// listen to a queue of voicePayloads for that guild
// voicePayloads provide data meant for a voice channel in a discord guild
// we can remain connected to the same channel while we process a relatively contiguous stream of voicePayloads
// for that channel
func (b *bot) connectVoicebox(g *discordgo.Guild) *voicebox {
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
			log.Printf("Disconnect voice in guild %v", g)
			_ = vc.Speaking(false)
			_ = vc.Disconnect()
			vc = nil
		}
	}
	goAfk := func() {
		log.Printf("Join afk channel %v in guild %v", g.AfkChannelID, g)
		vc, err = b.session.ChannelVoiceJoin(g.ID, g.AfkChannelID, true, true)
		if err != nil {
			log.Printf("Error join afk: %#v", err)
			disconnect()
		} else {
			dcTimer = time.AfterFunc(1*time.Minute, disconnect)
		}
	}
	// defer goAfk()
	queue := make(chan *voicePayload, MaxVoiceQueue)
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
				log.Printf("Speak to channel %v in guild %v", vp.channelID, g)
				vc, err = b.session.ChannelVoiceJoin(g.ID, vp.channelID, false, true)
				if err != nil {
					log.Printf("Error join channel: %#v\n", err)
					afkTimer = time.AfterFunc(300*time.Millisecond, goAfk)
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
				log.Printf("Quit voice in guild %v", g)
				disconnect()
				return
			}
		}
	}()

	return &voicebox{
		guild: g,
		queue: queue,
		quit:  quit,
	}
}

func (b *bot) reconnectVoicebox(g *discordgo.Guild) (err error) {
	// TODO synchronize connect with end of quit
	vb, ok := b.voiceboxes[g.ID]
	if ok {
		vb.quit <- struct{}{}
	}
	b.voiceboxes[g.ID] = b.connectVoicebox(g)
	return
}

// Say some audio frames to a guild
// Say drops the payload if the voicebox queue is full
func (b *bot) say(vp *voicePayload, guildID string) (err error) {
	vb, ok := b.voiceboxes[guildID]
	if ok {
		select {
		case vb.queue <- vp:
		default:
			err = fmt.Errorf("Full voicebox in guild %v", vb.guild)
		}
	} else {
		err = fmt.Errorf("No voicebox registered for guild id %v", guildID)
	}
	return
}

// Write a message to a channel in a guild
func (b *bot) write(channelID string, message string, tts bool) (err error) {
	if tts {
		_, err = b.session.ChannelMessageSendTTS(channelID, message)
	} else {
		_, err = b.session.ChannelMessageSend(channelID, message)
	}
	return
}

// React with an emoji to a message in a channel in a guild
func (b *bot) react(channelID string, messageID string, emoji string) (err error) {
	err = b.session.MessageReactionAdd(channelID, messageID, emoji)
	return
}

// TODO Listen to some audio frames in a guild
func (b *bot) listen() (err error) {
	return nil
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	log.Printf("Saw a message: %#v\n", m.Message)
	// ctx, err := getMessageContext(s, m.Message)
	ctx, err := NewContext(m.Message)
	if err != nil {
		log.Printf("Error resolving message context: %v", err)
		return
	}
	if ctx.isOwnContext() {
		return
	}

	for _, c := range conditions {
		if c.trigger(ctx) {
			// shadow c in the closure of the goroutine
			go func(c condition) {
				defer func() {
					if err := recover(); err != nil {
						log.Printf("Recovered from panic in perform %T on message create: %v", c.response, err)
					}
				}()
				log.Printf("Perform %T on message create: %v", c.response, c.response)
				err := c.response.perform(ctx)
				if err != nil {
					log.Printf("Error in perform %T on message create: %v", c.response, err)
				}
			}(c)
		}
	}
}

// TODO some quick and dirty hard coded experiments for now
func onVoiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	log.Printf("Saw a voice state update: %#v\n", v.VoiceState)
	userID := v.VoiceState.UserID
	// guildID := v.VoiceState.GuildID
	channelID := v.VoiceState.ChannelID

	if me.occupancy[userID] != channelID {
		me.occupancy[userID] = channelID
		if channelID == "" {
			return
		}

		ctx, err := NewContext(v.VoiceState)
		if err != nil {
			log.Printf("Error resolving message context: %v", err)
			return
		}
		if ctx.isOwnContext() {
			return
		}

		for _, c := range conditions {
			if c.trigger(ctx) {
				// shadow c because it changes in the closure via for loop
				go func(c condition) {
					defer func() {
						if err := recover(); err != nil {
							log.Printf("Recovered from panic in perform %T on voice state update: %v", c.response, err)
						}
					}()
					log.Printf("Perform %T on voice state update: %v", c.response, c.response)
					err := c.response.perform(ctx)
					if err != nil {
						log.Printf("Error in perform %T on voice state update: %v", c.response, err)
					}
				}(c)
			}
			// if userID == shyronnieID && c.name == "shyronnie" {
			// 	go func() {
			// 		// shadow c because it changes in the closure via for loop
			// 		defer func(c condition) {
			// 			if err := recover(); err != nil {
			// 				log.Printf("Recovered from panic in action on voice state update: %v", err)
			// 			}
			// 			vp := &voicePayload{
			// 				buffer:    c.response.(*voiceAction).buffer,
			// 				channelID: channelID,
			// 			}
			// 			time.AfterFunc(100*time.Millisecond, func() {
			// 				err := me.say(vp, guildID)
			// 				if err != nil {
			// 					log.Printf("Error in speak on voice state update: %v", err)
			// 				}
			// 			})
			// 		}(c)
			// 	}()
			// 	return
			// }
		}
	}
	return
}

// func getVoiceStateContext(s *discordgo.Session, v *discordgo.VoiceState) (ctx context, err error) {
// 	ctx.author, err = s.User(v.UserID)
// 	if err != nil {
// 		return
// 	}
// 	ctx.channel, err = s.Channel(v.ChannelID)
// 	if err != nil {
// 		return
// 	}
// 	ctx.guild, err = s.Guild(ctx.channel.GuildID)
// 	if err != nil {
// 		return
// 	}
// 	return
// }

// func getMessageContext(s *discordgo.Session, m *discordgo.Message) (ctx context, err error) {
// 	ctx.author = m.Author
// 	ctx.message = m.Content
// 	ctx.messageID = m.ID
// 	ctx.channel, err = s.Channel(m.ChannelID)
// 	if err != nil {
// 		return
// 	}
// 	ctx.guild, err = s.Guild(ctx.channel.GuildID)
// 	if err != nil {
// 		return
// 	}
// 	return
// }

// func isAuthorAllowed(author *discordgo.User) bool {
// 	return author.ID != me.self.ID
// }

// func (g discordgo.Guild) String() string {
// 	return fmt.Sprintf("%v %v", g.Name, g.ID)
// }

// func (c discordgo.Channel) String() string {
// 	return fmt.Sprintf("%v %v", c.Name, c.ID)
// }
