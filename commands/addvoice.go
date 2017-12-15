package commands

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jeffreymkabot/aoebot"
	"github.com/jonas747/dca"
	"gopkg.in/mgo.v2/bson"
)

type guildPrefs struct {
	GuildID       string `bson:"guild"`
	SpamChannelID string `bson:"spam_channel"`
}

func getGuildPrefs(b *aoebot.Bot, guildID string) (*guildPrefs, error) {
	coll := b.Driver.DB("aoebot").C("guilds")
	query := bson.M{
		"guild": guildID,
	}
	prefs := &guildPrefs{}
	err := coll.Find(query).One(&prefs)
	return prefs, err
}

var addvoiceCmdRegexp = regexp.MustCompile(`^on "(\S.*)"$`)

func parseAddVoiceCmd(arg string, usage string) (phrase string, err error) {
	submatches := addvoiceCmdRegexp.FindStringSubmatch(arg)
	if submatches == nil {
		err = errors.New(usage)
		return
	}

	if submatches[1] == "" {
		err = errors.New("Couldn't parse phrase")
		return
	}
	phrase = strings.ToLower(submatches[1])
	return
}

type AddVoice struct {
	aoebot.BaseCommand
}

func (a *AddVoice) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *AddVoice) Usage() string {
	return `addvoice on "[phrase]"`
}

func (a *AddVoice) Short() string {
	return `Associate a sound clip with a phrase`
}

func (a *AddVoice) Long() string {
	return `Create an automatic audio response when a message matches [phrase].
You need to attach an audio file to the same message that invokes this command.
I will only take the first couple of seconds from the audio file.
Phrase is not case-sensitive and needs to match the entire message content to trigger the response.
Responses can be removed with the delvoice command.`
}

func (a *AddVoice) Examples() []string {
	return []string{
		`addvoice on "skrrt"`,
		`addvoice on "gotta go fast"`,
	}
}

func (a *AddVoice) Run(env *aoebot.Environment, args []string) error {
	f := flag.NewFlagSet(a.Name(), flag.ContinueOnError)
	filters := f.String("af", dca.StdEncodeOptions.AudioFilter, "ffmpeg filters")
	err := f.Parse(args)
	if err != nil {
		return err
	}

	if env.Guild == nil {
		return errors.New("No guild")
	}
	if len(env.Bot.Driver.ConditionsGuild(env.Guild.ID)) >= env.Bot.Config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}
	if len(env.TextMessage.Attachments) == 0 {
		return errors.New("No attached file")
	}

	argString := strings.Join(f.Args(), " ")
	phrase, err := parseAddVoiceCmd(argString, a.Usage())
	if err != nil {
		return err
	}

	url := env.TextMessage.Attachments[0].URL
	filename := env.TextMessage.Attachments[0].Filename
	duration := time.Duration(env.Bot.Config.MaxManagedVoiceDuration) * time.Second
	file, err := dcaFromURL(url, filename, duration, withFilters(*filters))
	if err != nil {
		return err
	}

	// if the guild has a spam text channel set up, restrict the voice condition to act only on
	// phrases written to the spam channel
	textChannelID := ""
	if prefs, err := getGuildPrefs(env.Bot, env.Guild.ID); err == nil {
		log.Printf("Using saved guild prefs %#v", prefs)
		textChannelID = prefs.SpamChannelID
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		TextChannelID:   textChannelID,
		Action: aoebot.NewActionEnvelope(&aoebot.VoiceAction{
			File:  file.Name(),
			Alias: filename,
		}),
	}

	return env.Bot.Driver.ConditionAdd(cond, env.Author.String())
}

func (a *AddVoice) Ack(env *aoebot.Environment) string {
	return "✅"
}

type encodeOption func(*dca.EncodeOptions)

func withFilters(filters string) encodeOption {
	return func(enc *dca.EncodeOptions) {
		if strings.TrimSpace(filters) != "" {
			enc.AudioFilter = filters
		}
	}
}

const limiterFilter = "loudnorm=i=-29"

const voiceFilePathTmpl = "media/audio/%s.dca"

func dcaFromURL(url string, fname string, maxDuration time.Duration, options ...encodeOption) (*os.File, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var encodeOptions = &dca.EncodeOptions{
		Volume:           256,
		Channels:         2,
		FrameRate:        48000,
		FrameDuration:    20,
		Bitrate:          64,
		Application:      dca.AudioApplicationAudio,
		CompressionLevel: 10,
		PacketLoss:       1,
		BufferedFrames:   100,
		VBR:              true,
	}
	for _, opt := range options {
		opt(encodeOptions)
	}
	// apply a limiter at the end of the signal chain
	if encodeOptions.AudioFilter != "" {
		encodeOptions.AudioFilter += ", "
	}
	encodeOptions.AudioFilter += limiterFilter

	encoder, err := dca.EncodeMem(resp.Body, encodeOptions)
	if err != nil {
		return nil, err
	}
	defer encoder.Cleanup()

	f, err := os.Create(fmt.Sprintf(voiceFilePathTmpl, fname))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	frameDuration := encoder.FrameDuration()

	// count frames to make sure we do not exceed the maximum allowed file size
	var frame []byte
	for fileDuration := time.Duration(0); fileDuration < maxDuration; fileDuration += frameDuration {
		frame, err = encoder.ReadFrame()
		if err != nil {
			if err == io.EOF {
				return f, nil
			}
			return nil, err
		}

		_, err = f.Write(frame)
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

var delvoiceCmdRegexp = regexp.MustCompile(`^"(\S.*)" on "(\S.*)"$`)

func parseDelVoiceCmd(arg string, usage string) (filename string, phrase string, err error) {
	submatches := delvoiceCmdRegexp.FindStringSubmatch(arg)
	if submatches == nil {
		err = errors.New(usage)
		return
	}

	if submatches[1] == "" {
		err = errors.New("Couldn't parse filename")
		return
	}
	filename = submatches[1]

	if submatches[2] == "" {
		err = errors.New("Couldn't parse phrase")
		return
	}
	phrase = strings.ToLower(submatches[2])
	return
}

type DelVoice struct {
	aoebot.BaseCommand
}

func (d *DelVoice) Name() string {
	return strings.Fields(d.Usage())[0]
}

func (d *DelVoice) Usage() string {
	return `delvoice "[filename]" on "[phrase]"`
}

func (d *DelVoice) Short() string {
	return `Unassociate a sound file with a phrase`
}

func (d *DelVoice) Long() string {
	return `Remove an automatic audio response created by addvoice.
Suppose there is a response created using the file "greenhillzone.wav" on the phrase "gotta go fast".
This response can be deleted with:
delvoice "greenhillzone.wav" on "gotta go fast"`
}

func (d *DelVoice) Examples() []string {
	return []string{
		`delvoice "greenhillzone.wav" on "gotta go fast"`,
	}
}

func (d *DelVoice) Run(env *aoebot.Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild")
	}

	argString := strings.Join(args, " ")
	filename, phrase, err := parseDelVoiceCmd(argString, d.Usage())
	if err != nil {
		return err
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: aoebot.NewActionEnvelope(&aoebot.VoiceAction{
			// TODO why does it need both ??
			Alias: filename,
			File:  fmt.Sprintf(voiceFilePathTmpl, filename),
		}),
	}

	return env.Bot.Driver.ConditionDisable(cond)
}

func (a *DelVoice) Ack(env *aoebot.Environment) string {
	return "🗑"
}
