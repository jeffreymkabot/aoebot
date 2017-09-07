package commands

import (
	"errors"
	"flag"
	"github.com/bwmarrin/discordgo"
	"github.com/jeffreymkabot/aoebot"
	"log"
	"regexp"
	"strings"
)

var reactCmdRegex = regexp.MustCompile(`^(?:<:(\S+:\S+)>|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

type AddReact struct{}

func (a *AddReact) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *AddReact) Usage() string {
	return `addreact [-regex] [emoji] on [phrase]`
}

func (a *AddReact) Short() string {
	return `Associate an emoji with a phrase`
}

func (a *AddReact) Long() string {
	return `Create an automatic message reaction based on the content of a message.
Use the regex flag if "phrase" should be interpretted as a regular expression.
Accepted syntax described here: https://github.com/google/re2/wiki/Syntax
Otherwise, phrase is not case-sensitive and needs to match the entire message content to trigger the reaction.
This is the inverse of the delreact command.`
}

func (a *AddReact) Examples() []string {
	return []string{
		`addreact :cat: on "meow"`,
		`addreact -regex :wave: on "^(hello|hi)(,? aoebot)?[!?\.]?$"`,
	}
}

func (a *AddReact) IsOwnerOnly() bool {
	return false
}

func (a *AddReact) Run(env *aoebot.Environment, args []string) error {
	f := flag.NewFlagSet(a.Name(), flag.ContinueOnError)
	isRegex := f.Bool("regex", false, "parse phrase as a regular expression")
	err := f.Parse(args)
	if err != nil {
		return err
	}
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}
	if len(env.Bot.Driver.ConditionsGuild(env.Guild.ID)) >= env.Bot.Config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}

	argString := strings.Join(f.Args(), " ")
	if !reactCmdRegex.MatchString(argString) {
		return (&aoebot.Help{}).Run(env, []string{"addreact"})
	}
	submatches := reactCmdRegex.FindStringSubmatch(argString)

	var emoji string
	if len(submatches[1]) > 0 {
		emoji = submatches[1]
	} else {
		emoji = submatches[2]
	}
	if len(emoji) == 0 {
		return errors.New("Bad emoji")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = submatches[3]
	} else {
		phrase = submatches[4]
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	log.Printf("Trying emoji %v\n", emoji)
	// immediately try to react to this message with that emoji
	// to verify that the argument passed to the command is a valid emoji for reactions
	err = env.Bot.React(env.TextChannel.ID, env.TextMessage.ID, emoji)
	if err != nil {
		if restErr, ok := err.(discordgo.RESTError); ok && restErr.Message != nil {
			return errors.New(restErr.Message.Message)
		}
		return err
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Action: aoebot.NewActionEnvelope(&aoebot.ReactAction{
			Emoji: emoji,
		}),
	}
	if *isRegex {
		regexPhrase, err := regexp.Compile(phrase)
		if err != nil {
			return err
		}
		cond.RegexPhrase = regexPhrase.String()
	} else {
		cond.Phrase = strings.ToLower(phrase)
	}
	err = env.Bot.Driver.ConditionAdd(cond, env.Author.String())
	if err != nil {
		return err
	}
	return nil
}

type DelReact struct{}

func (a *DelReact) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *DelReact) Usage() string {
	return `delreact [-regex] [emoji] on [phrase]`
}

func (a *DelReact) Short() string {
	return `Unassociate an emoji with a phrase`
}

func (a *DelReact) Long() string {
	return `Remove an existing automatic message reaction.
Use the regex flag if "phrase" should be interpretted as a regular expression.
Accepted syntax described here: https://github.com/google/re2/wiki/Syntax
This is the inverse of the addreact command.
For example, an assocation created by "addreact 😊 on hello" can be removed with "delreact 😊 on hello".`
}

func (a *DelReact) IsOwnerOnly() bool {
	return false
}

func (a *DelReact) Run(env *aoebot.Environment, args []string) error {
	f := flag.NewFlagSet(a.Name(), flag.ContinueOnError)
	isRegex := f.Bool("regex", false, "parse phrase as a regular expression")
	err := f.Parse(args)
	if err != nil {
		return err
	}
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}

	argString := strings.Join(f.Args(), " ")
	if !reactCmdRegex.MatchString(argString) {
		return (&aoebot.Help{}).Run(env, []string{"delreact"})
	}
	submatches := reactCmdRegex.FindStringSubmatch(argString)

	var emoji string
	if len(submatches[1]) > 0 {
		emoji = submatches[1]
	} else {
		emoji = submatches[2]
	}
	if len(emoji) == 0 {
		return errors.New("Bad emoji")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = submatches[3]
	} else {
		phrase = submatches[4]
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Action: aoebot.NewActionEnvelope(&aoebot.ReactAction{
			Emoji: emoji,
		}),
	}
	if *isRegex {
		regexPhrase, err := regexp.Compile(phrase)
		if err != nil {
			return err
		}
		cond.RegexPhrase = regexPhrase.String()
	} else {
		cond.Phrase = strings.ToLower(phrase)
	}

	err = env.Bot.Driver.ConditionDelete(cond)
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `🗑️`, false)
	return nil
}
