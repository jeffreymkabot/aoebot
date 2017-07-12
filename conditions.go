package aoebot

import (
	"fmt"
	"gopkg.in/mgo.v2/bson"
)

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
// TODO refactor Action.Envelope()
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
