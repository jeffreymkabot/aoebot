package aoebot

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Command interface {
	Name() string
	Usage() string
	Short() string
	Long() string
	IsOwnerOnly() bool
	Run(*Environment, []string) error
}

type help struct {
}

func (h *help) Name() string {
	return strings.Fields(h.Usage())[0]
}

func (h *help) Usage() string {
	return `help [command]`
}

func (h *help) Short() string {
	return `Get help about my commands`
}

func (h *help) Long() string {
	return h.Short() + "." + "."
}

func (h *help) IsOwnerOnly() bool {
	return false
}

func (h *help) Run(env *Environment, args []string) error {
	if len(args) == 1 {
		for _, c := range env.Bot.commands {
			if strings.ToLower(args[0]) == strings.ToLower(c.Name()) {
				embed := &discordgo.MessageEmbed{}
				embed.Title = c.Name()
				embed.Color = 0x00ff80
				if env.Bot.config.HelpThumbnail != "" {
					embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
						URL: env.Bot.config.HelpThumbnail,
					}
				}
				embed.Fields = []*discordgo.MessageEmbedField{
					&discordgo.MessageEmbedField{
						Name:  "Usage",
						Value: c.Usage(),
					},
					&discordgo.MessageEmbedField{
						Name:  "Description",
						Value: c.Long(),
					},
				}
				_, err := env.Bot.session.ChannelMessageSendEmbed(env.TextChannel.ID, embed)
				return err
			}
		}
	}
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 4, 0, ' ', 0)
	fmt.Fprintf(w, "```\n")
	fmt.Fprintf(w, "All commands start with \"%s\".\n", env.Bot.config.Prefix)
	fmt.Fprintf(w, "For example, \"%s help\".\n", env.Bot.config.Prefix)
	fmt.Fprintf(w, "To get more help about a command use: help [command].\n")
	fmt.Fprintf(w, "For example, \"%s help addchannel\".\n", env.Bot.config.Prefix)
	fmt.Fprintf(w, "\n")
	for _, c := range env.Bot.commands {
		if !c.IsOwnerOnly() {
			fmt.Fprintf(w, "%s    \t%s\n", c.Name(), c.Short())
		}
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "```\n")
	w.Flush()
	return env.Bot.Write(env.TextChannel.ID, buf.String(), false)
}

type testreact struct{}

func (tr *testreact) Name() string {
	return strings.Fields(tr.Usage())[0]
}

func (tr *testreact) Usage() string {
	return `testreact`
}

func (tr *testreact) Short() string {
	return `Test that react actions can be dispatched`
}

func (tr *testreact) Long() string {
	return tr.Short()
}

func (tr *testreact) IsOwnerOnly() bool {
	return false
}

func (tr *testreact) Run(env *Environment, args []string) error {
	return (&ReactAction{
		Emoji: `🤖`,
	}).perform(env)
}

type testwrite struct{}

func (tw *testwrite) Name() string {
	return strings.Fields(tw.Usage())[0]
}

func (tw *testwrite) Usage() string {
	return `testwrite`
}

func (tw *testwrite) Short() string {
	return `Test that write actions can be dispatched`
}

func (tw *testwrite) Long() string {
	return tw.Short()
}

func (tw *testwrite) IsOwnerOnly() bool {
	return false
}

func (tw *testwrite) Run(env *Environment, args []string) error {
	return (&WriteAction{
		Content: `Hello World`,
		TTS:     false,
	}).perform(env)
}

type testvoice struct{}

func (tv *testvoice) Name() string {
	return strings.Fields(tv.Usage())[0]
}

func (tv *testvoice) Usage() string {
	return `testvoice`
}

func (tv *testvoice) Short() string {
	return `Test that voice actions can be dispatched`
}

func (tv *testvoice) Long() string {
	return tv.Short()
}

func (tv *testvoice) IsOwnerOnly() bool {
	return false
}

func (tv *testvoice) Run(env *Environment, args []string) error {
	return (&VoiceAction{
		File: `media/audio/40 enemy.dca`,
	}).perform(env)
}

