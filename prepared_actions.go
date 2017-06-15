package main

import (
	// "fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

var conditions []condition = []condition{
	{
		trigger: func(ctx *context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.textMessage.Content), " "), "?mayo")
		},
		response: &textAction{
			content: "Is mayonnaise an instrument?",
			tts:     true,
		},
	},
	{
		trigger: func(ctx *context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.textMessage.Content), " "), "aoebot?")
		},
		response: &textAction{
			content: ":robot:",
			tts:     false,
		},
	},
	{
		trigger: func(ctx *context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.textMessage.Content), " "), "aoebot")
		},
		response: &emojiReactionAction{
			emoji: "🤖", // unicode for :robot:
		},
	},
	{
		trigger: func(ctx *context) bool {
			return strings.Contains(strings.ToLower(ctx.textMessage.Content), "heroes of the storm")
		},
		response: &textAction{
			content: ":nauseated_face:",
			tts:     false,
		},
	},
	{
		trigger: func(ctx *context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.textMessage.Content), " "), "hots", "hots?")
		},
		response: &emojiReactionAction{
			emoji: "🤢", // unicode for :nauseated_face:
		},
	},
	{
		trigger: func(ctx *context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.textMessage.Content), " "), "smash")
		},
		response: &textAction{
			content: "Smash that ready button!",
			tts:     true,
		},
	},
	{
		trigger: func(ctx *context) bool {
			return false // ctx.channel.ID != "" && ctx.author.ID == willowID
		},
		response: &voiceAction{
			file: "media/audio/40 enemy.dca",
		},
		name: "willow",
	},
	{
		trigger: func(ctx *context) bool {
			return ctx.Type == VoiceStateContext && ctx.author.ID == shyronnieID && ctx.voiceChannel != nil
		},
		response: &voiceAction{
			file: "media/audio/shyronnie1.dca",
		},
		name: "shyronnie",
	},
	{
		trigger: func(ctx *context) bool {
			return containsKeyword(strings.Split(strings.ToLower(ctx.textMessage.Content), " "), "bruh")
		},
		response: &voiceAction{
			file: "media/audio/H3H3_BRUH.dca",
		},
		name: "bruh",
	},
	{
		trigger: func(ctx *context) bool {
			return ctx.author.ID == willowID && strings.ToLower(ctx.textMessage.Content) == "aoebot reconnect voice"
		},
		response: &reconnectVoiceAction{
			content: "Sure thing dad :slight_smile:",
		},
	},
	{
		trigger: func(ctx *context) bool {
			return ctx.author.ID == willowID && strings.ToLower(ctx.textMessage.Content) == "aoebot restart"
		},
		response: &restartAction{
			content: "Okay dad :eyes:",
		},
	},
	{
		trigger: func(ctx *context) bool {
			return ctx.author.ID == willowID && strings.ToLower(ctx.textMessage.Content) == "aoebot go to sleep"
		},
		response: &quitAction{
			content: "Are you sure dad? :flushed: :zzz:",
		},
	},
}

func loadVoiceActionFiles() error {
	for _, c := range conditions {
		va, ok := c.response.(*voiceAction)
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
				trigger: func(ctx *context) bool {
					return containsKeyword(strings.Split(strings.ToLower(ctx.textMessage.Content), " "), phrase)
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
