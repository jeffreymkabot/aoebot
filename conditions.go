package main

import (
	"fmt"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"regexp"
)

// Condition defines a set of requirements an environment should meet for an action to be performed on that environment
type Condition struct {
	Name string `json:"name"`
	// e.g. MessageContext, VoiceStateContext
	ContextType    int            `json:"ctype" bson:"ctype"`
	Phrase         string         `json:"phrase,omitempty" bson:"phrase,omitempty"`
	IsRegex        bool           `json:"isRegex,omitempty" bson:"isRegex,omitempty"`
	GuildID        string         `json:"guild,omitempty" bson:"guild,omitempty"`
	TextChannelID  string         `json:"textChannel,omitempty" bson:"textChannel,omitempty"`
	VoiceChannelID string         `json:"voiceChannel,omitempty" bson:"voiceChannel,omitempty"`
	UserID         string         `json:"user,omitempty" bson:"user,omitempty"`
	Action         ActionEnvelope `json:"action" bson:"action"`
}

type ActionType string

const (
	write     ActionType = "write"
	say       ActionType = "say"
	react     ActionType = "react"
	stats     ActionType = "stats"
	reconnect ActionType = "reconnect"
	restart   ActionType = "restart"
	quit      ActionType = "quit"
)

var ActionF = map[ActionType]func() Action{
	write:   func() Action { return &WriteAction{} },
	say:     func() Action { return &SayAction{} },
	react:   func() Action { return &ReactAction{} },
	stats:   func() Action { return &StatsAction{} },
	restart: func() Action { return &RestartAction{} },
	quit:    func() Action { return &QuitAction{} },
}

type ActionEnvelope struct {
	Action
	Type ActionType
}

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
	if f, ok := ActionF[tmp.Type]; ok {
		a := f()
		tmp.Action.Unmarshal(&a)
		ae.Action = a
		ae.Type = tmp.Type
	} else {
		err = fmt.Errorf("Unsupported action type %v", tmp.Type)
	}
	return err
}

var conditions = []Condition{
	{
		ContextType: MessageContext,
		Phrase:      `?testwrite`,
		Action: ActionEnvelope{
			Type: write,
			Action: &WriteAction{
				Content: `hello world`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `?testvoice`,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/40 enemy.dca`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `?testreact`,
		Action: ActionEnvelope{
			Type: react,
			Action: &ReactAction{
				Emoji: `🤖`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\baoebot\b`,
		IsRegex:     true,
		Action: ActionEnvelope{
			Type: react,
			Action: &ReactAction{
				Emoji: `🤖`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bheroes of the storm\b`,
		IsRegex:     true,
		Action: ActionEnvelope{
			Type: write,
			Action: &WriteAction{
				Content: `🤢`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bhots\b`,
		IsRegex:     true,
		Action: ActionEnvelope{
			Type: react,
			Action: &ReactAction{
				Emoji: `🤢`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bsmash\b`,
		IsRegex:     true,
		Action: ActionEnvelope{
			Type: write,
			Action: &WriteAction{
				Content: `Smash that ready button!`,
				TTS:     true,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bbruh\b`,
		IsRegex:     true,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/H3H3_BRUH.dca`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bnice shades\b`,
		IsRegex:     true,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/my_vision_is_augmented.dca`,
			},
		},
	},
	{
		ContextType: VoiceStateContext,
		UserID:      willowID,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/41 neutral.dca`,
			},
		},
	},
	{
		ContextType: VoiceStateContext,
		UserID:      shyronnieID,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/shyronnie1.dca`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot reconnect voice`,
		UserID:      willowID,
		Action: ActionEnvelope{
			Type: reconnect,
			Action: &ReconnectVoiceAction{
				Content: `Sure thing dad 🙂`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot restart`,
		UserID:      willowID,
		Action: ActionEnvelope{
			Type: reconnect,
			Action: &RestartAction{
				Content: `Okay dad 👀`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot go to sleep`,
		UserID:      willowID,
		Action: ActionEnvelope{
			Type: quit,
			Action: &QuitAction{
				Content: `Are you sure dad? 😳 💤`,
			},
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot kill yourself`,
		UserID:      willowID,
		Action: ActionEnvelope{
			Type: quit,
			Action: &QuitAction{
				Content: `💀`,
				Force:   true,
			},
		},
	},
	{
		ContextType:   MessageContext,
		Phrase:        `aoebot stats`,
		TextChannelID: ttyChannelID,
		Action: ActionEnvelope{
			Type:   stats,
			Action: &StatsAction{},
		},
	},
	{
		ContextType:    adHocContext,
		VoiceChannelID: openmicChannelID,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/vomit_1.dca`,
			},
		},
	},
	{
		ContextType:    adHocContext,
		VoiceChannelID: openmicChannelID,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/vomit_cough.dca`,
			},
		},
	},
	{
		ContextType:    adHocContext,
		VoiceChannelID: openmicChannelID,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/vomit_long.dca`,
			},
		},
	},
	{
		ContextType:    adHocContext,
		VoiceChannelID: openmicChannelID,
		Action: ActionEnvelope{
			Type: say,
			Action: &SayAction{
				File: `media/audio/vomit_help.dca`,
			},
		},
	},
}

// Load the audio frames for every audio file used in voice actions into memory
func loadVoiceActionFiles() error {
	for _, c := range conditions {
		va, ok := c.Action.Action.(*SayAction)
		if ok {
			// TODO could go va.load() for async
			err := va.load()
			// TODO could allow fail individually
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Generate conditions for aoe voice chat actions based on files with names matching a specific pattern.
// Files names matching regex(^0*(\d+).*\.dca$) create a condition that plays the audio in the matching file
// When the bot sees a message containing a token equal to the group captured in (\d+)
// E.g. File name "01 yes.dca" --> voice action plays when the bot sees a message containing a token equal to "1"
func createAoeChatCommands() error {
	files, err := ioutil.ReadDir("./media/audio")
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`^0*(\d+)(.*)\.dca$`)

	for _, file := range files {
		fname := file.Name()
		if re.MatchString(fname) {
			match := re.FindStringSubmatch(fname)
			phrase := match[1]
			name := match[2]
			c := Condition{
				Name:        name,
				ContextType: MessageContext,
				Phrase:      fmt.Sprintf(`\b%v\b`, phrase),
				IsRegex:     true,
				Action: ActionEnvelope{
					Type: say,
					Action: &SayAction{
						File: "media/audio/" + fname,
					},
				},
			}
			conditions = append(conditions, c)
		}
	}
	return nil
}