type reconnect struct{}

func (r *reconnect) Name() string {
	return strings.Fields(r.Usage())[0]
}

func (r *reconnect) Usage() string {
	return `reconnect`
}

func (r *reconnect) Short() string {
	return `Refresh the voice worker for this guild`
}

func (r *reconnect) Long() string {
	return r.Short() + "."
}

func (r *reconnect) IsOwnerOnly() bool {
	return true
}

func (r *reconnect) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild")
	}
	_ = env.Bot.Write(env.TextChannel.ID, `Sure thing 🙂`, false)
	env.Bot.speakTo(env.Guild)
	return nil
}

type restart struct{}

func (r *restart) Name() string {
	return strings.Fields(r.Usage())[0]
}

func (r *restart) Usage() string {
	return `restart`
}

func (r *restart) Short() string {
	return `Restart discord session`
}

func (r *restart) Long() string {
	return r.Short() + "."
}

func (r *restart) IsOwnerOnly() bool {
	return true
}

func (r *restart) Run(env *Environment, args []string) error {
	_ = env.Bot.Write(env.TextChannel.ID, `Okay dad 👀`, false)
	env.Bot.Stop()
	return env.Bot.Start()
}

type shutdown struct{}

func (s *shutdown) Name() string {
	return strings.Fields(s.Usage())[0]
}

func (s *shutdown) Usage() string {
	return `shutdown [-hard]`
}

func (s *shutdown) Short() string {
	return `Quit`
}

func (s *shutdown) Long() string {
	return s.Short() + "."
}

func (s *shutdown) IsOwnerOnly() bool {
	return true
}

func (s *shutdown) Run(env *Environment, args []string) error {
	f := flag.NewFlagSet(s.Name(), flag.ContinueOnError)
	isHard := f.Bool("hard", false, "shutdown without cleanup")
	err := f.Parse(args)
	if err != nil && *isHard {
		_ = env.Bot.Write(env.TextChannel.ID, `💀`, false)
		env.Bot.die(ErrForceQuit)
	} else {
		_ = env.Bot.Write(env.TextChannel.ID, `Are you sure dad? 😳 💤`, false)
		env.Bot.die(ErrQuit)
	}
	return nil
}

type Channel *discordgo.Channel

type addchannel struct{}

func (ac *addchannel) Name() string {
	return strings.Fields(ac.Usage())[0]
}

func (ac *addchannel) Usage() string {
	return `addchannel [-openmic] [-users n]`
}

func (ac *addchannel) Short() string {
	return `Create a temporary voice channel`
}

func (ac *addchannel) Long() string {
	return `Create an ad hoc voice channel in this guild.
Use the "openmic" flag to override the channel's "Use Voice Activity" permission.
Use the "users" flag to limit the number of users that can join the channel.
I will automatically delete voice channels when I see they are vacant.
I will only create so many voice channels for each guild.`
}

func (ac *addchannel) IsOwnerOnly() bool {
	return false
}

