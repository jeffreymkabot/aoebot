/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package main

import (
	"flag"
	// "fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"time"
)

const (
	mainChannelID  = "140142172979724288"
	memesChannelID = "305119943995686913"
	willowID       = "140136792849514496"
	shyronnieID    = "140898747264663552"
)

// these live globally for the lifetime of the bot
var (
	me bot
	// selfUser              *discordgo.User
	// token                 string
	// voiceQueues           map[*discordgo.Guild](chan<- *voicePayload)
	// voiceQuits            map[*discordgo.Guild](chan<- struct{})
	// voiceChannelOccupancy map[string]string
	// sessionQuit           chan struct{}
)

// dispatch voice data to a particular discord guild
// listen to a queue of voicePayloads for that guild
// voicePayloads provide data meant for a voice channel in a discord guild
// we can remain connected to the same channel while we process a relatively contiguous stream of voicePayloads
// for that channel
// func speak(s *discordgo.Session, g *discordgo.Guild) (chan<- *voicePayload, chan<- struct{}) {
// 	var vc *discordgo.VoiceConnection
// 	var err error

// 	// afk after a certain amount of time not talking
// 	var afkTimer *time.Timer
// 	// disconnect voice after a certain amount of time afk
// 	var dcTimer *time.Timer

// 	// disconnect() and goAfk() get invoked as the function arg in time.AfterFunc()
// 	// need to use closures so they can manipulate the same Session s and VoiceConnection vc used in speak()
// 	disconnect := func() {
// 		if vc != nil {
// 			log.Printf("Disconnect voice in guild: %v", g.ID)
// 			_ = vc.Speaking(false)
// 			_ = vc.Disconnect()
// 			vc = nil
// 		}
// 	}
// 	goAfk := func() {
// 		log.Printf("Join afk channel: %v", g.AfkChannelID)
// 		vc, err = s.ChannelVoiceJoin(g.ID, g.AfkChannelID, true, true)
// 		if err != nil {
// 			log.Printf("Error join afk: %#v\n", err)
// 			disconnect()
// 		} else {
// 			dcTimer = time.AfterFunc(5*time.Minute, disconnect)
// 		}
// 	}
// 	defer goAfk()
// 	queue := make(chan *voicePayload)
// 	quit := make(chan struct{})

// 	go func() {
// 		for {
// 			select {
// 			case vp := <-queue:
// 				if afkTimer != nil {
// 					afkTimer.Stop()
// 				}
// 				if dcTimer != nil {
// 					dcTimer.Stop()
// 				}
// 				log.Printf("Speak\n")
// 				vc, err = s.ChannelVoiceJoin(g.ID, vp.channelID, false, true)
// 				if err != nil {
// 					log.Printf("Error join channel: %#v\n", err)
// 					break
// 				}
// 				_ = vc.Speaking(true)
// 				time.Sleep(100 * time.Millisecond)
// 				for _, sample := range vp.buffer {
// 					vc.OpusSend <- sample
// 				}
// 				time.Sleep(100 * time.Millisecond)
// 				_ = vc.Speaking(false)
// 				afkTimer = time.AfterFunc(300*time.Millisecond, goAfk)
// 			case <-quit:
// 				if afkTimer != nil {
// 					afkTimer.Stop()
// 				}
// 				if dcTimer != nil {
// 					dcTimer.Stop()
// 				}
// 				log.Printf("Quit voice in guild: %v", g.ID)
// 				disconnect()
// 				return
// 			}
// 		}
// 	}()
// 	return queue, quit
// }

func onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Ready: %#v\n", r)
	// voiceQueues = make(map[*discordgo.Guild](chan<- *voicePayload))
	// voiceQuits = make(map[*discordgo.Guild](chan<- struct{}))
	// voiceChannelOccupancy = make(map[string]string)
	me.voiceboxes = make(map[string]*voicebox)
	me.occupancy = make(map[string]string)
	time.Sleep(100 * time.Millisecond)
	for _, g := range r.Guilds {
		// exec independent per each guild g
		me.speakTo(g)
		for _, vs := range g.VoiceStates {
			me.occupancy[vs.UserID] = vs.ChannelID
		}
	}
	s.AddHandler(onMessageCreate)
	s.AddHandler(onVoiceStateUpdate)
	_, _ = s.ChannelMessageSend(mainChannelID, ":sun_with_face::robot::sun_with_face:")
}

