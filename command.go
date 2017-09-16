package aoebot

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strings"
	"text/tabwriter"

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

type CommandWithExamples interface {
	Command
	Examples() []string
}

type Help struct{}

func (h *Help) Name() string {
	return strings.Fields(h.Usage())[0]
}

func (h *Help) Usage() string {
	return `help [command]`
}

func (h *Help) Short() string {
	return `Get help about my commands`
}

func (h *Help) Long() string {
	return h.Short() + "."
}

func (h *Help) Examples() []string {
	return []string{
		`help addchannel`,
	}
}

func (h *Help) IsOwnerOnly() bool {
	return false
}

func (h *Help) Run(env *Environment, args []string) error {
	if len(args) == 1 {
		for _, c := range env.Bot.commands {
			if strings.ToLower(args[0]) == strings.ToLower(c.Name()) {
				embed := &discordgo.MessageEmbed{}
				embed.Title = c.Name()
				embed.Color = 0x00ff80
				if env.Bot.Config.HelpThumbnail != "" {
					embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
						URL: env.Bot.Config.HelpThumbnail,
					}
				}
				embed.Fields = []*discordgo.MessageEmbedField{}
				embed.Fields = append(embed.Fields,
					&discordgo.MessageEmbedField{
						Name:  "Usage",
						Value: fmt.Sprintf("`%s %s`", env.Bot.Config.Prefix, c.Usage()),
					})
				if ce, ok := c.(CommandWithExamples); ok && len(ce.Examples()) > 0 {
					examples := ""
					for _, ex := range ce.Examples() {
						examples += fmt.Sprintf("`%s %s`\n", env.Bot.Config.Prefix, ex)
					}
					embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
						Name:  "Example",
						Value: examples,
					})
				}
				embed.Fields = append(embed.Fields,
					&discordgo.MessageEmbedField{
						Name:  "Description",
						Value: c.Long(),
					})
				_, err := env.Bot.Session.ChannelMessageSendEmbed(env.TextChannel.ID, embed)
				return err
			}
		}
	}
	embed := &discordgo.MessageEmbed{}
	embed.Title = h.Name()
	embed.Color = 0x1dd7f8
	if env.Bot.Config.HelpThumbnail != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: env.Bot.Config.HelpThumbnail,
		}
	}
	embed.Description = fmt.Sprintf("All commands start with `%s`.\n", env.Bot.Config.Prefix)
	embed.Description += fmt.Sprintf("To get more help about any command use `%s %s`.\n", env.Bot.Config.Prefix, h.Usage())
	if len(h.Examples()) > 0 {
		embed.Description += fmt.Sprintf("For example, `%s %s`.\n", env.Bot.Config.Prefix, h.Examples()[0])
	}
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 4, 4, 0, '.', 0)
	for _, c := range env.Bot.commands {
		if !c.IsOwnerOnly() {
			fmt.Fprintf(w, "`%s..\t%s`\n", c.Name(), c.Short())
		}
	}
	w.Flush()
	embed.Fields = []*discordgo.MessageEmbedField{
		&discordgo.MessageEmbedField{
			Name:  "Commands",
			Value: buf.String(),
		},
	}
	_, err := env.Bot.Session.ChannelMessageSendEmbed(env.TextChannel.ID, embed)
	return err
}

type Reconnect struct{}

func (r *Reconnect) Name() string {
	return strings.Fields(r.Usage())[0]
}

func (r *Reconnect) Usage() string {
	return `reconnect`
}

func (r *Reconnect) Short() string {
	return `Refresh the voice worker for this guild`
}

func (r *Reconnect) Long() string {
	return r.Short() + "."
}

func (r *Reconnect) IsOwnerOnly() bool {
	return true
}

func (r *Reconnect) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild")
	}
	_ = env.Bot.Write(env.TextChannel.ID, `Sure thing 🙂`, false)
	env.Bot.speakTo(env.Guild)
	return nil
}

type Restart struct{}

func (r *Restart) Name() string {
	return strings.Fields(r.Usage())[0]
}

func (r *Restart) Usage() string {
	return `restart`
}

func (r *Restart) Short() string {
	return `Restart discord session`
}

func (r *Restart) Long() string {
	return r.Short() + "."
}

func (r *Restart) IsOwnerOnly() bool {
	return true
}

func (r *Restart) Run(env *Environment, args []string) error {
	_ = env.Bot.Write(env.TextChannel.ID, `Okay dad 👀`, false)
	env.Bot.Stop()
	return env.Bot.Start()
}

type Shutdown struct{}

func (s *Shutdown) Name() string {
	return strings.Fields(s.Usage())[0]
}

func (s *Shutdown) Usage() string {
	return `shutdown [-hard]`
}

func (s *Shutdown) Short() string {
	return `Quit`
}

func (s *Shutdown) Long() string {
	return s.Short() + "."
}

func (s *Shutdown) IsOwnerOnly() bool {
	return true
}

func (s *Shutdown) Run(env *Environment, args []string) error {
	f := flag.NewFlagSet(s.Name(), flag.ContinueOnError)
	isHard := f.Bool("hard", false, "shutdown without cleanup")
	err := f.Parse(args)
	if err != nil && *isHard {
		_ = env.Bot.Write(env.TextChannel.ID, `💀`, false)
		env.Bot.Die(ErrForceQuit)
	} else {
		_ = env.Bot.Write(env.TextChannel.ID, `Are you sure dad? 😳 💤`, false)
		env.Bot.Die(ErrQuit)
	}
	return nil
}
