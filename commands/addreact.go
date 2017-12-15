package commands

import (
	"errors"
	"flag"
	"log"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jeffreymkabot/aoebot"
)

// emojis have a special text representation for guild custom emojis
var reactCmdRegexp = regexp.MustCompile(`^(?:<:(\S+:\S+)>|(\S.*)) on "(\S.*)"$`)

func parseReactCmd(arg string, usage string) (emoji string, phrase string, err error) {
	submatches := reactCmdRegexp.FindStringSubmatch(arg)
	if submatches == nil {
		err = errors.New(usage)
		return
	}

	if submatches[1] != "" {
		emoji = submatches[1]
	} else if submatches[2] != "" {
		emoji = submatches[2]
	} else {
		err = errors.New("Couldn't parse emoji")
		return
	}

	if submatches[3] == "" {
		err = errors.New("Couldn't parse phrase")
		return
	}
	phrase = strings.ToLower(submatches[3])
	return
}

type AddReact struct {
	aoebot.BaseCommand
}

func (a *AddReact) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *AddReact) Usage() string {
	return `addreact [-regex] [emoji] on "[phrase]"`
}

func (a *AddReact) Short() string {
	return `Associate an emoji with a phrase`
}

func (a *AddReact) Long() string {
	return `Create an automatic reaction when a message matches [phrase].
[phrase] is not case-sensitive and normally needs to match the entire message.
Alternatively, use the [-regex] flag to treat phrase as a regular expression.
Use a regular expression to match against patterns in the message instead of the entire message.
Supprted regex described here: https://github.com/google/re2/wiki/Syntax.
Reactions can be removed with the delreact command.`
}

func (a *AddReact) Examples() []string {
	return []string{
		`addreact :cat: on "meow"`,
		`addreact -regex :wave: on "^hi(,? aoebot)?[!?]?$"`,
	}
}

func (a *AddReact) Run(env *aoebot.Environment, args []string) error {
	f := flag.NewFlagSet(a.Name(), flag.ContinueOnError)
	isRegex := f.Bool("regex", false, "parse phrase as a regular expression")
	err := f.Parse(args)
	if err != nil {
		return err
	}
	if env.Guild == nil {
		return errors.New("No guild")
	}
	if len(env.Bot.Driver.ConditionsGuild(env.Guild.ID)) >= env.Bot.Config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}

	argString := strings.Join(f.Args(), " ")
	emoji, phrase, err := parseReactCmd(argString, a.Usage())
	if err != nil {
		return err
	}

	log.Printf("Trying emoji %v\n", emoji)
	// immediately try to react to this message with that emoji
	// to verify that the argument passed to the command is a valid emoji for reactions
	unreact, err := env.Bot.React(env.TextChannel.ID, env.TextMessage.ID, emoji)
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
			unreact()
			return err
		}
		cond.RegexPhrase = regexPhrase.String()
	} else {
		cond.Phrase = strings.ToLower(phrase)
	}
	err = env.Bot.Driver.ConditionAdd(cond, env.Author.String())
	if err != nil {
		unreact()
		return err
	}
	return nil
}

func (a *AddReact) Ack(env *aoebot.Environment) string {
	return "✅"
}

type DelReact struct {
	aoebot.BaseCommand
}

func (a *DelReact) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *DelReact) Usage() string {
	return `delreact [-regex] [emoji] on "[phrase]"`
}

func (a *DelReact) Short() string {
	return `Unassociate an emoji with a phrase`
}

func (a *DelReact) Long() string {
	return `Remove an automatic reaction created by addreact.
Use the [-regex] flag if [phrase] should be treated as a regular expression.
Accepted syntax described here: https://github.com/google/re2/wiki/Syntax.`
}

func (d *DelReact) Examples() []string {
	return []string{
		`delreact :cat: on "meow"`,
		`delreact -regex :wave: on "^hi(,? aoebot)?[!?]?$"`,
	}
}

func (a *DelReact) Run(env *aoebot.Environment, args []string) error {
	f := flag.NewFlagSet(a.Name(), flag.ContinueOnError)
	isRegex := f.Bool("regex", false, "parse phrase as a regular expression")
	err := f.Parse(args)
	if err != nil {
		return err
	}
	if env.Guild == nil {
		return errors.New("No guild")
	}

	argString := strings.Join(f.Args(), " ")
	emoji, phrase, err := parseReactCmd(argString, a.Usage())
	if err != nil {
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

	return env.Bot.Driver.ConditionDisable(cond)
}

func (a *DelReact) Ack(env *aoebot.Environment) string {
	return "🗑"
}
