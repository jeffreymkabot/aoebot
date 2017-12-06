package commands

import (
	"errors"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jeffreymkabot/aoebot"
)

// TODO worth putting in config file?
const memesPerPage = 100
const memesPerRow = 5

type GetMemes struct{}

func (g *GetMemes) Name() string {
	return strings.Fields(g.Usage())[0]
}

func (g *GetMemes) Usage() string {
	return `getmemes`
}

func (g *GetMemes) Short() string {
	return `List actions created by add* commands`
}

func (g *GetMemes) Long() string {
	return g.Short() + "."
}

func (g *GetMemes) IsOwnerOnly() bool {
	return false
}

func (g *GetMemes) Run(env *aoebot.Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild")
	}

	conds := env.Bot.Driver.ConditionsGuild(env.Guild.ID)
	if len(conds) == 0 {
		return errors.New("no memes")
	}

	embeds := memesEmbeds(conds)

	for _, embed := range embeds {
		_, err := env.Bot.Session.ChannelMessageSendEmbed(env.TextChannel.ID, embed)
		if err != nil {
			return err
		}
	}

	return nil
}

func memesEmbeds(conds []aoebot.Condition) (embeds []*discordgo.MessageEmbed) {
	title := strconv.Itoa(len(conds)) + " memes :)"

	for i := 0; i < len(conds); i += memesPerPage {
		end := i + memesPerPage
		if end > len(conds) {
			end = len(conds)
		}

		page := conds[i:end]
		desc := strconv.Itoa(i) + " : " + strconv.Itoa(end)

		embeds = append(embeds, memesEmbed(page, title, desc))
	}
	return
}

func memesEmbed(conds []aoebot.Condition, title string, desc string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}
	embed.Title = title
	embed.Description = desc
	embed.Fields = []*discordgo.MessageEmbedField{}

	for i := 0; i < len(conds); i += memesPerRow {
		end := i + memesPerRow
		if end > len(conds) {
			end = len(conds)
		}
		row := conds[i:end]

		field := &discordgo.MessageEmbedField{Name: strconv.Itoa(i)}
		for _, cond := range row {
			field.Value += cond.GeneratedName() + "\n"
		}
		embed.Fields = append(embed.Fields, field)
	}
	return embed
}
