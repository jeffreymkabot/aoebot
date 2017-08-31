package aoebot

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Driver is a wrapper around a MongoDB session
// Actions are discovered as subdocments of entries in the "conditions" collection
// Conditions specify properties of Environments that they correspond to
type Driver struct {
	*mgo.Session
}

// newDriver starts a new MongoDB session
// Clients SHOULD call Driver.Close() to stop any Drivers they start
func newDriver(dbURL string) (d *Driver, err error) {
	session, err := mgo.Dial(dbURL)
	d = &Driver{
		session,
	}
	return
}

// actions are discovered as subdocments of entries in the "conditions" collection
// Conditions are specify properties of Environments that they correspond to
func (d *Driver) actions(env *Environment) []Action {
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

func (d *Driver) conditionsGuild(guildID string) []Condition {
	conditions := []Condition{}
	coll := d.DB("aoebot").C("conditions")
	query := bson.M{
		"createdby": bson.M{
			"$exists": true,
		},
		"guild":   guildID,
		"enabled": true,
	}
	err := coll.Find(query).All(&conditions)
	if err != nil {
		log.Printf("Error in query guild custom conditions %v", err)
	}
	log.Printf("Found %v custom conditions for guild", len(conditions))
	return conditions
}

func (d *Driver) conditionAdd(c *Condition, creator string) error {
	if len(creator) < 1 {
		return errors.New("Creator name is too short")
	}
	coll := d.DB("aoebot").C("conditions")
	info, err := coll.Upsert(c, bson.M{
		"$set": bson.M{
			"name":      c.GeneratedName(),
			"createdby": creator,
			"enabled":   true,
		},
	})
	if err != nil {
		return err
	}
	log.Printf("Added Condition %#v", info)
	return nil
}

func (d *Driver) conditionDelete(c *Condition) error {
	coll := d.DB("aoebot").C("conditions")
	err := coll.Remove(c)
	return err
}

func (d *Driver) channels() []Channel {
	channels := []Channel{}
	coll := d.DB("aoebot").C("channels")
	err := coll.Find(nil).All(&channels)
	if err != nil {
		log.Printf("Error in query managed channels %v", err)
	}
	return channels
}

func (d *Driver) channelsGuild(guildID string) []Channel {
	channels := []Channel{}
	coll := d.DB("aoebot").C("channels")
	query := bson.M{
		"guildid": guildID,
	}
	err := coll.Find(query).All(&channels)
	if err != nil {
		log.Printf("Error in query guild managed channels %v", err)
	}
	return channels
}

func (d *Driver) channelAdd(ch Channel) error {
	coll := d.DB("aoebot").C("channels")
	err := coll.Insert(ch)
	return err
}

func (d *Driver) channelDelete(channelID ...string) error {
	coll := d.DB("aoebot").C("channels")
	query := bson.M{
		"id": bson.M{
			"$in": channelID,
		},
	}
	err := coll.Remove(query)
	return err
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
	Name            string          `json:"name,omitempty" bson:"name,omitempty"`
	IsEnabled       bool            `json:"enabled,omitempty" bson:"enabled,omitempty"`
	CreatedBy       string          `json:"createdby,omitempty" bson:"createdby,omitempty"`
	EnvironmentType EnvironmentType `json:"type" bson:"type"`
	Phrase          string          `json:"phrase,omitempty" bson:"phrase,omitempty"`
	RegexPhrase     string          `json:"regex,omitempty" bson:"regex,omitempty"`
	GuildID         string          `json:"guild,omitempty" bson:"guild,omitempty"`
	TextChannelID   string          `json:"textChannel,omitempty" bson:"textChannel,omitempty"`
	VoiceChannelID  string          `json:"voiceChannel,omitempty" bson:"voiceChannel,omitempty"`
	UserID          string          `json:"user,omitempty" bson:"user,omitempty"`
	Action          ActionEnvelope  `json:"action" bson:"action"`
}

func (c Condition) GeneratedName() string {
	isRegex := c.RegexPhrase != ""
	if isRegex {
		return fmt.Sprintf("%s \t%s \ton \t\"%s\"", c.Action.Type, c.Action.Action, c.RegexPhrase)
	}
	return fmt.Sprintf("%s \t%s \ton \t\"%s\"", c.Action.Type, c.Action.Action, c.Phrase)
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
	write: func() Action { return &WriteAction{} },
	voice: func() Action { return &VoiceAction{} },
	react: func() Action { return &ReactAction{} },
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
