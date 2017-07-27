package aoebot

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
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
			fmt.Fprintf(w, "For example, \"%s help\".\n\n", b.prefix)
			for _, c := range b.commands {
				fmt.Fprintf(w, "%s    \t%s\n", c.name(), c.short)
			}
			fmt.Fprintf(w, "\nTo get more help about a command use help [command].\n")
			fmt.Fprintf(w, "More coming soon!\n")
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
		return (&SayAction{
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

var addreact = &command{
	usage:       `addreact`, // addreact [emoji] [phrase], could support regex with a -r flag
	short:       ``,
	long:        ``,
	isProtected: true,
	run: func(b *Bot, env *Environment, args []string) error {
		if len(args) < 2 {
			return errors.New("Not enough arguments")
		}
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild?
		}
		if len(b.driver.ConditionsGuild(env.Guild.ID)) >= MaxGuildCustomConditions {
			return errors.New("I'm not allowed make any more memes in this guild")
		}

		emoji := args[0]
		// immediately try to react to this message with that emoji
		// to verify that the argument passed to the command is a valid emoji for reactions
		err := b.React(env.TextChannel.ID, env.TextMessage.ID, emoji)
		if err != nil {
			if restErr, ok := err.(discordgo.RESTError); ok && restErr.Message != nil {
				return errors.New(restErr.Message.Message)
			}
			return err
		}
		phrase := strings.ToLower(strings.Join(args[1:], " "))
		if len(phrase) < 1 {
			return errors.New("Bad phrase")
		}
		cond := &Condition{
			Name:            fmt.Sprintf("react %s on (%s)", emoji, phrase),
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
	usage:       `delreact`,
	short:       ``,
	long:        ``,
	isProtected: true,
	run: func(b *Bot, env *Environment, args []string) error {
		if len(args) < 2 {
			return errors.New("Not enough arguments")
		}
		if env.Guild == nil {
			return errors.New("No guild") // ErrNoGuild?
		}

		emoji := args[0]

		phrase := strings.ToLower(strings.Join(args[1:], " "))
		if len(phrase) < 1 {
			return errors.New("Bad phrase")
		}

		cond := &Condition{
			Name:            fmt.Sprintf("react %s on (%s)", emoji, phrase),
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
		return nil
	},
}

var addwrite = &command{
	usage: `addwrite`,
	short: ``,
	long:  ``,
}

var addvoice = &command{
	usage: ``,
	short: ``,
	long:  ``,
}

var source = &command{
	usage: `source`,
	short: `Get my source code`,
	long:  ``,
	run: func(b *Bot, env *Environment, args []string) error {
		return b.Write(env.TextChannel.ID, `https://github.com/jeffreymkabot/aoebot/tree/develop`, false)
	},
}
