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

// ErrTooManyArguments indicates that a user attempted to invoke a command with too many arguments
var ErrTooManyArguments = errors.New("Command was invoked with too many arguments")

// ErrProtectedCommand indicates that a user attempted to invoke a command they do not have permission for
var ErrProtectedCommand = errors.New("Someone who is not the owner attempted to execute a protected command")

// modeled after golang.org/src/cmd/go/main.Command
type command struct {
	usage       string
	short       string
	long        string
	isProtected bool
	run         func(*Bot, *Environment, []string) error
}

func (c *command) name() string {
	split := strings.Fields(c.usage)
	return split[0]
}

var help = &command{
	usage: `help [command]`,
	short: `Get help about my commands`,
	run: func(b *Bot, env *Environment, args []string) error {
		foundArg := false
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 0, 4, 0, ' ', 0)
		fmt.Fprintf(w, "```\n")
		if len(args) == 1 {
			for _, c := range b.commands {
				if strings.ToLower(args[0]) == strings.ToLower(c.name()) {
					foundArg = true
					fmt.Fprintf(w, "Usage: \t%s\n\n", c.usage)
					if len(c.long) > 0 {
						fmt.Fprintf(w, "%s\n", c.long)
					}
				}
			}
		}
		if !foundArg {
			fmt.Fprintf(w, "All commands start with \"%s\".\n", b.prefix)
			fmt.Fprintf(w, "For example, \"%s help\".\n", b.prefix)
			fmt.Fprintf(w, "To get more help about a command use: help [command].\n")
			fmt.Fprintf(w, "For example, \"%s help addchannel\".\n", b.prefix)
			fmt.Fprintf(w, "\n")
			for _, c := range b.commands {
				if !c.isProtected {
					fmt.Fprintf(w, "%s    \t%s\n", c.name(), c.short)
				}
			}
			fmt.Fprintf(w, "\n")
		}
		fmt.Fprintf(w, "```\n")
		w.Flush()
		return b.Write(env.TextChannel.ID, buf.String(), false)
	},
}

var testwrite = &command{
	usage: `testwrite`,
	short: `Test that write actions can be dispatched`,
	run: func(b *Bot, env *Environment, args []string) error {
		return (&WriteAction{
			Content: `Hello World`,
			TTS:     false,
		}).performFunc(env)(b)
	},
}

var testreact = &command{
	usage: `testreact`,
	short: `Test that react actions can be dispatched`,
	run: func(b *Bot, env *Environment, args []string) error {
		return (&ReactAction{
			Emoji: `🤖`,
		}).performFunc(env)(b)
	},
}

var testvoice = &command{
	usage: `testvoice`,
	short: `Test that voice actions can be dispatched`,
	run: func(b *Bot, env *Environment, args []string) error {
		return (&VoiceAction{
			File: `media/audio/40 enemy.dca`,
		}).performFunc(env)(b)
	},
}

var stats = &command{
	usage: `stats`,
	short: `Print runtime information`,
	run: func(b *Bot, env *Environment, args []string) error {
		return b.Write(env.TextChannel.ID, b.Stats().String(), false)
	},
}

var reconnect = &command{
	usage:       `reconnect`,
	short:       `Disconnect and make a new voice worker for this guild`,
	isProtected: true,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild == nil {
			return errors.New("No guild")
		}
		_ = b.Write(env.TextChannel.ID, `Sure thing 🙂`, false)
		b.speakTo(env.Guild)
		return nil
	},
}

var restart = &command{
	usage:       `restart`,
	short:       `Restart my discord session`,
	isProtected: true,
	run: func(b *Bot, env *Environment, args []string) error {
		_ = b.Write(env.TextChannel.ID, `Okay dad 👀`, false)
		b.Stop()
		return b.Start()
	},
}

var shutdown = &command{
	usage:       `shutdown [-hard]`,
	short:       `Signal to my host application to quit`,
	isProtected: true,
	run: func(b *Bot, env *Environment, args []string) error {
		f := flag.NewFlagSet("shutdown", flag.ContinueOnError)
		isHard := f.Bool("hard", false, "shutdown without cleanup")
		err := f.Parse(args)
		if err != nil && *isHard {
			_ = b.Write(env.TextChannel.ID, `💀`, false)
			b.die(ErrForceQuit)
		} else {
			_ = b.Write(env.TextChannel.ID, `Are you sure dad? 😳 💤`, false)
			b.die(ErrQuit)
		}
		return nil
	},
}

type Channel *discordgo.Channel