func (ac *addchannel) Run(env *Environment, args []string) error {
	f := flag.NewFlagSet("addchannel", flag.ContinueOnError)
	isOpen := f.Bool("openmic", false, "permit voice activity")
	userLimit := f.Int("users", 0, "limit users to `n`")
	err := f.Parse(args)
	if err != nil {
		return err
	}

	if env.Guild == nil {
		return errors.New("No guild")
	}
	if len(env.Bot.driver.channelsGuild(env.Guild.ID)) >= env.Bot.config.MaxManagedChannels {
		return errors.New("I'm not allowed to make any more channels in this guild 😦")
	}
	chName := fmt.Sprintf("@!%s", env.Author)
	if *isOpen {
		chName = "open" + chName
	}

	ch, err := env.Bot.session.GuildChannelCreate(env.Guild.ID, chName, `voice`)
	if err != nil {
		return err
	}
	log.Printf("Created channel %s", ch.Name)

	delete := func(ch Channel) {
		log.Printf("Deleting channel %s", ch.Name)
		_, _ = env.Bot.session.ChannelDelete(ch.ID)
		_ = env.Bot.driver.channelDelete(ch.ID)
	}
	err = env.Bot.driver.channelAdd(Channel(ch))
	if err != nil {
		delete(ch)
		return err
	}

	isEmpty := func(ch Channel) bool {
		for _, v := range env.Guild.VoiceStates {
			if v.ChannelID == ch.ID {
				return false
			}
		}
		return true
	}
	interval := time.Duration(env.Bot.config.ManagedChannelPollInterval) * time.Second
	env.Bot.addRoutine(channelManager(ch, delete, isEmpty, interval))

	if *isOpen {
		err = env.Bot.session.ChannelPermissionSet(ch.ID, env.Guild.ID, `role`, discordgo.PermissionVoiceUseVAD, 0)
		if err != nil {
			delete(ch)
			return err
		}
	}
	if userLimit != nil {
		data := struct {
			UserLimit int `json:"user_limit"`
		}{*userLimit}
		_, err = env.Bot.session.RequestWithBucketID("PATCH", discordgo.EndpointChannel(ch.ID), data, discordgo.EndpointChannel(ch.ID))
		if err != nil {
			delete(ch)
			return err
		}
	}
	return nil
}

func channelManager(ch Channel, delete func(ch Channel), isEmpty func(ch Channel) bool, pollInterval time.Duration) func(quit <-chan struct{}) {
	return func(quit <-chan struct{}) {
		for {
			select {
			case <-quit:
				return
			case <-time.After(60 * time.Second):
				if isEmpty(ch) {
					delete(ch)
					return
				}
			}
		}
	}
}

type getmemes struct{}

func (g *getmemes) Name() string {
	return strings.Fields(g.Usage())[0]
}

func (g *getmemes) Usage() string {
	return `getmemes`
}

func (g *getmemes) Short() string {
	return `Get the memes I have on file for this guild`
}

func (g *getmemes) Long() string {
	return g.Short() + "."
}

func (g *getmemes) IsOwnerOnly() bool {
	return false
}

func (g *getmemes) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}
	conds := env.Bot.driver.conditionsGuild(env.Guild.ID)
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 4, 0, ' ', 0)
	fmt.Fprintf(w, "```\n")
	for _, c := range conds {
		fmt.Fprintf(w, "%s\n", c.GeneratedName())
	}
	fmt.Fprintf(w, "```\n")
	w.Flush()
	return env.Bot.Write(env.TextChannel.ID, buf.String(), false)
}