// TODO on channel join ?? ~themesong~

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	log.Printf("Saw someone's message: %#v\n", m.Message)
	ctx, err := getMessageContext(s, m.Message)
	if err != nil {
		log.Printf("Error resolving message context: %#v\n\n", err)
	}
	if !isAuthorAllowed(ctx.author) || !isChannelAllowed(ctx.channel) {
		return
	}

	for _, c := range conditions {
		if c.trigger(ctx) {
			go c.response.perform(ctx)
		}
	}
}

// TODO some quick and dirty hard coded experiments for now
func onVoiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	log.Printf("Saw a voice state update: %#v\n", v.VoiceState)
	userID := v.VoiceState.UserID
	guildID := v.VoiceState.GuildID
	guild, _ := s.Guild(guildID)
	channelID := v.VoiceState.ChannelID

	if me.occupancy[userID] != channelID {
		me.occupancy[userID] = channelID
		if channelID == "" {
			return
		}
		// if userID == willowID {
		// 	for _, c := range conditions {
		// 		if c.name == "39" {
		// 			vp := &voicePayload{
		// 				buffer:    c.response.(*voiceAction).buffer,
		// 				channelID: channelID,
		// 				guild:     guild,
		// 			}
		// 			time.AfterFunc(100*time.Millisecond, func() {
		// 				voiceQueues[guild] <- vp
		// 			})
		// 			return
		// 		}
		// 	}
		// }
		if userID == shyronnieID {
			for _, c := range conditions {
				if c.name == "shyronnie" {
					vp := &voicePayload{
						buffer:    c.response.(*voiceAction).buffer,
						channelID: channelID,
						guild:     guild,
					}
					time.AfterFunc(100*time.Millisecond, func() {
						me.voiceboxes[guild.ID].queue <- vp
					})
					return
				}
			}
		}
	}
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

func prepare() (err error) {
	// dynamically bind some voice actions
	err = createAoeChatCommands()
	if err != nil {
		return
	}
	log.Print("Registered aoe commands")

	err = loadVoiceActionFiles()
	if err != nil {
		return
	}
	log.Print("Loaded voice actions")

	return
}

// func wakeup(token string) (session *discordgo.Session, quit chan struct{}, err error) {
// 	session, err = discordgo.New("Bot " + token)
// 	if err != nil {
// 		return
// 	}

// 	selfUser, err = session.User("@me")
// 	if err != nil {
// 		return
// 	}

// 	session.AddHandler(onReady)
// 	// listen to discord websocket for events
// 	// this function triggers the ready event on success
// 	err = session.Open()
// 	if err != nil {
// 		return
// 	}
// 	quit = make(chan struct{})
// 	go func() {
// 		log.Printf("Listening for session quit...")
// 		<-quit
// 		log.Printf("...Got a session quit")
// 		sleep(session)
// 	}()
// 	return
// }

// TODO bot session is a composition that includes discordgo.Session
// func sleep(session *discordgo.Session) {
// 	for _, v := range voiceQuits {
// 		v <- struct{}{}
// 	}
// 	session.Close()
// 	log.Printf("Closed session")
// 	os.Exit(0)
// }

func main() {
	var token string
	flag.StringVar(&token, "t", "", "Bot Auth Token")
	flag.Parse()
	if token == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	err := prepare()
	if err != nil {
		log.Fatalf("Error in prepare: %#v\n", err)
	}

	me = bot{
		token: token,
	}

	err = me.wakeup()
	if err != nil {
		log.Fatalf("Error in wakeup: %#v\n", err)
	}
	defer me.sleep()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c // wait on ctrl-C
}
