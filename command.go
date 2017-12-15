package aoebot

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/bwmarrin/discordgo"
)

type Command interface {
	Name() string
	Usage() string
	Short() string
	Long() string
	IsOwnerOnly() bool
	Run(*Environment, []string) error
	Examples() []string
	Aliases() []string
	Ack(env *Environment) string
}

// BaseCommand is a nop command.
// Real Commands may embed BaseCommand to use its default function implementations.
type BaseCommand struct{}

func (b *BaseCommand) Name() string {
	return ""
}

func (b *BaseCommand) Usage() string {
	return ""
}

func (b *BaseCommand) Short() string {
	return ""
}

func (b *BaseCommand) Long() string {
	return ""
}

func (b *BaseCommand) IsOwnerOnly() bool {
	return false
}

func (b *BaseCommand) Run() error {
	return nil
}

func (b *BaseCommand) Examples() []string {
	return []string{}
}

func (b *BaseCommand) Aliases() []string {
	return []string{}
}

func (b *BaseCommand) Ack(env *Environment) string {
	return ""
}

type Help struct {
	BaseCommand
}

func (h *Help) Name() string {
	return strings.Fields(h.Usage())[0]
}

func (h *Help) Usage() string {
	return `help [-here] [command]`
}

func (h *Help) Short() string {
	return `Get help about my commands`
}

func (h *Help) Long() string {
	return h.Short() + ". Use the [-here] flag to make me respond in the same channel, otherwise I will whisper you."
}

func (h *Help) Examples() []string {
	return []string{
		"help addchannel",
		"help -here addchannel",
	}
}

func (h *Help) Run(env *Environment, args []string) error {
	f := flag.NewFlagSet(h.Name(), flag.ContinueOnError)
	shouldRespondInline := f.Bool("here", false, "respond in same channel")
	if err := f.Parse(args); err != nil {
		return err
	}

	args = f.Args()

	respChannelID := env.TextChannel.ID
	fromDmChannel := env.TextChannel.Type == discordgo.ChannelTypeDM || env.TextChannel.Type == discordgo.ChannelTypeGroupDM
	// open a private msg channel if the message did not come from one
	if !*shouldRespondInline && !fromDmChannel {
		if dm, err := env.Bot.Session.UserChannelCreate(env.Author.ID); err != nil {
			return err
		} else {
			respChannelID = dm.ID
		}
	}

	if len(args) == 1 {
		for _, c := range env.Bot.commands {
			if strings.ToLower(args[0]) == strings.ToLower(c.Name()) {
				embed := helpWithCommandEmbed(env, c)
				_, err := env.Bot.Session.ChannelMessageSendEmbed(respChannelID, embed)
				return err
			}
		}
	}

	embed := h.embed(env)
	_, err := env.Bot.Session.ChannelMessageSendEmbed(respChannelID, embed)
	return err
}

func (h *Help) Ack(env *Environment) string {
	if env.Guild != nil {
		return "📬"
	}
	return ""
}

var helpDescTmpl = template.Must(template.New("helpDesc").Parse(
	"All commands start with `{{.Prefix}}`.\nTo get more help about any command use {{.Prefix}} {{.Usage}}"))

func (h *Help) embed(env *Environment) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}
	embed.Title = h.Name()
	embed.Color = 0x1dd7f8
	if env.Bot.Config.HelpThumbnail != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: env.Bot.Config.HelpThumbnail,
		}
	}
	data := struct {
		Prefix string
		Usage  string
	}{env.Bot.Config.Prefix, h.Usage()}
	buf := &bytes.Buffer{}
	helpDescTmpl.Execute(buf, data)
	embed.Description = buf.String()
	embed.Fields = []*discordgo.MessageEmbedField{}
	if len(h.Examples()) > 0 {
		embed.Fields = append(embed.Fields, examplesEmbedField(env.Bot.Config.Prefix, h.Examples()))
	}
	buf.Reset()
	tw := tabwriter.NewWriter(buf, 4, 4, 0, '.', 0)
	for _, c := range env.Bot.commands {
		if !c.IsOwnerOnly() {
			fmt.Fprintf(tw, "`%s..\t%s`\n", c.Name(), c.Short())
		}
	}
	tw.Flush()
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
	if len(cmd.Examples()) > 0 {
		embed.Fields = append(embed.Fields, examplesEmbedField(env.Bot.Config.Prefix, cmd.Examples()))
	}
	embed.Fields = append(embed.Fields,
		&discordgo.MessageEmbedField{
			Name:  "Description",
			Value: cmd.Long(),
		})
	if len(cmd.Aliases()) > 0 {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Aliases: `%s`", strings.Join(cmd.Aliases(), ", ")),
		}
	}
	return embed
}

func examplesEmbedField(prefix string, examples []string) *discordgo.MessageEmbedField {
	buf := &bytes.Buffer{}
	for _, ex := range examples {
		fmt.Fprintf(buf, "`%s %s`\n", prefix, ex)
	}
	return &discordgo.MessageEmbedField{
		Name:  "Example",
		Value: buf.String(),
	}
}

type Reconnect struct {
	BaseCommand
}

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

func (r *Reconnect) Run(env *Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild")
	}
	env.Bot.Write(env.TextChannel.ID, `Sure thing 🙂`, false)
	env.Bot.speakTo(env.Guild)
	return nil
}

type Restart struct {
	BaseCommand
}

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

type Shutdown struct {
	BaseCommand
}

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
