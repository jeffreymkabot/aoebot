package commands

import (
	"errors"
	"github.com/jeffreymkabot/aoebot"
	"regexp"
	"strings"
)

var writeCmdRegex = regexp.MustCompile(`^"(\S.*)" on "(\S.*)"$`)

type AddWrite struct{}

func (a *AddWrite) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *AddWrite) Usage() string {
	return `addwrite "[response]" on "[phrase]"`
}

func (a *AddWrite) Short() string {
	return `Associate a response with a phrase`
}

func (a *AddWrite) Long() string {
	return `Create an automatic response when a message matches [phrase].
[phrase] is not case-sensitive and needs to match the entire message content to trigger the response.
Responses can be removed with the delwrite command.`
}

func (a *AddWrite) Examples() []string {
	return []string{
		`addwrite "pong" on "ping"`,
		`addwrite ":alien: ayy lmao :alien:" on "it's dat boi"`,
	}
}

func (a *AddWrite) IsOwnerOnly() bool {
	return false
}

func (a *AddWrite) Run(env *aoebot.Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}
	if len(env.Bot.Driver.ConditionsGuild(env.Guild.ID)) >= env.Bot.Config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}

	argString := strings.Join(args, " ")
	if !writeCmdRegex.MatchString(argString) {
		return (&aoebot.Help{}).Run(env, []string{"addwrite"})
	}
	submatches := writeCmdRegex.FindStringSubmatch(argString)

	response := submatches[1]
	if len(response) == 0 {
		return errors.New("Couldn't parse response")
	}

	phrase := strings.ToLower(submatches[2])
	if len(phrase) == 0 {
		return errors.New("Couldn't parse phrase")
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: aoebot.NewActionEnvelope(&aoebot.WriteAction{
			Content: response,
		}),
	}

	err := env.Bot.Driver.ConditionAdd(cond, env.Author.String())
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `+`, false)
	return nil
}

type DelWrite struct{}

func (d *DelWrite) Name() string {
	return strings.Fields(d.Usage())[0]
}

func (d *DelWrite) Usage() string {
	return `delwrite "[response]" on "[phrase]"`
}

func (d *DelWrite) Short() string {
	return `Unassociate a response with a phrase`
}

func (d *DelWrite) Long() string {
	return `Remove an automatic response created by addwrite.`
}

func (d *DelWrite) Examples() []string{
	return []string{
		`delwrite "who's there?" on "hello"`,
	}
}

func (d *DelWrite) IsOwnerOnly() bool {
	return false
}

func (d *DelWrite) Run(env *aoebot.Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}

	argString := strings.Join(args, " ")
	if !writeCmdRegex.MatchString(argString) {
		return (&aoebot.Help{}).Run(env, []string{"delwrite"})
	}
	submatches := writeCmdRegex.FindStringSubmatch(argString)

	response := submatches[1]
	if len(response) == 0 {
		return errors.New("Couldn't parse response")
	}

	phrase := strings.ToLower(submatches[2])
	if len(phrase) == 0 {
		return errors.New("Couldn't parse phrase")
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: aoebot.NewActionEnvelope(&aoebot.WriteAction{
			Content: response,
		}),
	}

	err := env.Bot.Driver.ConditionDelete(cond)
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `🗑️`, false)
	return nil
}
