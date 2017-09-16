package commands

import (
	"strings"

	"github.com/jeffreymkabot/aoebot"
)

type Stats struct{}

func (s *Stats) Name() string {
	return strings.Fields(s.Usage())[0]
}

func (s *Stats) Usage() string {
	return `stats`
}
func (s *Stats) Short() string {
	return `Print runtime information`
}
func (s *Stats) Long() string {
	return s.Short() + "."
}

func (s *Stats) IsOwnerOnly() bool {
	return false
}

func (s *Stats) Run(env *aoebot.Environment, args []string) error {
	return env.Bot.Write(env.TextChannel.ID, env.Bot.Stats().String(), false)
}
