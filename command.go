package aoebot

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
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

type CommandWithAck interface {
	Command
	Ack(env *Environment) string
}

type CommandWithAliases interface {
	Command
	Aliases() []string
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
	var dm *discordgo.Channel
	var err error
	if env.TextChannel.Type == discordgo.ChannelTypeDM || env.TextChannel.Type == discordgo.ChannelTypeGroupDM {
		log.Print("Reusing current dm channel")
		dm = env.TextChannel
	} else if dm, err = env.Bot.Session.UserChannelCreate(env.Author.ID); err != nil {
		return err
	}

	if len(args) == 1 {
		for _, c := range env.Bot.commands {
			if strings.ToLower(args[0]) == strings.ToLower(c.Name()) {
				embed := helpWithCommandEmbed(env, c)
				_, err = env.Bot.Session.ChannelMessageSendEmbed(dm.ID, embed)
				return err
			}
		}
	}

	embed := h.embed(env)
	_, err = env.Bot.Session.ChannelMessageSendEmbed(dm.ID, embed)
	return err
}

func (h *Help) Ack(env *Environment) string {
	if env.Guild != nil {
		return "📬"
	}
	return ""
}

func (h *Help) embed(env *Environment) *discordgo.MessageEmbed {
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
	embed.Fields = []*discordgo.MessageEmbedField{}
	if len(h.Examples()) > 0 {
		embed.Fields = append(embed.Fields, examplesEmbedField(env.Bot.Config.Prefix, h.Examples()))
	}
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 4, 4, 0, '.', 0)
	for _, c := range env.Bot.commands {
		if !c.IsOwnerOnly() {
			fmt.Fprintf(w, "`%s..\t%s`\n", c.Name(), c.Short())
		}
	}
	w.Flush()
	embed.Fields = append(embed.Fields,
		&discordgo.MessageEmbedField{
			Name:  "Commands",
			Value: buf.String(),
		})
	return embed
}

func helpWithCommandEmbed(env *Environment, cmd Command) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}
	embed.Title = cmd.Name()
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
			Value: fmt.Sprintf("`%s %s`", env.Bot.Config.Prefix, cmd.Usage()),
		})
	if cmdEx, ok := cmd.(CommandWithExamples); ok && len(cmdEx.Examples()) > 0 {
		embed.Fields = append(embed.Fields, examplesEmbedField(env.Bot.Config.Prefix, cmdEx.Examples()))
	}
	embed.Fields = append(embed.Fields,
		&discordgo.MessageEmbedField{
			Name:  "Description",
			Value: cmd.Long(),
		})
	if cmdAlias, ok := cmd.(CommandWithAliases); ok && len(cmdAlias.Aliases()) > 0 {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: "Aliases: `" + strings.Join(cmdAlias.Aliases(), ", ") + "`",
		}
	}
	return embed
}

func examplesEmbedField(prefix string, examples []string) *discordgo.MessageEmbedField {
	list := ""
	for _, ex := range examples {
		list += fmt.Sprintf("`%s %s`\n", prefix, ex)
	}
	return &discordgo.MessageEmbedField{
		Name:  "Example",
		Value: list,
	}
}

type Reconnect struct{}

func (r *Reconnect) Name() string {
	return strings.Fields(r.Usage())[0]
}

func (r *Reconnect) Usage() string {
	return `reconnect`
}

func (r *Reconnect) Short() string {
	return `Refresh the voice player for this guild`
}

func (r *Reconnect) Long() string {
	return r.Short() + "."
}

func (r *Reconnect) IsOwnerOnly() bool {
	return false
}

func (r *Reconnect) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild")
	}
	env.Bot.Write(env.TextChannel.ID, `Sure thing 🙂`, false)
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
	env.Bot.Write(env.TextChannel.ID, `Okay dad 👀`, false)
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
		env.Bot.Write(env.TextChannel.ID, `💀`, false)
		env.Bot.Die(ErrForceQuit)
	} else {
		env.Bot.Write(env.TextChannel.ID, `Are you sure dad? 😳 💤`, false)
		env.Bot.Die(ErrQuit)
	}
	return nil
}
