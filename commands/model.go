package commands

import (
	"errors"
	"log"
	"sort"
	"strings"

	"github.com/jeffreymkabot/aoebot"
	"gopkg.in/mgo.v2/bson"
)

type guildPrefs struct {
	GuildID       string            `bson:"guild"`
	SpamChannelID string            `bson:"spam_channel,omitempty"`
	GameRoles     map[string]string `bson:"game_roles,omitempty"`
}

type gameAlias struct {
	Game  string
	Alias string
}

func getGuildPrefs(bot *aoebot.Bot, guildID string) (*guildPrefs, error) {
	coll := bot.Driver.DB("aoebot").C("guilds")
	prefs := &guildPrefs{GuildID: guildID}
	err := coll.Find(prefs).One(&prefs)
	if err == nil && prefs.GameRoles == nil {
		prefs.GameRoles = make(map[string]string)
	}
	return prefs, err
}

func setGuildPrefs(bot *aoebot.Bot, prefs *guildPrefs) error {
	coll := bot.Driver.DB("aoebot").C("guilds")
	query := guildPrefs{GuildID: prefs.GuildID}
	info, err := coll.Upsert(query, bson.M{"$set": prefs})
	if err == nil {
		log.Printf("set guild prefs %#v", info)
	}
	return err
}

// db table has a unique index on alias field
// empty string for not found
func getGameByAlias(bot *aoebot.Bot, alias string) string {
	coll := bot.Driver.DB("aoebot").C("games")
	query := bson.M{"alias": strings.ToLower(alias)}
	ga := gameAlias{}
	coll.Find(query).One(&ga)
	return ga.Game
}

// register a number of aliases for a given game
// overwrite an existing entry for an alias for a different game
func addGameByAliases(bot *aoebot.Bot, game string, aliases ...string) error {
	if game == "" {
		return errors.New("invalid game")
	}
	game = strings.ToLower(game)
	if aliasOf := getGameByAlias(bot, game); aliasOf != "" && aliasOf != game {
		return errors.New(game + " is already a nickname for " + aliasOf)
	}
	coll := bot.Driver.DB("aoebot").C("games")
	for _, alias := range aliases {
		alias = strings.ToLower(alias)
		// silently ignore aliases that start with different letter than game name
		// avoid some intentional collisions between game aliases by mischievous users
		if alias != "" && game[0] == alias[0] {
			if _, err := coll.Upsert(bson.M{"alias": alias}, gameAlias{Game: game, Alias: alias}); err != nil {
				return err
			}
		}
	}
	return nil
}

// get all unique games
func getAllGames(bot *aoebot.Bot) (games []string) {
	coll := bot.Driver.DB("aoebot").C("games")
	coll.Find(nil).Distinct("game", &games)
	sort.Strings(games)
	return
}