var reactCmdRegex = regexp.MustCompile(`^(?:<:(\S+:\S+)>|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

type addreact struct{}

func (a *addreact) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *addreact) Usage() string {
	return `addreact [-regex] [emoji] on [phrase]`
}

func (a *addreact) Short() string {
	return `Associate an emoji with a phrase`
}

func (a *addreact) Long() string {
	return `Create an automatic message reaction based on the content of a message.
Use the regex flag if "phrase" should be interpretted as a regular expression.
Accepted syntax described here: https://github.com/google/re2/wiki/Syntax
Otherwise, phrase is not case-sensitive and needs to match the entire message content to trigger the reaction.
This is the inverse of the delreact command.`
}

func (a *addreact) IsOwnerOnly() bool {
	return false
}

func (a *addreact) Run(env *Environment, args []string) error {
	f := flag.NewFlagSet(a.Name(), flag.ContinueOnError)
	isRegex := f.Bool("regex", false, "parse phrase as a regular expression")
	err := f.Parse(args)
	if err != nil {
		return err
	}
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}
	if len(env.Bot.driver.conditionsGuild(env.Guild.ID)) >= env.Bot.config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}

	argString := strings.Join(f.Args(), " ")
	if !reactCmdRegex.MatchString(argString) {
		return (&help{}).Run(env, []string{"addreact"})
	}
	submatches := reactCmdRegex.FindStringSubmatch(argString)

	var emoji string
	if len(submatches[1]) > 0 {
		emoji = submatches[1]
	} else {
		emoji = submatches[2]
	}
	if len(emoji) == 0 {
		return errors.New("Bad emoji")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = submatches[3]
	} else {
		phrase = submatches[4]
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	log.Printf("Trying emoji %v\n", emoji)
	// immediately try to react to this message with that emoji
	// to verify that the argument passed to the command is a valid emoji for reactions
	err = env.Bot.React(env.TextChannel.ID, env.TextMessage.ID, emoji)
	if err != nil {
		if restErr, ok := err.(discordgo.RESTError); ok && restErr.Message != nil {
			return errors.New(restErr.Message.Message)
		}
		return err
	}

	cond := &Condition{
		EnvironmentType: message,
		GuildID:         env.Guild.ID,
		Action: NewActionEnvelope(&ReactAction{
			Emoji: emoji,
		}),
	}
	if *isRegex {
		regexPhrase, err := regexp.Compile(phrase)
		if err != nil {
			return err
		}
		cond.RegexPhrase = regexPhrase.String()
	} else {
		cond.Phrase = strings.ToLower(phrase)
	}
	err = env.Bot.driver.conditionAdd(cond, env.Author.String())
	if err != nil {
		return err
	}
	return nil
}

type delreact struct{}

func (a *delreact) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *delreact) Usage() string {
	return `delreact [-regex] [emoji] on [phrase]`
}

func (a *delreact) Short() string {
	return `Unassociate an emoji with a phrase`
}

func (a *delreact) Long() string {
	return `Remove an existing automatic message reaction.
Use the regex flag if "phrase" should be interpretted as a regular expression.
Accepted syntax described here: https://github.com/google/re2/wiki/Syntax
This is the inverse of the addreact command.
For example, an assocation created by "addreact 😊 on hello" can be removed with "delreact 😊 on hello".`
}

func (a *delreact) IsOwnerOnly() bool {
	return false
}

func (a *delreact) Run(env *Environment, args []string) error {
	f := flag.NewFlagSet(a.Name(), flag.ContinueOnError)
	isRegex := f.Bool("regex", false, "parse phrase as a regular expression")
	err := f.Parse(args)
	if err != nil {
		return err
	}
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}

	argString := strings.Join(f.Args(), " ")
	if !reactCmdRegex.MatchString(argString) {
		return (&help{}).Run(env, []string{"delreact"})
	}
	submatches := reactCmdRegex.FindStringSubmatch(argString)

	var emoji string
	if len(submatches[1]) > 0 {
		emoji = submatches[1]
	} else {
		emoji = submatches[2]
	}
	if len(emoji) == 0 {
		return errors.New("Bad emoji")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = submatches[3]
	} else {
		phrase = submatches[4]
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	cond := &Condition{
		EnvironmentType: message,
		GuildID:         env.Guild.ID,
		Action: NewActionEnvelope(&ReactAction{
			Emoji: emoji,
		}),
	}
	if *isRegex {
		regexPhrase, err := regexp.Compile(phrase)
		if err != nil {
			return err
		}
		cond.RegexPhrase = regexPhrase.String()
	} else {
		cond.Phrase = strings.ToLower(phrase)
	}

	err = env.Bot.driver.conditionDelete(cond)
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `🗑️`, false)
	return nil
}

var writeCmdRegex = regexp.MustCompile(`^(?:"(\S.*)"|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

type addwrite struct{}

func (a *addwrite) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *addwrite) Usage() string {
	return `addwrite [response] on [phrase]`
}

func (a *addwrite) Short() string {
	return `Associate a response with a phrase`
}

func (a *addwrite) Long() string {
	return `Create an automatic response based on the content of phrase.
Phrase is not case-sensitive and needs to match the entire message content to trigger the response.
This is the inverse of the delwrite command.`
}

func (a *addwrite) IsOwnerOnly() bool {
	return false
}

func (a *addwrite) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}
	if len(env.Bot.driver.conditionsGuild(env.Guild.ID)) >= env.Bot.config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}

	argString := strings.Join(args, " ")
	if !writeCmdRegex.MatchString(argString) {
		return (&help{}).Run(env, []string{"addwrite"})
	}
	submatches := writeCmdRegex.FindStringSubmatch(argString)

	var response string
	if len(submatches[1]) > 0 {
		response = submatches[1]
	} else {
		response = submatches[2]
	}
	if len(response) == 0 {
		return errors.New("Bad response")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = strings.ToLower(submatches[3])
	} else {
		phrase = strings.ToLower(submatches[4])
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	cond := &Condition{
		EnvironmentType: message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: NewActionEnvelope(&WriteAction{
			Content: response,
		}),
	}

	err := env.Bot.driver.conditionAdd(cond, env.Author.String())
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `+`, false)
	return nil
}

