package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
)

// Condition defines a set of requirements an environment should meet for an action to be performed on that environment
type Condition struct {
	Name string `json:"name"`
	// e.g. MessageContext, VoiceStateContext
	ContextType    int    `json:"ctype"`
	Phrase         string `json:"phrase,omitempty"`
	IsRegex        bool   `json:"isRegex,omitempty"`
	GuildID        string `json:"guild,omitempty"`
	TextChannelID  string `json:"textChannel,omitempty"`
	VoiceChannelID string `json:"voiceChannel,omitempty"`
	UserID         string `json:"user,omitempty"`
	// e.g. textAction, quitAction
	ActionType int    `json:"atype"`
	Action     Action `json:"action"`
}

var conditions = []Condition{
	{
		ContextType: MessageContext,
		Phrase:      `?testwrite`,
		ActionType:  write,
		Action: &textAction{
			content: `hello world`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `?testvoice`,
		ActionType:  say,
		Action: &voiceAction{
			file: `media/audio/40 enemy.dca`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `?testreact`,
		ActionType:  react,
		Action: &emojiReactionAction{
			emoji: `🤖`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\baoebot\b`,
		IsRegex:     true,
		ActionType:  react,
		Action: &emojiReactionAction{
			emoji: `🤖`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bheroes of the storm\b`,
		IsRegex:     true,
		ActionType:  write,
		Action: &textAction{
			content: `🤢`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bhots\b`,
		IsRegex:     true,
		ActionType:  react,
		Action: &emojiReactionAction{
			emoji: `🤢`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bsmash\b`,
		IsRegex:     true,
		ActionType:  write,
		Action: &textAction{
			content: `Smash that ready button!`,
			tts:     true,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bbruh\b`,
		IsRegex:     true,
		ActionType:  say,
		Action: &voiceAction{
			file: `media/audio/H3H3_BRUH.dca`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `\bnice shades\b`,
		IsRegex:     true,
		ActionType:  say,
		Action: &voiceAction{
			file: `media/audio/my_vision_is_augmented.dca`,
		},
	},
	{
		ContextType: VoiceStateContext,
		UserID:      willowID,
		ActionType:  say,
		Action: &voiceAction{
			file: `media/audio/41 neutral.dca`,
		},
	},
	{
		ContextType: VoiceStateContext,
		UserID:      shyronnieID,
		ActionType:  say,
		Action: &voiceAction{
			file: `media/audio/shyronnie1.dca`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot reconnect voice`,
		UserID:      willowID,
		ActionType:  reconnect,
		Action: &reconnectVoiceAction{
			content: `Sure thing dad 🙂`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot restart`,
		UserID:      willowID,
		ActionType:  reconnect,
		Action: &restartAction{
			content: `Okay dad 👀`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot go to sleep`,
		UserID:      willowID,
		ActionType:  quit,
		Action: &quitAction{
			content: `Are you sure dad? 😳 💤`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot kill yourself`,
		UserID:      willowID,
		ActionType:  quit,
		Action: &quitAction{
			content: `💀`,
			force:   true,
		},
	},
	{
		ContextType:   MessageContext,
		Phrase:        `aoebot stats`,
		TextChannelID: ttyChannelID,
		ActionType:    stats,
		Action:        &statsAction{},
	},
	{
		ContextType:    adHocContext,
		VoiceChannelID: openmicChannelID,
		ActionType:     say,
		Action: &voiceAction{
			file: `media/audio/vomit_1.dca`,
		},
	},
	{
		ContextType:    adHocContext,
		VoiceChannelID: openmicChannelID,
		ActionType:     say,
		Action: &voiceAction{
			file: `media/audio/vomit_cough.dca`,
		},
	},
	{
		ContextType:    adHocContext,
		VoiceChannelID: openmicChannelID,
		ActionType:     say,
		Action: &voiceAction{
			file: `media/audio/vomit_long.dca`,
		},
	},
	{
		ContextType:    adHocContext,
		VoiceChannelID: openmicChannelID,
		ActionType:     say,
		Action: &voiceAction{
			file: `media/audio/vomit_help.dca`,
		},
	},
}

func (c Condition) persist() {
	me.db.
}

// Load the audio frames for every audio file used in voice actions into memory
func loadVoiceActionFiles() error {
	for _, c := range conditions {
		va, ok := c.Action.(*voiceAction)
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

	re := regexp.MustCompile(`^0*(\d+).*\.dca$`)

	for _, file := range files {
		fname := file.Name()
		if re.MatchString(fname) {
			phrase := re.FindStringSubmatch(fname)[1]
			c := Condition{
				ContextType: MessageContext,
				Phrase:      fmt.Sprintf(`\b%v\b`, phrase),
				IsRegex:     true,
				ActionType:  say,
				Action: &voiceAction{
					file: "media/audio/" + fname,
				},
			}
			conditions = append(conditions, c)
		}
	}
	return nil
}
