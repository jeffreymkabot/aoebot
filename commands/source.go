package commands

import (
	"strings"

	"github.com/jeffreymkabot/aoebot"
)

type Source struct{}

func (s *Source) Name() string {
	return strings.Fields(s.Usage())[0]
}

func (s *Source) Usage() string {
	return `source`
}
func (s *Source) Short() string {
	return `Get my source code`
}
func (s *Source) Long() string {
	return s.Short() + "."
}

func (s *Source) IsOwnerOnly() bool {
	return false
}

func (s *Source) Run(env *aoebot.Environment, args []string) error {
	return env.Bot.Write(env.TextChannel.ID, `https://github.com/jeffreymkabot/aoebot/tree/develop`, false)
}
