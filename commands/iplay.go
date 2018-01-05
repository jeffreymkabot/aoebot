package commands

import (
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jeffreymkabot/aoebot"
)

type IPlay struct {
	aoebot.BaseCommand
}

func (ip *IPlay) Name() string {
	return strings.Fields(ip.Usage())[0]
}

func (ip *IPlay) Usage() string {
	return "iplay [names]..."
}

func (ip *IPlay) Short() string {
	return "Add yourself to a game"
}

func (ip *IPlay) Long() string {
	return `Give yourself a role that can be mentioned to see who wants to play a game.
E.g. "Who wants to play @overwatch?"
Games need to have been registerd using addgame.  You can refer to games by any registered nickname.
You can give yourself roles for multiple games at the same time.  Separate game names with spaces.
You can remove a role using idontplay.`
}

func (ip *IPlay) Examples() []string {
	return []string{
		"iplay pubg",
		"iplay counterstrike worldofwarcraft",
		"iplay csgo wow dota2 overwatch lol smash pubg",
	}
}

func (ip *IPlay) Ack(env *aoebot.Environment) string {
	return "🆗"
}

func (ip *IPlay) Run(env *aoebot.Environment, args []string) error {
	if len(args) == 0 {
		return errors.New("no games 😦")
	}
	if env.Guild == nil {
		return errors.New("no guild")
	}
	games, missing := aliasesToGames(env.Bot, args)
	if len(missing) > 0 {
		missingStr := "I haven't heard of " + strings.Join(missing, ",") + ". Try using `addgame` for new games."
		env.Bot.Write(env.TextChannel.ID, missingStr, false)
	}
	if len(games) == 0 {
		return errors.New("no games 😦")
	}
	// env.Bot.Session.ChannelTyping(env.TextChannel.ID)
	prefs, err := getGuildPrefs(env.Bot, env.Guild.ID)
	if err != nil {
		return errors.New("couldn't lookup guild data 😦")
	}
	roleIDs := getRolesByGame(env.Bot, prefs, games, true)
	if len(roleIDs) == 0 {
		return errors.New("couldn't get any guild roles 😦")
	}
	for _, roleID := range roleIDs {
		err = env.Bot.Session.GuildMemberRoleAdd(env.Guild.ID, env.Author.ID, roleID)
	}
	return err
}

func aliasesToGames(bot *aoebot.Bot, aliases []string) (games []string, missing []string) {
	for _, alias := range aliases {
		if game := getGameByAlias(bot, alias); game != "" {
			games = append(games, game)
		} else {
			missing = append(missing, alias)
		}
	}
	return
}

// ugh TODO cleanup
// get roleIDs that correspond to a game name in a given guild
// optionally create roles for games that do not have one
func getRolesByGame(bot *aoebot.Bot, prefs *guildPrefs, games []string, createIfMissing bool) (roleIDs []string) {
	createdAnyRoles := false
	for _, game := range games {
		roleID := prefs.GameRoles[game]
		// don't check roleID, ok so we can correct entry if GameRoles actually has an empty string
		// also correct for deleted roles
		if (roleID == "" || !isRoleInGuild(bot, prefs.GuildID, roleID)) && createIfMissing {
			newRoleID, err := createMentionableRole(bot, prefs.GuildID, game)
			if err != nil {
				log.Printf("failed to create role for game %v: %#v", game, err)
			} else {
				createdAnyRoles = true
				prefs.GameRoles[game] = newRoleID
				roleID = newRoleID
			}
		}
		if roleID != "" && isRoleInGuild(bot, prefs.GuildID, roleID) {
			roleIDs = append(roleIDs, roleID)
		}
	}
	if createdAnyRoles {
		setGuildPrefs(bot, prefs)
	}
	return
}

// true when a guild has a role with the provided id
func isRoleInGuild(bot *aoebot.Bot, guildID string, roleID string) bool {
	_, err := bot.Session.State.Role(guildID, roleID)
	return err == nil
}

// ugh TODO cleanup
// create a mentionable role in a guild with a particular name and with the same permissions as @everone
func createMentionableRole(bot *aoebot.Bot, guildID string, name string) (roleID string, err error) {
	data := struct {
		Name        string `json:"name"`
		Mentionable bool   `json:"mentionable"`
	}{name, true}
	resp, err := bot.Session.RequestWithBucketID("POST", discordgo.EndpointGuildRoles(guildID), data, discordgo.EndpointGuildRoles(guildID))
	if err != nil {
		return
	}
	role := discordgo.Role{}
	err = json.Unmarshal(resp, &role)
	if err == nil {
		roleID = role.ID
	}
	return
}

// separate command instead of a flag for user convenience
type IDontPlay struct {
	aoebot.BaseCommand
}

func (idp *IDontPlay) Name() string {
	return strings.Fields(idp.Usage())[0]
}

func (idp *IDontPlay) Usage() string {
	return "idontplay [name]..."
}

func (idp *IDontPlay) Short() string {
	return "Take yourself off a game"
}

func (idp *IDontPlay) Long() string {
	return `Remove a role assigned with addgame.
You can remove multiple roles at the same time.  Separate game names with spaces.`
}

func (idp *IDontPlay) Examples() []string {
	return []string{
		"idontplay hots",
		"idontplay csgo pubg dota2",
	}
}

func (idb *IDontPlay) Ack(env *aoebot.Environment) string {
	return "🆗"
}

func (idp *IDontPlay) Run(env *aoebot.Environment, args []string) error {
	if len(args) == 0 {
		return errors.New("no games 😦")
	}
	if env.Guild == nil {
		return errors.New("no guild")
	}
	games, missing := aliasesToGames(env.Bot, args)
	if len(missing) > 0 {
		missingStr := "I haven't heard of " + strings.Join(missing, ",")
		env.Bot.Write(env.TextChannel.ID, missingStr, false)
	}
	if len(games) == 0 {
		return nil
	}
	prefs, err := getGuildPrefs(env.Bot, env.Guild.ID)
	if err != nil {
		return errors.New("couldn't lookup guild data 😦")
	}
	// don't create missing roles and don't report any errors for missing roles
	roleIDs := getRolesByGame(env.Bot, prefs, games, false)
	for _, roleID := range roleIDs {
		_ = env.Bot.Session.GuildMemberRoleRemove(env.Guild.ID, env.Author.ID, roleID)
	}
	return nil
}
