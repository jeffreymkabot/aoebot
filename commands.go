package aoebot

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strings"
	"text/tabwriter"
)

// ErrTooManyArguments indicates that a user attempted to invoke a command with too many arguments
var ErrTooManyArguments = errors.New("Command was invoked with too many arguments")

// ErrProtectedCommand indicates that a user attempted to invoke a command they do not have permission for
var ErrProtectedCommand = errors.New("Someone who is not the owner attempted to execute a protected command")

var ErrNoCommand = errors.New("Missing command")

// modeled after golang.org/src/cmd/go/main.Command
type command struct {
	usage       string
	short       string
	long        string
	isProtected bool
	flag        flag.FlagSet
	run         func(*Bot, *Environment, []string) error
}

func (c *command) name() string {
	split := strings.Fields(c.usage)
	return split[0]
}

var help = &command{
	usage: `help [command]`,
	short: `Get help about commands`,
	run: func(b *Bot, env *Environment, args []string) error {
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 0, 4, 0, ' ', 0)
		fmt.Fprintf(w, "```\n")
		if len(args) == 0 {
			fmt.Fprintf(w, "All commands start with \"%s\".\n", b.prefix)
			fmt.Fprintf(w, "For example, \"%s help\".\n\n", b.prefix)
			for _, c := range b.commands {
				fmt.Fprintf(w, "%s    \t%s\n", c.name(), c.short)
			}
			fmt.Fprintf(w, "\nTo get more help about a command use help [command]\n")
		} else if len(args) == 1 {
			for _, c := range b.commands {
				if strings.ToLower(args[0]) == strings.ToLower(c.name()) {
					fmt.Fprintf(w, "Usage: \t%s\n", c.usage)
					if len(c.long) > 0 {
						fmt.Fprintf(w, "%s\n", c.long)
					}
				}
			}
		} else {
			return ErrTooManyArguments
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
	short: `Print runtime infomration`,
	run: func(b *Bot, env *Environment, args []string) error {
		return b.Write(env.TextChannel.ID, b.Stats().String(), false)
	},
}

var reconnect = &command{
	usage:       `reconnect`,
	short:       `Disconnect and make a new voice worker for a guild`,
	isProtected: true,
	run: func(b *Bot, env *Environment, args []string) error {
		if env.Guild != nil {
			_ = b.Write(env.TextChannel.ID, `Sure thing dad 🙂`, false)
			b.speakTo(env.Guild)
			return nil
		}
		return errors.New("no guild")
	},
}

var restart = &command{
	usage:       `restart`,
	short:       `Restart the discord session`,
	isProtected: true,
	run: func(b *Bot, env *Environment, args []string) error {
		_ = b.Write(env.TextChannel.ID, `Okay dad 👀`, false)
		b.Sleep()
		return b.Wakeup()
	},
}

var shutdown = &command{
	usage:       `shutdown [hard]`,
	short:       `Signal to my host application to quit`,
	isProtected: true,
	run: func(b *Bot, env *Environment, args []string) error {
		if len(args) > 0 && strings.ToLower(args[0]) == `hard` {
			_ = b.Write(env.TextChannel.ID, `Are you sure dad? 😳 💤`, false)
			b.die(ErrForceQuit)
		} else {
			_ = b.Write(env.TextChannel.ID, `💀`, false)
			b.die(ErrQuit)
		}
		return nil
	},
}

var addchannel = &command{
	usage: `addchannel [open]`,
	short: `Create a new temproary channel`,
	run: func(b *Bot, env *Environment, args []string) error {
		return nil
	},
}