type delwrite struct{}

func (d *delwrite) Name() string {
	return strings.Fields(d.Usage())[0]
}

func (d *delwrite) Usage() string {
	return `delwrite [response] on [phrase]`
}

func (d *delwrite) Short() string {
	return `Unassociate a response with a phrase`
}

func (d *delwrite) Long() string {
	return `Remove an existing automatic response to a phrase.
This is the inverse of the addwrite command.
For example, an association created by "addwrite who's there? on hello" can be removed with delwrite who's there? on hello".`
}

func (d *delwrite) IsOwnerOnly() bool {
	return false
}

func (d *delwrite) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}

	argString := strings.Join(args, " ")
	if !writeCmdRegex.MatchString(argString) {
		return (&help{}).Run(env, []string{"delwrite"})
	}
	submatches := writeCmdRegex.FindStringSubmatch(argString)

	var response string
	if len(submatches[1]) > 0 {
		response = submatches[1]
	} else {
		response = submatches[2]
	}
	if len(response) == 0 {
		return errors.New("Bad response")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = strings.ToLower(submatches[3])
	} else {
		phrase = strings.ToLower(submatches[4])
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	cond := &Condition{
		EnvironmentType: message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: NewActionEnvelope(&WriteAction{
			Content: response,
		}),
	}

	err := env.Bot.driver.conditionDelete(cond)
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `🗑️`, false)
	return nil
}

var addvoiceCmdRegexp = regexp.MustCompile(`^on (?:"(\S.*)"|(\S.*))$`)

type addvoice struct{}

func (a *addvoice) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *addvoice) Usage() string {
	return `addvoice on [phrase]`
}

func (a *addvoice) Short() string {
	return `Associate a sound clip with a phrase`
}

func (a *addvoice) Long() string {
	return `Upload an audio file that can be played in response to a phrase.
You need to attach the audio file to the message that invokes this command.
I will only take the first couple of seconds from the audio file.
Phrase is not case-sensitive and needs to match the entire message content to trigger the response.
This is the inverse of delvoice.`
}

func (a *addvoice) IsOwnerOnly() bool {
	return false
}

func (a *addvoice) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}
	if len(env.Bot.driver.conditionsGuild(env.Guild.ID)) >= env.Bot.config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}
	if len(env.TextMessage.Attachments) == 0 {
		return errors.New("No attached file")
	}

	argString := strings.Join(args, " ")
	if !addvoiceCmdRegexp.MatchString(argString) {
		return (&help{}).Run(env, []string{"addvoice"})
	}
	submatches := addvoiceCmdRegexp.FindStringSubmatch(argString)

	var phrase string
	if len(submatches[1]) > 0 {
		phrase = strings.ToLower(submatches[1])
	} else {
		phrase = strings.ToLower(submatches[2])
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	url := env.TextMessage.Attachments[0].URL
	filename := env.TextMessage.Attachments[0].Filename
	file, err := dcaFromURL(url, filename, env.Bot.config.MaxManagedVoiceDuration)
	if err != nil {
		return err
	}
	cond := &Condition{
		EnvironmentType: message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: NewActionEnvelope(&VoiceAction{
			File: file.Name(),
		}),
	}

	err = env.Bot.driver.conditionAdd(cond, env.Author.String())
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `+`, false)
	return nil
}

