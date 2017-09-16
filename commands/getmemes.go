package commands

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/jeffreymkabot/aoebot"
)

type GetMemes struct{}

func (g *GetMemes) Name() string {
	return strings.Fields(g.Usage())[0]
}

func (g *GetMemes) Usage() string {
	return `getmemes`
}

func (g *GetMemes) Short() string {
	return `List actions created by add* commands`
}

func (g *GetMemes) Long() string {
	return g.Short() + "."
}

func (g *GetMemes) IsOwnerOnly() bool {
	return false
}

func (g *GetMemes) Run(env *aoebot.Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}
	conds := env.Bot.Driver.ConditionsGuild(env.Guild.ID)
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 4, 0, ' ', 0)
	fmt.Fprintf(w, "```\n")
	for _, c := range conds {
		fmt.Fprintf(w, "%s\n", c.GeneratedName())
	}
	fmt.Fprintf(w, "```\n")
	w.Flush()
	return env.Bot.Write(env.TextChannel.ID, buf.String(), false)
}
