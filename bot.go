/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"
	"flag"
	"github.com/bwmarrin/discordgo"
)

// these live globally for the lifetime of the bot
var (
	selfId string
	token  string
	voiceQueue = make(chan voicePayload)
)

type voicePayload struct {
	buffer [][]byte
	channelId string
	guildId string
}

func transmitVoice(s *discordgo.Session) {
	for vp := range voiceQueue {
		// anonymous immediate to use defer since transmit voice is an infinite loop
		err := func(vp voicePayload) error {
			vc, err := s.ChannelVoiceJoin(vp.guildId, vp.channelId, false, true)
			if err != nil {
				return err
			}
			defer vc.Disconnect()

			_ = vc.Speaking(true)
			defer vc.Speaking(false)

			time.Sleep (100 * time.Millisecond)
			for _, sample := range vp.buffer {
				vc.OpusSend <- sample
			}
			time.Sleep(100 * time.Millisecond)

			return err
		}(vp)
		if (err != nil) {
			fmt.Printf("Error transmit voice: %v\n", err)
		}
	}
}

func getVoiceChannelIdByContext(ctx context) (string) {
	// fmt.Printf("# voice states %v\n", len(ctx.guild.VoiceStates))
	for _, vs := range ctx.guild.VoiceStates {
		if vs.UserID == ctx.author.ID {
			return vs.ChannelID
		}
	}
	return ""
}

// TODO on channel join ?? ~themesong~

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	fmt.Printf("Saw someone's message: %v\n", m.Message)
	ctx, err := getMessageContext(s, m.Message)
	if err != nil {
		fmt.Printf("Error resolving message context: %v\n\n", err)
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

func init() {
	flag.StringVar(&token, "t", "", "Bot Auth Token")
}

func main() {
	flag.Parse()
	if token == "" {
		flag.Usage()
		os.Exit(1)
	}

	// dynamically bind some voice actions
	err := createAoeChatCommands()
	if (err != nil) {
		fmt.Printf("Error create aoe commands: %v\n", err)
	}
	fmt.Println("Registered aoe commands")

	fmt.Println("Initiate discord session")
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Printf("Error initiate session: %v\n", err)
		return
	}
	fmt.Printf("Got session\n")

	botUser, err := discord.User("@me")
	if err != nil {
		fmt.Printf("Error get user: %v\n", err)
		return
	}
	fmt.Printf("Got me %v\n", botUser)

	selfId = botUser.ID

	go transmitVoice(discord)

	// listen to discord websocket for events
	err = discord.Open()
	defer discord.Close()

	if err != nil {
		fmt.Printf("Error opening session: %v\n", err)
	}
	fmt.Printf("Open session\n")

	discord.AddHandler(onMessageCreate)

	// TODO need a context with a channel to which to send message
	// hello := &textAction{
	// 	content: "Hello :)",
	// 	tts: false,
	// }
	// goodbye := &textAction{
	// 	content: "Goodbye :(",
	// 	tts: false,
	// }

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c // wait on ctrl-C
}