func dcaFromURL(url string, fname string, duration int) (f *os.File, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// -t duration arg before -i reads only duration seconds from the input file
	ffmpeg := exec.Command("ffmpeg", "-t", string(duration), "-i", "pipe:0", "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")
	ffmpeg.Stdin = resp.Body
	// ffmpeg.Stederr = os.Stderr
	ffmpegout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return
	}

	dca := exec.Command("./vendor/dca-rs", "--raw", "-i", "pipe:0")
	dca.Stdin = ffmpegout
	// dca.Stdout = os.Stderr

	f, err = os.Create(fmt.Sprintf("./media/audio/%s.dca", fname))
	if err != nil {
		return
	}
	defer f.Close()

	dca.Stdout = f

	err = ffmpeg.Start()
	if err != nil {
		return
	}
	err = dca.Start()
	if err != nil {
		return
	}
	err = dca.Wait()
	if err != nil {
		return
	}
	return
}

var delvoiceCmdRegexp = regexp.MustCompile(`^(?:"(\S.*)"|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

type delvoice struct{}

func (d *delvoice) Name() string {
	return strings.Fields(d.Usage())[0]
}

func (d *delvoice) Usage() string {
	return `delvoice [filename] on [phrase]`
}

func (d *delvoice) Short() string {
	return `Unassociate a sound file with a phrase`
}

func (d *delvoice) Long() string {
	return `Remove an existing association between sound clip and a phrase.
This is the inverse of addvoice.
Files uploaded with addvoice are saved to a relative path and with a new file extension.
The relative path and new file extension can be discovered with the getmemes command.
For example, suppose an assocation is created by uploading the file "greenhillzone.wav" with the command "addvoice on gotta go fast".
The "getmemes" command will show: say ./media/audio/greenhillzone.wav.dca on "gotta go fast".
This assocation can be deleted with "delvoice ./media/audio/greenhillzone.wav.dca on gotta go fast".`
}

func (d *delvoice) IsOwnerOnly() bool {
	return false
}

func (d *delvoice) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild
	}

	argString := strings.Join(args, " ")
	if !delvoiceCmdRegexp.MatchString(argString) {
		return (&help{}).Run(env, []string{"delvoice"})
	}
	submatches := delvoiceCmdRegexp.FindStringSubmatch(argString)

	var filename string
	if len(submatches[1]) > 0 {
		filename = strings.ToLower(submatches[1])
	} else {
		filename = strings.ToLower(submatches[2])
	}
	if len(filename) == 0 {
		return errors.New("Bad filename")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = strings.ToLower(submatches[3])
	} else {
		phrase = strings.ToLower(submatches[4])
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	cond := &Condition{
		EnvironmentType: message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: NewActionEnvelope(&VoiceAction{
			File: filename,
		}),
	}

	err := env.Bot.driver.conditionDelete(cond)
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `🗑️`, false)
	return nil
}

type source struct{}

func (s *source) Name() string {
	return strings.Fields(s.Usage())[0]
}

func (s *source) Usage() string {
	return `source`
}
func (s *source) Short() string {
	return `Get my source code`
}
func (s *source) Long() string {
	return s.Short() + "."
}

func (s *source) IsOwnerOnly() bool {
	return false
}

func (s *source) Run(env *Environment, args []string) error {
	return env.Bot.Write(env.TextChannel.ID, `https://github.com/jeffreymkabot/aoebot/tree/develop`, false)
}

type stats struct{}

func (s *stats) Name() string {
	return strings.Fields(s.Usage())[0]
}

func (s *stats) Usage() string {
	return `stats`
}
func (s *stats) Short() string {
	return `Print runtime information`
}
func (s *stats) Long() string {
	return s.Short() + "."
}

func (s *stats) IsOwnerOnly() bool {
	return false
}

func (s *stats) Run(env *Environment, args []string) error {
	return env.Bot.Write(env.TextChannel.ID, env.Bot.Stats().String(), false)
}
