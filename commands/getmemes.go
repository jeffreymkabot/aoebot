package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jeffreymkabot/aoebot"
)

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
	embed := memesEmbed(conds)
	_, err := env.Bot.Session.ChannelMessageSendEmbed(env.TextChannel.ID, embed)
	return err
}

func memesEmbed(conds []aoebot.Condition) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}
	embed.Title = fmt.Sprintf("%d memes", len(conds))
	embed.Fields = []*discordgo.MessageEmbedField{}

	var page []aoebot.Condition
	for i := 0; i < len(conds); i += 5 {
		page = conds[i : i+5]
		if i + 5 > len(conds) {
			page = conds[i : len(conds)]
		}
		field := &discordgo.MessageEmbedField{Name: strconv.Itoa(i)}
		for _, c := range page {
			field.Value += c.GeneratedName() + "\n"
		}
		embed.Fields = append(embed.Fields, field)
	}
	return embed
}
