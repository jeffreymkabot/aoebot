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
)

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
		me.voiceboxes[g.ID] = me.speakTo(g)
		for _, vs := range g.VoiceStates {
			me.occupancy[vs.UserID] = vs.ChannelID
		}
	}
	s.AddHandler(onMessageCreate)
	s.AddHandler(onVoiceStateUpdate)
	// _, _ = s.ChannelMessageSend(mainChannelID, "hello world")
}

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
	// guild, _ := s.Guild(guildID)
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
						// guild:     guild,
					}
					time.AfterFunc(100*time.Millisecond, func() {
						me.voiceboxes[guildID].queue <- vp
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
	<-c // wait on ctrl-C to prop open main thread
}
