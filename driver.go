package aoebot

import (
	"encoding/json"
	"fmt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"regexp"
	"strings"
)

// Driver is used by a Bot to discover Actions corresponding to an Environment
// Clients can implement their own driver type to control the behavior of a Bot
type Driver interface {
	Actions(*Environment) []Action
}

// DefaultDriver is the default implementation of the Driver interface
// DefaultDriver is a wrapper around a MongoDB session
// Actions are discovered as subdocments of entries in the "conditions" collection
// Conditions are specify properties of Environments that they correspond to
type DefaultDriver struct {
	*mgo.Session
}

// NewDefaultDriver starts a new MongoDB session
// Clients SHOULD call DefaultDriver.Close() to stop any DefaultDrivers they start
func NewDefaultDriver(dbURL string) (d *DefaultDriver, err error) {
	session, err := mgo.Dial(dbURL)
	d = &DefaultDriver{
		session,
	}
	return
}

// Actions are discovered as subdocments of entries in the "conditions" collection
// Conditions are specify properties of Environments that they correspond to
func (d *DefaultDriver) Actions(env *Environment) []Action {
	actions := []Action{}
	coll := d.DB("aoebot").C("conditions")
	query := queryEnvironment(env)
	log.Printf("Using query %s", query)
	conditions := []Condition{}
	err := coll.Find(query).All(&conditions)
	if err != nil {
		log.Printf("Error in query %v", err)
	}
	for _, c := range conditions {
		if c.RegexPhrase != "" && env.TextMessage != nil {
			if regexp.MustCompile(c.RegexPhrase).MatchString(strings.ToLower(env.TextMessage.Content)) {
				actions = append(actions, c.Action.Action)
			}
		} else {
			actions = append(actions, c.Action.Action)
		}
	}
	return actions
}

type query bson.M

func (q query) String() string {
	queryjson, _ := json.Marshal(q)
	return fmt.Sprintf("%s", queryjson)
}

func queryEnvironment(env *Environment) query {
	and := []bson.M{
		bson.M{
			"enabled": true,
		},
	}
	if env.Guild != nil {
		and = append(and, emptyOrEqual("guild", env.Guild.ID))
	}
	if env.Author != nil {
		and = append(and, emptyOrEqual("user", env.Author.ID))
	}
	if env.TextChannel != nil {
		and = append(and, emptyOrEqual("textChannel", env.TextChannel.ID))
	}
	if env.VoiceChannel != nil {
		and = append(and, emptyOrEqual("textChannel", env.VoiceChannel.ID))
	}
	phrase := ""
	if env.TextMessage != nil {
		phrase = strings.ToLower(env.TextMessage.Content)
	}
	and = append(and, emptyOrEqual("phrase", phrase))
	return query(bson.M{
		"type": env.Type,
		"$and": and,
	})
}

func emptyOrEqual(field string, value interface{}) bson.M {
	return bson.M{
		field: bson.M{
			"$in": []interface{}{
				value,
				nil,
			},
		},
	}
}

// Condition defines a set of requirements an environment should meet for a particular action to be performed on that environment
type Condition struct {
	Name            string          `json:"name"`
	EnvironmentType EnvironmentType `json:"type" bson:"type"`
	Phrase          string          `json:"phrase,omitempty" bson:"phrase,omitempty"`
	RegexPhrase     string          `json:"regex,omitempty" bson:"regex,omitempty"`
	GuildID         string          `json:"guild,omitempty" bson:"guild,omitempty"`
	TextChannelID   string          `json:"textChannel,omitempty" bson:"textChannel,omitempty"`
	VoiceChannelID  string          `json:"voiceChannel,omitempty" bson:"voiceChannel,omitempty"`
	UserID          string          `json:"user,omitempty" bson:"user,omitempty"`
	Action          ActionEnvelope  `json:"action" bson:"action"`
}

// ActionEnvelope encapsulates an Action and its ActionType
type ActionEnvelope struct {
	Type ActionType
	Action
}

// NewActionEnvelope creates an around an Action
// TODO refactor Action.Envelope() ??
func NewActionEnvelope(a Action) ActionEnvelope {
	return ActionEnvelope{
		Type:   a.kind(),
		Action: a,
	}
}

// ActionTypeMap is a one-to-one correspondence between an ActionType and a type implementing Action
// Calling a function retrieved from ActionTypeMap returns a pointer to a concrete instance of that Type
var ActionTypeMap = map[ActionType]func() Action{
	write:     func() Action { return &WriteAction{} },
	say:       func() Action { return &SayAction{} },
	react:     func() Action { return &ReactAction{} },
	stats:     func() Action { return &StatsAction{} },
	reconnect: func() Action { return &ReconnectVoiceAction{} },
	restart:   func() Action { return &RestartAction{} },
	quit:      func() Action { return &QuitAction{} },
}

// SetBSON lets ActionEnvelope implement the bson.Setter interface
// ActionEnvelope needs to have its be partially unmarshalled into an intermediate struct
// in order to deterimine which concrete type its Action field can be unmarshalled into
func (ae *ActionEnvelope) SetBSON(raw bson.Raw) error {
	var err error
	var tmp struct {
		Action bson.Raw
		Type   ActionType
	}
	err = raw.Unmarshal(&tmp)
	if err != nil {
		return err
	}
	if f, ok := ActionTypeMap[tmp.Type]; ok {
		a := f()
		tmp.Action.Unmarshal(a)
		ae.Action = a
		ae.Type = tmp.Type
	} else {
		err = fmt.Errorf("Unsupported action type %v", tmp.Type)
	}
	return err
}
