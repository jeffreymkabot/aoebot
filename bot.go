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

// listen on a channel of voicePayload
// voicePayloads provide data meant to be dispatched to a voice channel in a discord guild
// while we process a relatively contiguous stream of voicePayloads we can remain connected to the same channel
func transmitVoice(s * discordgo.Session) {
	var vc *discordgo.VoiceConnection
	var err error
	var ok bool
	for {
		select {
			// 
			case vp := <- voiceQueue:
				if vp.channelId == "" {
					break
				}
				fmt.Printf("exec voice payload\n")
				// current connection in this guild
				vc, ok = s.VoiceConnections[vp.guild.ID]
				// disconnect from any channel we are already in if it isn't the new one
				if ok && vc.ChannelID != vp.channelId {
					vc.Speaking(false)
					vc.Disconnect()
					vc = nil
				}
				if (vc == nil) {
					vc, err = s.ChannelVoiceJoin(vp.guild.ID, vp.channelId, false, true)
					if err != nil {
						fmt.Printf("Error join channel: %v\n", err)
						break
					}
				}
				_ = vc.Speaking(true)
				time.Sleep (100 * time.Millisecond)
				for _, sample := range vp.buffer {
					vc.OpusSend <- sample
				}
				fmt.Printf("sent voice payload\n")
				// wait a little bit before allowing hte possibility of disconnect
				time.Sleep(200 * time.Millisecond)
			// when there is no payload ready to dispatch, disconnect from any open voice connection
			default:
				if (vc != nil) {
					fmt.Printf("disconnect from voice connection\n")
					vc.Speaking(false)
					vc.Disconnect()
					vc = nil
				}
		}
	}
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
		return
	}
	fmt.Println("Registered aoe commands")

	err = loadVoiceActionFiles()
	if (err != nil) {
		fmt.Printf("Error load voice action: %v\n", err)
		return
	}
	fmt.Println("Loaded voice actions")

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
		return
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
