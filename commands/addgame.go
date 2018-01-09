package commands

import (
	"errors"
	"strings"

	"github.com/jeffreymkabot/aoebot"
)

type AddGame struct {
	aoebot.BaseCommand
}

func (ag *AddGame) Name() string {
	return strings.Fields(ag.Usage())[0]
}

func (ag *AddGame) Usage() string {
	return "addgame [name] [nickname]..."
}

func (ag *AddGame) Short() string {
	return "Create a game role that can be mentioned"
}

func (ag *AddGame) Long() string {
	return `Create a game that can be mentioned to see who wants to play.
[name] should make it obvious what game it is, but shouldn't be super long.
E.g. "overwatch" instead of "ow", but "pubg" instead of "playerunknownsbattlegrounds"
You don't need to give any nicknames, and if you want you can add them later.
Separate names and nicknames with spaces.  Names and nicknames cannot contain spaces.`
}

func (ag *AddGame) Examples() []string {
	return []string{
		"addgame pubg",
		"addgame csgo counterstrike",
		"addgame leagueoflegends league lol",
	}
}

func (ag *AddGame) Run(env *aoebot.Environment, args []string) error {
	if len(args) == 0 {
		return errors.New("no game 😦")
	}

	game := args[0]
	// use the canonical name as at least one alias
	aliases := args
	return addGameByAliases(env.Bot, game, aliases...)
}

func (ag *AddGame) Ack(env *aoebot.Environment) string {
	return "✅"
}

type ListGame struct {
	aoebot.BaseCommand
}

func (lg *ListGame) Aliases() []string {
	return []string{"games"}
}

func (lg *ListGame) Name() string {
	return strings.Fields(lg.Usage())[0]
}

func (lg *ListGame) Usage() string {
	return "listgames"
}

func (lg *ListGame) Short() string {
	return "List games created with addgame"
}

func (lg *ListGame) Run(env *aoebot.Environment, args []string) error {
	games := getAllGames(env.Bot)
	if len(games) == 0 {
		return errors.New("no games")
	}
	msg := "some games\n`" + strings.Join(games, "`\n`") + "`"
	return env.Bot.Write(env.TextChannel.ID, msg, false)
}