var addchannel = &command{
	usage: `addchannel [-openmic] [-users n]`,
	short: `Create a temporary voice channel`,
	long: `Create an ad hoc voice channel in this guild.
	Use the "openmic" flag to override the channel's "Use Voice Activity" permission.
	Use the "users" flag to limit the number of users that can join the channel.
	I will automatically delete voice channels when I see they are vacant.
	I will only create so many voice channels for each guild.`,
	run: func(b *Bot, env *Environment, args []string) error {
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
		if len(b.driver.ChannelsGuild(env.Guild.ID)) >= MaxGuildManagedChannels {
			return errors.New("I'm not allowed to make any more channels in this guild 😦")
		}
		chName := fmt.Sprintf("@!%s", env.Author)
		if *isOpen {
			chName = `open` + chName
		}

		ch, err := b.session.GuildChannelCreate(env.Guild.ID, chName, `voice`)
		if err != nil {
			return err
		}
		log.Printf("Created channel %s", ch.Name)

		delete := func(ch Channel) {
			log.Printf("Deleting channel %s", ch.Name)
			_, _ = b.session.ChannelDelete(ch.ID)
			_ = b.driver.ChannelDelete(ch.ID)
		}
		err = b.driver.ChannelAdd(Channel(ch))
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
		b.addRoutine(channelManager(ch, delete, isEmpty))

		if *isOpen {
			err = b.session.ChannelPermissionSet(ch.ID, env.Guild.ID, `role`, discordgo.PermissionVoiceUseVAD, 0)
			if err != nil {
				delete(ch)
				return err
			}
		}
		if userLimit != nil {
			data := struct {
				UserLimit int `json:"user_limit"`
			}{*userLimit}
			_, err = b.session.RequestWithBucketID("PATCH", discordgo.EndpointChannel(ch.ID), data, discordgo.EndpointChannel(ch.ID))
			if err != nil {
				delete(ch)
				return err
			}
		}
		return nil
	},
}

func channelManager(ch Channel, delete func(ch Channel), isEmpty func(ch Channel) bool) func(quit <-chan struct{}) {
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

var getmemes = &command{
	usage: `getmemes`,
	short: `Get the memes I have on file for this guild`,
	long:  ``,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild?
		}
		conds := b.driver.ConditionsGuild(env.Guild.ID)
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 0, 4, 0, ' ', 0)
		fmt.Fprintf(w, "```\n")
		for _, c := range conds {
			fmt.Fprintf(w, "%s\n", c.GeneratedName())
		}
		fmt.Fprintf(w, "```\n")
		w.Flush()
		return b.Write(env.TextChannel.ID, buf.String(), false)
		// return b.Write(env.TextChannel.ID, `🚧👷✋`, false)
	},
}

var reactRegexp = regexp.MustCompile(`^(?:<:(\S+:\S+)>|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

var addreact = &command{
	usage: `addreact [emoji] on [phrase]`, // could support regex with a -r flag
	short: `Associate an emoji with a phrase`,
	long: `Create an automatic message reaction based on the content of a message.
	Phrase is not case-sensitive and needs to match the entire message content to trigger the reaction.
	This is the inverse of the delreact command.`,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild?
		}
		if len(b.driver.ConditionsGuild(env.Guild.ID)) >= MaxGuildCustomConditions {
			return errors.New("I'm not allowed make any more memes in this guild")
		}

		argString := strings.Join(args, " ")
		if !reactRegexp.MatchString(argString) {
			return help.run(b, env, []string{"addreact"})
		}
		submatches := reactRegexp.FindStringSubmatch(argString)

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
			phrase = strings.ToLower(submatches[3])
		} else {
			phrase = strings.ToLower(submatches[4])
		}
		if len(phrase) == 0 {
			return errors.New("Bad phrase")
		}

		log.Printf("Trying emoji %v\n", emoji)
		// immediately try to react to this message with that emoji
		// to verify that the argument passed to the command is a valid emoji for reactions
		err := b.React(env.TextChannel.ID, env.TextMessage.ID, emoji)
		if err != nil {
			if restErr, ok := err.(discordgo.RESTError); ok && restErr.Message != nil {
				return errors.New(restErr.Message.Message)
			}
			return err
		}

		cond := &Condition{
			EnvironmentType: message,
			GuildID:         env.Guild.ID,
			Phrase:          phrase,
			Action: NewActionEnvelope(&ReactAction{
				Emoji: emoji,
			}),
		}
		err = b.driver.ConditionAdd(cond, env.Author.String())
		if err != nil {
			return err
		}
		return nil
	},
}

var delreact = &command{
	usage: `delreact [emoji] on [phrase]`,
	short: `Unassociate an emoji with a phrase`,
	long: `Remove an existing automatic message reaction.
	This is the inverse of the addreact command.
	For example, an assocation created by "addreact 😊 on hello" can be removed with "delreact 😊 on hello".`,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild?
		}

		argString := strings.Join(args, " ")
		if !reactRegexp.MatchString(argString) {
			return help.run(b, env, []string{"delreact"})
		}
		submatches := reactRegexp.FindStringSubmatch(argString)

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
			Action: NewActionEnvelope(&ReactAction{
				Emoji: emoji,
			}),
		}

		err := b.driver.ConditionDelete(cond)
		if err != nil {
			return err
		}
		_ = b.Write(env.TextChannel.ID, `🗑️`, false)
		return nil
	},
}

var writeRegexp = regexp.MustCompile(`^(?:"(\S.*)"|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

var addwrite = &command{
	usage: `addwrite [response] on [phrase]`,
	short: `Associate a response with a phrase`,
	long: `Create an automatic response based on the content of phrase.
	Phrase is not case-sensitive and needs to match the entire message content to trigger the response.
	This is the inverse of the delwrite command.`,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild?
		}
		if len(b.driver.ConditionsGuild(env.Guild.ID)) >= MaxGuildCustomConditions {
			return errors.New("I'm not allowed make any more memes in this guild")
		}

		argString := strings.Join(args, " ")
		if !writeRegexp.MatchString(argString) {
			return help.run(b, env, []string{"addwrite"})
		}
		submatches := writeRegexp.FindStringSubmatch(argString)

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

		err := b.driver.ConditionAdd(cond, env.Author.String())
		if err != nil {
			return err
		}
		_ = b.Write(env.TextChannel.ID, `+`, false)
		return nil
	},
}

