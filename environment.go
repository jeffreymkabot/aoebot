package aoebot

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// EnvironmentType indicates the source of a context
type EnvironmentType int

const (
	Message EnvironmentType = iota
	Voicestate
	Adhoc
)

// Environment captures an environment that can elicit bot actions
// TODO more generic to support capturing the Environment of more events
type Environment struct {
	Bot          *Bot
	Type         EnvironmentType
	Guild        *discordgo.Guild
	TextChannel  *discordgo.Channel
	TextMessage  *discordgo.Message
	VoiceChannel *discordgo.Channel
	Author       *discordgo.User
}

// NewEnvironment creates a new environment based on a seed event/trigger
func NewEnvironment(b *Bot, seed interface{}) (*Environment, error) {
	var err error
	env := &Environment{
		Bot: b,
	}
	switch s := seed.(type) {
	case *discordgo.Message:
		env.Type = Message
		env.TextMessage = s
		env.Author = s.Author
		env.TextChannel, err = b.Session.State.Channel(s.ChannelID)
		if err != nil {
			return nil, err
		}
		if env.TextChannel.Type == discordgo.ChannelTypeGuildText {
			env.Guild, err = b.Session.State.Guild(env.TextChannel.GuildID)
			if err != nil {
				return nil, err
			}
		}
	case *discordgo.VoiceState:
		env.Type = Voicestate
		env.Author, err = b.Session.User(s.UserID)
		if err != nil {
			return nil, err
		}
		env.VoiceChannel, err = b.Session.State.Channel(s.ChannelID)
		if err != nil {
			return nil, err
		}
		env.Guild, err = b.Session.State.Guild(env.VoiceChannel.GuildID)
		if err != nil {
			return nil, err
		}
	default:
		err = fmt.Errorf("Unsupported type %T for context seed", s)
		return nil, err
	}
	return env, nil
}

// Satisfies is true when an environment meets the requirements defined in a Condition
// Some conditions are more specific than others
// Satisfies panics if the regexPhrase in c Condition does not compile
func (env Environment) Satisfies(c Condition) bool {
	typeMatch := env.Type == c.EnvironmentType
	guildMatch := c.GuildID == "" || (env.Guild != nil && env.Guild.ID == c.GuildID)
	userMatch := c.UserID == "" || (env.Author != nil && env.Author.ID == c.UserID)
	textChannelMatch := c.TextChannelID == "" || (env.TextChannel != nil && env.TextChannel.ID == c.TextChannelID)
	voiceChannelMatch := c.VoiceChannelID == "" || (env.VoiceChannel != nil && env.VoiceChannel.ID == c.VoiceChannelID)
	envPhrase := ""
	if env.TextMessage != nil {
		envPhrase = strings.ToLower(env.TextMessage.Content)
	}
	phraseMatch := c.Phrase == "" || envPhrase == c.Phrase
	regexMatch := c.RegexPhrase == "" || regexp.MustCompile(c.RegexPhrase).MatchString(envPhrase)
	return typeMatch && guildMatch && userMatch && textChannelMatch && voiceChannelMatch && phraseMatch && regexMatch
}
