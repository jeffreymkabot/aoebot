package commands

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jeffreymkabot/aoebot"
)

// TODO worth putting in config file?
const memesPerPage = 100
const memesPerRow = 5

type Memes struct {
	aoebot.BaseCommand
}

func (g *Memes) Name() string {
	return strings.Fields(g.Usage())[0]
}

func (g *Memes) Aliases() []string {
	return []string{"getmemes", "ls", "listmemes", "list"}
}

func (g *Memes) Usage() string {
	return `memes`
}

func (g *Memes) Short() string {
	return `List actions created by add* commands`
}

func (g *Memes) Long() string {
	return g.Short() + "."
}

func (g *Memes) Run(env *aoebot.Environment, args []string) error {
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
	title := strconv.Itoa(len(conds)) + " memes, wow :))"

	for i := 0; i < len(conds); i += memesPerPage {
		end := i + memesPerPage
		if end > len(conds) {
			end = len(conds)
		}

		page := conds[i:end]
		embeds = append(embeds, memesEmbed(page, title, i))
	}
	return
}

func memesEmbed(page []aoebot.Condition, title string, offset int) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}
	embed.Title = title
	embed.Description = strconv.Itoa(offset) + " : " + strconv.Itoa(offset+len(page)-1)
	embed.Fields = []*discordgo.MessageEmbedField{}

	for i := 0; i < len(page); i += memesPerRow {
		end := i + memesPerRow
		if end > len(page) {
			end = len(page)
		}
		row := page[i:end]

		field := &discordgo.MessageEmbedField{Name: strconv.Itoa(i + offset)}
		buf := &bytes.Buffer{}
		for _, cond := range row {
			buf.WriteString(cond.GeneratedName() + "\n")
		}
		field.Value = buf.String()
		embed.Fields = append(embed.Fields, field)
	}
	return embed
}
