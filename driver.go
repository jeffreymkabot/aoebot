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
// Conditions specify properties of Environments that they correspond to
func (d *Driver) actions(env *Environment) []Action {
	coll := d.DB("aoebot").C("conditions")
	query := queryEnvironment(env)
	log.Printf("Using query %s", query)

	conditions := []Condition{}
	err := coll.Find(query).All(&conditions)
	if err != nil {
		log.Printf("Error in query %v", err)
	}

	actions := []Action{}
	for _, cond := range conditions {
		if cond.RegexPhrase != "" && env.TextMessage != nil {
			re, err := regexp.Compile(cond.RegexPhrase)
			if err == nil && re.MatchString(strings.ToLower(env.TextMessage.Content)) {
				actions = append(actions, cond.Action.Action)
			}
		} else {
			actions = append(actions, cond.Action.Action)
		}
	}
	return actions
}

// ConditionsGuild returns all the custom conditions created through the discord message interface
// that are exclusive to a particular guild.
func (d *Driver) ConditionsGuild(guildID string) []Condition {
	coll := d.DB("aoebot").C("conditions")
	// conditions created through discord message interface always have createdby
	query := bson.M{
		"createdby": bson.M{
			"$exists": true,
		},
		"guild":   guildID,
		"enabled": true,
	}

	conditions := []Condition{}
	err := coll.Find(query).All(&conditions)
	if err != nil {
		log.Printf("Error in query guild custom conditions %v", err)
	}
	log.Printf("Found %v custom conditions for guild", len(conditions))
	return conditions
}

// ConditionAdd inserts a new custom condition for a guild.
// ConditionAdd overwites an existing condition with the same environment and action to prevent duplication,
// enabling it if it was disabled.
func (d *Driver) ConditionAdd(c *Condition, creator string) error {
	if creator == "" {
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
	log.Printf("added Condition %#v", info)
	return nil
}

// ConditionDisable disables a condition and any of its duplicates.
func (d *Driver) ConditionDisable(c *Condition) error {
	coll := d.DB("aoebot").C("conditions")
	info, err := coll.UpdateAll(c, bson.M{
		"$set": bson.M{
			"enabled": false,
		},
	})
	if err != nil {
		return err
	}
	log.Printf("disabled Condition %#v", info)
	return nil
}

// Channels retrieves all managed channels registered for any guild.
func (d *Driver) Channels() []channel {
	channels := []channel{}
	coll := d.DB("aoebot").C("channels")
	err := coll.Find(nil).All(&channels)
	if err != nil {
		log.Printf("Error in query managed channels %v", err)
	}
	return channels
}

// ChannelsGuild retrieves all managed channels registered for a particular guild.
func (d *Driver) ChannelsGuild(guildID string) []channel {
	channels := []channel{}
	coll := d.DB("aoebot").C("channels")
	query := bson.M{
		"channel.guildid": guildID,
	}
	err := coll.Find(query).All(&channels)
	if err != nil {
		log.Printf("Error in query guild managed channels %v", err)
	}
	return channels
}

// ChannelsGuild registers a new managed channel.
// Registered managed channels are recovered when the bot restarts.
func (d *Driver) ChannelAdd(ch channel) error {
	coll := d.DB("aoebot").C("channels")
	return coll.Insert(ch)
}

// ChannelDelete unregisters a managed channel.
// Managed channels that are unregistered are lost when the bot restarts.
func (d *Driver) ChannelDelete(channelID ...string) error {
	coll := d.DB("aoebot").C("channels")
	query := bson.M{
		"channel.id": bson.M{
			"$in": channelID,
		},
	}
	return coll.Remove(query)
}

type query bson.M

// make queries pleasant to read in log messages
func (q query) String() string {
	queryjson, _ := json.Marshal(q)
	return string(queryjson)
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

// bson clause { "$in": [value, null] }
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

// Condition defines a set of requirements an environment should meet
// for a particular action to be performed on that environment.
type Condition struct {
	// metadata
	Name      string `json:"name,omitempty" bson:"name,omitempty"`
	IsEnabled bool   `json:"enabled,omitempty" bson:"enabled,omitempty"`
	CreatedBy string `json:"createdby,omitempty" bson:"createdby,omitempty"`

	// requirements
	EnvironmentType EnvironmentType `json:"type" bson:"type"`
	Phrase          string          `json:"phrase,omitempty" bson:"phrase,omitempty"`
	RegexPhrase     string          `json:"regex,omitempty" bson:"regex,omitempty"`
	GuildID         string          `json:"guild,omitempty" bson:"guild,omitempty"`
	TextChannelID   string          `json:"textChannel,omitempty" bson:"textChannel,omitempty"`
	VoiceChannelID  string          `json:"voiceChannel,omitempty" bson:"voiceChannel,omitempty"`
	UserID          string          `json:"user,omitempty" bson:"user,omitempty"`

	// behavior
	Action ActionEnvelope `json:"action" bson:"action"`
}

// GeneratedName standardizes the name of a condition based its requirements and behavior.
// GeneratedName emits a string that can be used as the exact argument to a Del* command.
func (c Condition) GeneratedName() string {
	if c.Action.Type == react {
		if c.RegexPhrase != "" {
			return fmt.Sprintf("%s -regex `%s` on `%s`", c.Action.Type, c.Action.Action, c.RegexPhrase)
		}
		return fmt.Sprintf("%s `%s` on \"`%s`\"", c.Action.Type, c.Action.Action, c.Phrase)
	}
	return fmt.Sprintf("%s \"`%s`\" on \"`%s`\"", c.Action.Type, c.Action.Action, c.Phrase)
}

// ActionEnvelope encapsulates an Action and its ActionType.
// ActionEnvelope is used to unmarshal an action subdocument in a bson payload into the correct type.
type ActionEnvelope struct {
	Type ActionType
	Action
}

// NewActionEnvelope creates an around an Action.
func NewActionEnvelope(a Action) ActionEnvelope {
	return ActionEnvelope{
		Type:   a.kind(),
		Action: a,
	}
}

// ActionTypeMap is used to retrieve an empty concrete Action corresponding to an ActionType.
var ActionTypeMap = map[ActionType]func() Action{
	write: func() Action { return &WriteAction{} },
	voice: func() Action { return &VoiceAction{} },
	react: func() Action { return &ReactAction{} },
}

// SetBSON implements the bson.Setter interface.
// ActionEnvelope needs to be partially unmarshalled into an intermediate struct
// in order to deterimine which concrete type its Action field can be unmarshalled into.
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
