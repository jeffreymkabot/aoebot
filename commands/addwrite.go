package commands

import (
	"errors"
	"regexp"
	"strings"

	"github.com/jeffreymkabot/aoebot"
)

var writeCmdRegexp = regexp.MustCompile(`^"(\S.*)" on "(\S.*)"$`)

func parseWriteCmd(arg string, usage string) (response string, phrase string, err error) {
	submatches := writeCmdRegexp.FindStringSubmatch(arg)
	if submatches == nil {
		err = errors.New(usage)
		return
	}

	if submatches[1] == "" {
		err = errors.New("Couldn't parse response")
		return
	}
	response = submatches[1]

	if submatches[2] == "" {
		err = errors.New("Couldn't parse phrase")
		return
	}
	phrase = strings.ToLower(submatches[2])
	return
}

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
		return errors.New("No guild")
	}
	if len(env.Bot.Driver.ConditionsGuild(env.Guild.ID)) >= env.Bot.Config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}

	argString := strings.Join(args, " ")
	response, phrase, err := parseWriteCmd(argString, a.Usage())
	if err != nil {
		return err
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: aoebot.NewActionEnvelope(&aoebot.WriteAction{
			Content: response,
		}),
	}

	return env.Bot.Driver.ConditionAdd(cond, env.Author.String())
}

func (a *AddWrite) Ack(env *aoebot.Environment) string {
	return "✅"
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

func (d *DelWrite) Examples() []string {
	return []string{
		`delwrite "who's there?" on "hello"`,
	}
}

func (d *DelWrite) IsOwnerOnly() bool {
	return false
}

func (d *DelWrite) Run(env *aoebot.Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild")
	}

	argString := strings.Join(args, " ")
	response, phrase, err := parseWriteCmd(argString, d.Usage())
	if err != nil {
		return err
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: aoebot.NewActionEnvelope(&aoebot.WriteAction{
			Content: response,
		}),
	}

	return env.Bot.Driver.ConditionDisable(cond)
}

func (a *DelWrite) Ack(env *aoebot.Environment) string {
	return "🗑"
}
