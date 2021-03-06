package commands

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/jeffreymkabot/aoebot"
	"gopkg.in/mgo.v2/bson"
)

type Aoe2 struct {
	aoebot.BaseCommand
}

func (a *Aoe2) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *Aoe2) Aliases() []string {
	return []string{"aoe"}
}

func (a *Aoe2) Usage() string {
	return `aoe2`
}

func (a *Aoe2) Short() string {
	return `List Age of Empires 2 voice taunts`
}

func (a *Aoe2) Long() string {
	return a.Short() + "."
}

func (a *Aoe2) Run(env *aoebot.Environment, args []string) error {
	conditions := []aoebot.Condition{}
	coll := env.Bot.Driver.DB("aoebot").C("conditions")
	query := bson.M{
		"tags": "aoe2",
	}
	err := coll.Find(query).All(&conditions)
	if err != nil {
		return err
	}
	if len(conditions) == 0 {
		return nil
	}
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 4, 0, ' ', 0)
	fmt.Fprintf(w, "```\n")
	for _, c := range conditions {
		fmt.Fprintf(w, "%s    \t%s\n", c.Phrase, c.Name)
	}
	fmt.Fprintf(w, "```\n")
	return env.Bot.Write(env.TextChannel.ID, buf.String(), false)
}
