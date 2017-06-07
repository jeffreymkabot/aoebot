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
	MAX_VOICE_QUEUE = 100
	mainChannelID   = "140142172979724288"
	memesChannelID  = "305119943995686913"
	willowID        = "140136792849514496"
	shyronnieID     = "140898747264663552"
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
	guild *discordgo.Guild
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
	queue := make(chan *voicePayload, MAX_VOICE_QUEUE)
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
		guild: g,
		queue: queue,
		quit:  quit,
	}
}

// Say leaks if the voice box is full
func (b *bot) say(vp *voicePayload, guildID string) (err error) {
	vb, ok := b.voiceboxes[guildID]
	if ok {
		select {
		case vb.queue <- vp:
		default:
			err = fmt.Errorf("Full voicebox in guild %v %v", vb.guild.Name, vb.guild.ID)
		}
	} else {
		err = fmt.Errorf("No voicebox for guild id %v", guildID)
	}
	return
}

func (b *bot) write(message string, channelID string, tts bool) (err error) {
	if tts {
		_, err = b.session.ChannelMessageSendTTS(channelID, message)
	} else {
		_, err = b.session.ChannelMessageSend(channelID, message)
	}
	return
}

func onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Ready: %#v\n", r)
	me.voiceboxes = make(map[string]*voicebox)
	me.occupancy = make(map[string]string)
	time.Sleep(100 * time.Millisecond)
	for _, g := range r.Guilds {
		// exec independent per each guild g
		me.voiceboxes[g.ID] = me.connectVoicebox(g)
		for _, vs := range g.VoiceStates {
			me.occupancy[vs.UserID] = vs.ChannelID
		}
	}
	s.AddHandler(onMessageCreate)
	s.AddHandler(onVoiceStateUpdate)
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	log.Printf("Saw someone's message: %#v\n", m.Message)
	ctx, err := getMessageContext(s, m.Message)
	if err != nil {
		log.Printf("Error resolving message context: %v", err)
		return
	}
	if !isAuthorAllowed(ctx.author) || !isChannelAllowed(ctx.channel) {
		return
	}

	for _, c := range conditions {
		if c.trigger(ctx) {
			go func(ctx context, c condition) {
				defer func() {
					if err := recover(); err != nil {
						log.Printf("Recovered from panic in action perform: %v", err)
					}
				}()
				err := c.response.perform(ctx)
				if err != nil {
					log.Printf("Error in action perform: %v", err)
				}
			}(ctx, c)
		}
	}
}

// TODO some quick and dirty hard coded experiments for now
func onVoiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {

	log.Printf("Saw a voice state update: %#v\n", v.VoiceState)
	userID := v.VoiceState.UserID
	guildID := v.VoiceState.GuildID
	channelID := v.VoiceState.ChannelID

	if me.occupancy[userID] != channelID {
		me.occupancy[userID] = channelID
		if channelID == "" {
			return
		}
		// TODO getVoiceStateContext causes pretty bad slowdowns
		// ctx, err := getVoiceStateContext(s, v.VoiceState)
		// _ = ctx
		// if err != nil {
		// 	log.Printf("Error resolving voice state context: %v", err)
		// 	return
		// }

		for _, c := range conditions {
			// if c.trigger(ctx) {
			//  // shadow c otherwise it could change in the closure
			// 	go func(ctx context, c condition) {
			// 		defer func() {
			// 			if err := recover(); err != nil {
			// 				log.Printf("Recovered from panic in action perform: %v", err)
			// 			}
			// 		}()
			// 		time.Sleep(100 * time.Millisecond)
			// 		err := c.response.perform(ctx)
			// 		if err != nil {
			// 			log.Printf("Error in action perform: %v", err)
			// 		}
			// 	}(ctx, c)
			// }
			if userID == shyronnieID && c.name == "shyronnie" {
				go func() {
					// shadow c because otherwise it could change in the closure
					defer func(c condition) {
						if err := recover(); err != nil {
							log.Printf("Recovered from panic in action on voice state update: %v", err)
						}
						vp := &voicePayload{
							buffer:    c.response.(*voiceAction).buffer,
							channelID: channelID,
						}
						time.AfterFunc(100*time.Millisecond, func() {
							err := me.say(vp, guildID)
							if err != nil {
								log.Printf("Error in speak on voice state update: %v", err)
							}
						})
					}(c)
				}()
				return
			}
		}

	}
	return
}

func getVoiceStateContext(s *discordgo.Session, v *discordgo.VoiceState) (ctx context, err error) {
	ctx.session = s
	ctx.author, err = s.User(v.UserID)
	if err != nil {
		return
	}
	ctx.channel, err = s.Channel(v.ChannelID)
	if err != nil {
		return
	}
	ctx.guild, err = s.Guild(ctx.channel.GuildID)
	if err != nil {
		return
	}
	return
}

func getMessageContext(s *discordgo.Session, m *discordgo.Message) (ctx context, err error) {
	ctx.session = s
	ctx.author = m.Author
	ctx.message = m.Content
	ctx.messageId = m.ID
	ctx.channel, err = s.Channel(m.ChannelID)
	if err != nil {
		return
	}
	ctx.guild, err = s.Guild(ctx.channel.GuildID)
	if err != nil {
		return
	}
	return
}

func isChannelAllowed(channel *discordgo.Channel) bool {
	return true
}

func isAuthorAllowed(author *discordgo.User) bool {
	return author.ID != me.self.ID
}
