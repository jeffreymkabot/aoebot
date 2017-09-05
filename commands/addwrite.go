package commands

import (
	"errors"
	"regexp"
	"strings"
	"github.com/jeffreymkabot/aoebot"
)

var writeCmdRegex = regexp.MustCompile(`^(?:"(\S.*)"|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

type AddWrite struct{}

func (a *AddWrite) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *AddWrite) Usage() string {
	return `addwrite [response] on [phrase]`
}

func (a *AddWrite) Short() string {
	return `Associate a response with a phrase`
}

func (a *AddWrite) Long() string {
	return `Create an automatic response based on the content of phrase.
Phrase is not case-sensitive and needs to match the entire message content to trigger the response.
This is the inverse of the delwrite command.`
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

	var response string
	if len(submatches[1]) > 0 {
		response = submatches[1]
	} else {
		response = submatches[2]
	}
	if len(response) == 0 {
		return errors.New("Bad response")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = strings.ToLower(submatches[3])
	} else {
		phrase = strings.ToLower(submatches[4])
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
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
	return `delwrite [response] on [phrase]`
}

func (d *DelWrite) Short() string {
	return `Unassociate a response with a phrase`
}

func (d *DelWrite) Long() string {
	return `Remove an existing automatic response to a phrase.
This is the inverse of the addwrite command.
For example, an association created by "addwrite who's there? on hello" can be removed with delwrite who's there? on hello".`
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

	var response string
	if len(submatches[1]) > 0 {
		response = submatches[1]
	} else {
		response = submatches[2]
	}
	if len(response) == 0 {
		return errors.New("Bad response")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = strings.ToLower(submatches[3])
	} else {
		phrase = strings.ToLower(submatches[4])
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
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
