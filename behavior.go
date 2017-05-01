package main

import (
	"io/ioutil"
	"strings"
	"regexp"
)

// static / dynamic conditions in separate file
var conditions []condition = []condition{
	{
		trigger: func(ctx context) bool {
			return strings.ToLower(ctx.message) == "?mayo"
		},
		response: &textAction{
			content: "Is mayonnaise an instrument?",
			tts:     true,
		},
	},
	{
		trigger: func(ctx context) bool {
			return strings.ToLower(ctx.message) == "aoebot"
		},
		response: &textAction{
			content: ":robot:",
			tts:     false,
		},
	},
}

func createAoeChatCommands() error {
	files, err := ioutil.ReadDir("./media/audio")
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`^0*(\d+).*\.dca$`)

	for _, file := range files {
		fname := file.Name()
		if (re.MatchString(fname)) {
			c := condition{
				trigger: func(ctx context) bool {
					phrase := re.FindStringSubmatch(fname)[1]
					// TODO strings.contains phrase as a token
					return strings.ToLower(ctx.message) == phrase
				},
				response: &voiceAction{
					file: "media/audio/" + fname,
				},
			}
			conditions = append(conditions, c)
		}
	}
	
	for _, c := range conditions {
		va, ok := c.response.(*voiceAction)
		if (ok) {
			// TODO could go va.load() for async
			err := va.load()
			if (err != nil) {
				return err
			}
		}
	}

	return nil
}