var delwrite = &command{
	usage: `delwrite [response] on [phrase]`,
	short: `Unassociate a response with a phrase`,
	long: `Remove an existing automatic response to a phrase.
	This is the inverse of the addwrite command.
	For example, an association created by "addwrite who's there? on hello" can be removed with delwrite who's there? on hello".`,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild?
		}

		argString := strings.Join(args, " ")
		if !writeRegexp.MatchString(argString) {
			return help.run(b, env, []string{"delwrite"})
		}
		submatches := writeRegexp.FindStringSubmatch(argString)

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

		err := b.driver.ConditionDelete(cond)
		if err != nil {
			return err
		}
		_ = b.Write(env.TextChannel.ID, `🗑️`, false)
		return nil
	},
}

var addvoiceRegexp = regexp.MustCompile(`^on (?:"(\S.*)"|(\S.*))$`)

var addvoice = &command{
	usage: `addvoice on [phrase]`,
	short: `Associate a sound clip with a phrase`,
	long: `Upload an audio file that can be played in response to a phrase.
	I will only take the first couple of seconds from the audio file.
	Phrase is not case-sensitive and needs to match the entire message content to trigger the response.
	This is the inverse of delvoice.`,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild?
		}
		if len(b.driver.ConditionsGuild(env.Guild.ID)) >= MaxGuildCustomConditions {
			return errors.New("I'm not allowed make any more memes in this guild")
		}
		if len(env.TextMessage.Attachments) == 0 {
			return errors.New("No attached file")
		}

		argString := strings.Join(args, " ")
		if !addvoiceRegexp.MatchString(argString) {
			return help.run(b, env, []string{"addvoice"})
		}
		submatches := addvoiceRegexp.FindStringSubmatch(argString)

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
		file, err := dcaFromURL(url, filename)
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

		err = b.driver.ConditionAdd(cond, env.Author.String())
		if err != nil {
			return err
		}
		_ = b.Write(env.TextChannel.ID, `+`, false)
		return nil
	},
}

func dcaFromURL(url string, fname string) (f *os.File, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	duration := fmt.Sprintf("%d", MaxGuildVoiceActionDuration/time.Second)
	// -t duration arg before -i reads only duration seconds from the input file
	ffmpeg := exec.Command("./vendor/ffmpeg", "-t", duration, "-i", "pipe:0", "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")
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

var delvoiceRegexp = regexp.MustCompile(`^(?:"(\S.*)"|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

var delvoice = &command{
	usage: `delvoice [filename] on [phrase]`,
	short: `Unassociate a sound file with a phrase`,
	long: `Remove an existing association between sound clip and a phrase.
	This is the inverse of addvoice.
	Files uploaded with addvoice are saved to a relative path and with a new file extension.
	The relative path and new file extension can be discovered with the getmemes command.
	For example, suppose an assocation is created by uploading the file "greenhillzone.wav" with the command "addvoice on gotta go fast".
	The "getmemes" command will show: say ./media/audio/greenhillzone.wav.dca on "gotta go fast".
	This assocation can be deleted with "delvoice ./media/audio/greenhillzone.wav.dca on gotta go fast".`,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild
		}

		argString := strings.Join(args, " ")
		if !delvoiceRegexp.MatchString(argString) {
			return help.run(b, env, []string{"delvoice"})
		}
		submatches := delvoiceRegexp.FindStringSubmatch(argString)

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

		err := b.driver.ConditionDelete(cond)
		if err != nil {
			return err
		}
		_ = b.Write(env.TextChannel.ID, `🗑️`, false)
		return nil
	},
}

var source = &command{
	usage: `source`,
	short: `Get my source code`,
	long:  ``,
	run: func(b *Bot, env *Environment, args []string) error {
		return b.Write(env.TextChannel.ID, `https://github.com/jeffreymkabot/aoebot/tree/develop`, false)
	},
}
