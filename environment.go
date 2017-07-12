package aoebot

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"regexp"
	"strings"
)

// EnvironmentType indicates the source of a context
type EnvironmentType int

const (
	message EnvironmentType = iota
	voicestate
	adhoc
)

// Environment captures an environment that can elicit bot actions
// TODO more generic to support capturing the Environment of more events
type Environment struct {
	Type         EnvironmentType
	Guild        *discordgo.Guild
	TextChannel  *discordgo.Channel
	TextMessage  *discordgo.Message
	VoiceChannel *discordgo.Channel
	Author       *discordgo.User
}

// NewEnvironment creates a new environment based on a seed event/trigger
func NewEnvironment(session *discordgo.Session, seed interface{}) (env *Environment, err error) {
	env = &Environment{}
	switch s := seed.(type) {
	case *discordgo.Message:
		env.Type = message
		env.TextMessage = s
		env.Author = s.Author
		env.TextChannel, err = session.State.Channel(s.ChannelID)
		if err != nil {
			return
		}
		env.Guild, err = session.State.Guild(env.TextChannel.GuildID)
		if err != nil {
			return
		}
	case *discordgo.VoiceState:
		env.Type = voicestate
		env.Author, err = session.User(s.UserID)
		if err != nil {
			return
		}
		env.VoiceChannel, err = session.State.Channel(s.ChannelID)
		if err != nil {
			return
		}
		env.Guild, err = session.State.Guild(env.VoiceChannel.GuildID)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("Unsupported type %T for context seed", s)
		return
	}
	return
}

// Satisfies is true when an environment meets the requirements defined in a Condition
// Some conditions are more specific than others
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
