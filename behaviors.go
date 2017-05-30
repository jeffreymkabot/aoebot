package main

import (
	// "fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

var conditions []condition = []condition{
	{
		trigger: func(ctx context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.message), " "), "?mayo")
		},
		response: &textAction{
			content: "Is mayonnaise an instrument?",
			tts:     true,
		},
	},
	{
		trigger: func(ctx context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.message), " "), "aoebot", "aoebot?")
		},
		response: &textAction{
			content: ":robot:",
			tts:     false,
		},
	},
	{
		trigger: func(ctx context) bool {
			return strings.Contains(strings.ToLower(ctx.message), "heroes of the storm")
		},
		response: &textAction{
			content: ":nauseated_face:",
			tts:     false,
		},
	},
	{
		trigger: func(ctx context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.message), " "), "hots", "hots?")
		},
		response: &emojiReactionAction{
			emoji: "🤢", // unicode for :nauseated_face:
		},
	},
	{
		trigger: func(ctx context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.message), " "), "smash")
		},
		response: &textAction{
			content: "Smash that ready button!",
			tts:     true,
		},
	},
	{
		trigger: func(ctx context) bool {
			return false
		},
		response: &voiceAction{
			file: "media/audio/shyronnie1.dca",
		},
		name: "shyronnie",
	},
}

func loadVoiceActionFiles() error {
	for _, c := range conditions {
		va, ok := c.response.(*voiceAction)
		if ok {
			// TODO could go va.load() for async
			err := va.load()
			// TODO allow fail individually
			if err != nil {
				return err
			}
		}
	}
	return nil
}

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
			c := condition{
				trigger: func(ctx context) bool {
					return containsKeyword(strings.Split(strings.ToLower(ctx.message), " "), phrase)
				},
				response: &voiceAction{
					file: "media/audio/" + fname,
				},
				name: phrase,
			}
			conditions = append(conditions, c)
		}
	}
	return nil
}
