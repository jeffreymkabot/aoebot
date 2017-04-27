/*
aoebot uses a discord bot with token t to connect to your server and recreate the aoe2 chat experience
Inspired by and modeled after github.com/hammerandchisel/airhornbot
*/
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"regexp"
	"math/rand"
	"time"
	// "exec"
	"encoding/binary"
	"flag"
	"github.com/bwmarrin/discordgo"
)

// these live globally for the lifetime of the bot
var (
	selfId string
	token  string
)

var conditions []condition = []condition{
	{
		trigger: "?mayo",
		response: &textAction{
			content: "Is mayonnaise an instrument?",
			tts:     true,
		},
	},
	/*{
		trigger: "30",
		response: &voiceAction{
			file: "media/audio/30 wololo.dca",
		},
	},*/
}

// associate a response to trigger
type condition struct {
	trigger  string
	response action
}

// perform an action given the context (environment) of its trigger
type action interface {
	perform(s *discordgo.Session, ctx context) error
}

type context struct {
	guild   *discordgo.Guild
	channel *discordgo.Channel
	author  *discordgo.User
}

type textAction struct {
	content string
	tts     bool
}

// say something to the text channel of the original context
func (ta *textAction) perform(s *discordgo.Session, ctx context) error {
	var err error
	if ta.tts {
		_, err = s.ChannelMessageSendTTS(ctx.channel.ID, ta.content)
	} else {
		_, err = s.ChannelMessageSend(ctx.channel.ID, ta.content)
	}
	return err
}

type voiceAction struct {
	file   string
	buffer [][]byte
}

// say something to the voice channel of the user in the original context
func (va *voiceAction) perform(s *discordgo.Session, ctx context) error {
	var err error

	err = va.load()
	if err != nil {
		return err
	}

	vcId := getVoiceChannelIdByContext(s, ctx)
	if vcId == "" {
		s.ChannelMessageSend(ctx.channel.ID, "You should be in a voice channel!")
		return nil
	}

	vc, err := s.ChannelVoiceJoin(ctx.guild.ID, vcId, false, true)
	if err != nil {
		return err
	}
	defer vc.Disconnect()

	_ = vc.Speaking(true)
	defer vc.Speaking(false)

	// wait := -300 + rand.Intn(1000)
	// fmt.Printf("Randomly decided to wait %v ms\n", wait)
	// time.Sleep(time.Duration(wait) * time.Millisecond)
	_ = rand.Intn(10)
	time.Sleep (100* time.Millisecond)

	for _, sample := range va.buffer {
		vc.OpusSend <- sample
	}

	time.Sleep(100 * time.Millisecond)

	return err
}

func getVoiceChannelIdByContext(s *discordgo.Session, ctx context) (string) {
	for _, vs := range ctx.guild.VoiceStates {
		if vs.UserID == ctx.author.ID {
			return vs.ChannelID
		}
	}
	return ""
}

// need to user pointer receiver so the load method can modify the voiceAction's internal byte buffer
func (va *voiceAction) load() error {
	va.buffer = make([][]byte, 0)
	file, err := os.Open(va.file)
	if err != nil {
		return err
	}
	defer file.Close()

	var opuslen int16

	for {
		err = binary.Read(file, binary.LittleEndian, &opuslen)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}

		inbuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &inbuf)

		if err != nil {
			return err
		}

		va.buffer = append(va.buffer, inbuf)
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
		// TODO could delegate to func parseContent(s string) string
		// TODO alternative func matchesTrigger(s string, t string) bool
		if strings.ToLower(m.Content) == c.trigger {
			go c.response.perform(s, ctx)
			/*if err != nil {
				fmt.Printf("Error in response: %v\n", err)
			}*/
		}
	}
}

func getMessageContext(s *discordgo.Session, m *discordgo.Message) (context, error) {
	var ctx context
	var err error
	ctx.author = m.Author
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

func createVoiceConditions() error {
	files, err := ioutil.ReadDir("./media/audio")
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`^0*(\d+).*\.dca$`)

	for _, file := range files {
		fname := file.Name()
		if (re.MatchString(fname)) {
			c := condition{
				trigger: re.FindStringSubmatch(fname)[1],
				response: &voiceAction{
					file: "media/audio/" + fname,
				},
			}
			conditions = append(conditions, c)
		}
	}

	return nil
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
	err := createVoiceConditions()
	if (err != nil) {
		fmt.Printf("Error create conditions: %v\n", err)
	}
	fmt.Println("Registered voice conditions")

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

	// listen to discord websocket for events
	err = discord.Open()
	defer discord.Close()

	if err != nil {
		fmt.Printf("Error opening session: %v\n", err)
	}
	fmt.Printf("Open session\n")

	discord.AddHandler(onMessageCreate)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c // wait on ctrl-C
}
