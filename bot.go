/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package main

import (
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"os"
	"os/signal"
	"time"
)

// these live globally for the lifetime of the bot
var (
	selfId      string
	token       string
	voiceQueues map[*discordgo.Guild](chan<- *voicePayload)
)

// dispatch voice data to a particular discord guild
// listen to a queue of voicePayloads for that guild
// voicePayloads provide data meant for a voice channel in a discord guild
// we can remain connected to the same channel while we process a relatively contiguous stream of voicePayloads
// for that channel
func speak(s *discordgo.Session, g *discordgo.Guild) chan<- *voicePayload {
	var vc *discordgo.VoiceConnection
	var err error
	var t *time.Timer
	queue := make(chan *voicePayload)
	defer s.ChannelVoiceJoin(g.ID, g.AfkChannelID, false, true)
	go func() {
		for vp := range queue {
			if t != nil {
				t.Stop()
			}
			fmt.Printf("Speak\n")
			vc, err = s.ChannelVoiceJoin(g.ID, vp.channelID, false, true)
			if err != nil {
				fmt.Printf("Error join channel: %#v\n", err)
				break
			}
			_ = vc.Speaking(true)
			time.Sleep(100 * time.Millisecond)
			for _, sample := range vp.buffer {
				vc.OpusSend <- sample
			}
			time.Sleep(100 * time.Millisecond)
			_ = vc.Speaking(false)
			t = time.AfterFunc(300*time.Millisecond, func() {
				fmt.Printf("Join afk channel\n")
				vc, err = s.ChannelVoiceJoin(g.ID, g.AfkChannelID, true, true)
				if err != nil && vc != nil {
					fmt.Printf("Error join afk: %#v\n", err)
					_ = vc.Speaking(false)
					_ = vc.Disconnect()
					vc = nil
				}
			})
		}
	}()
	return queue
}

func onReady(s *discordgo.Session, r *discordgo.Ready) {
	fmt.Printf("Ready: %#v\n", r)
	voiceQueues = make(map[*discordgo.Guild](chan<- *voicePayload))
	time.Sleep(100 * time.Millisecond)
	for _, g := range r.Guilds {
		// exec independent per each guild g
		q := speak(s, g)
		voiceQueues[g] = q
	}
	s.AddHandler(onMessageCreate)
}

// TODO on channel join ?? ~themesong~

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	fmt.Printf("Saw someone's message: %#v\n", m.Message)
	ctx, err := getMessageContext(s, m.Message)
	if err != nil {
		fmt.Printf("Error resolving message context: %#v\n\n", err)
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

func getMessageContext(s *discordgo.Session, m *discordgo.Message) (context, error) {
	var ctx context
	var err error
	ctx.session = s
	ctx.author = m.Author
	ctx.message = m.Content
	ctx.messageId = m.ID
	ctx.channel, err = s.Channel(m.ChannelID)
	if err != nil {
		return ctx, err
	}
	ctx.guild, err = s.Guild(ctx.channel.GuildID)
	if err != nil {
		return ctx, err
	}
	return ctx, err
}

func isChannelAllowed(channel *discordgo.Channel) bool {
	return true
}

func isAuthorAllowed(author *discordgo.User) bool {
	return author.ID != selfId
}

func main() {
	flag.StringVar(&token, "t", "", "Bot Auth Token")
	flag.Parse()
	if token == "" {
		flag.Usage()
		os.Exit(1)
	}

	// dynamically bind some voice actions
	err := createAoeChatCommands()
	if err != nil {
		fmt.Printf("Error create aoe commands: %#v\n", err)
		return
	}
	fmt.Println("Registered aoe commands")

	err = loadVoiceActionFiles()
	if err != nil {
		fmt.Printf("Error load voice action: %#v\n", err)
		return
	}
	fmt.Println("Loaded voice actions")

	fmt.Println("Initiate discord session")
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Printf("Error initiate session: %#v\n", err)
		return
	}
	fmt.Printf("Got session\n")

	botUser, err := discord.User("@me")
	if err != nil {
		fmt.Printf("Error get user: %#v\n", err)
		return
	}
	fmt.Printf("Got me %#v\n", botUser)
	selfId = botUser.ID

	discord.AddHandler(onReady)
	// listen to discord websocket for events
	// this triggers the ready event on success
	err = discord.Open()
	defer discord.Close()
	if err != nil {
		fmt.Printf("Error opening session: %#v\n", err)
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c // wait on ctrl-C
}
