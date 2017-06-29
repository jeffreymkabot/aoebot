package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
)

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
			content: `Sure thing dad :slight_smile:`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot restart`,
		UserID:      willowID,
		ActionType:  reconnect,
		Action: &restartAction{
			content: `Okay dad :eyes:`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot go to sleep`,
		UserID:      willowID,
		ActionType:  quit,
		Action: &quitAction{
			content: `Are you sure dad? :flushed: :zzz:`,
		},
	},
	{
		ContextType: MessageContext,
		Phrase:      `aoebot kill yourself`,
		UserID:      willowID,
		ActionType:  quit,
		Action: &quitAction{
			content: ":skull:",
			force:   true,
		},
	},
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
