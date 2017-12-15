package commands

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jeffreymkabot/aoebot"
)

const defaultSides = 20

var sidesRegexp = regexp.MustCompile(`(\d+)`)

type Roll struct {
	aoebot.BaseCommand
}

func (r *Roll) Name() string {
	return strings.Fields(r.Usage())[0]
}

func (r *Roll) Aliases() []string {
	return []string{"rol", "dice", "random"}
}

func (r *Roll) Usage() string {
	return "roll d[n]"
}

func (r *Roll) Short() string {
	return "Roll dice"
}

func (r *Roll) Long() string {
	return "Roll dice."
}

func (r *Roll) Examples() []string {
	return []string{
		"roll d20",
		"roll d100",
	}
}

func (r *Roll) Run(env *aoebot.Environment, args []string) error {
	n := defaultSides
	err := error(nil)
	if len(args) > 0 {
		match := sidesRegexp.FindString(args[0])
		if n, err = strconv.Atoi(match); err != nil {
			return err
		}
	}

	result := rand.Intn(n) + 1
	env.Bot.Session.ChannelTyping(env.TextChannel.ID)
	time.Sleep(1 * time.Second)
	message := fmt.Sprintf("%s rolled %d on a d%d.", env.Author.Username, result, n)
	return env.Bot.Write(env.TextChannel.ID, message, false)
}
