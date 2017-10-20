package commands

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jeffreymkabot/aoebot"
	"github.com/jonas747/dca"
)

const voiceFilePath = "media/audio/%s.dca"

var addvoiceCmdRegexp = regexp.MustCompile(`^on "(\S.*)"$`)

type AddVoice struct{}

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

func (a *AddVoice) IsOwnerOnly() bool {
	return false
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
	if !addvoiceCmdRegexp.MatchString(argString) {
		return errors.New(a.Usage())
	}
	submatches := addvoiceCmdRegexp.FindStringSubmatch(argString)

	phrase := strings.ToLower(submatches[1])
	if len(phrase) == 0 {
		return errors.New("Couldn't parse phrase")
	}

	url := env.TextMessage.Attachments[0].URL
	filename := env.TextMessage.Attachments[0].Filename
	duration := time.Duration(env.Bot.Config.MaxManagedVoiceDuration) * time.Second
	file, err := dcaFromURL(url, filename, duration, withFilters(*filters))
	if err != nil {
		return err
	}
	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
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
	encodeOptions.AudioFilter += "loudnorm=i=-28"

	encoder, err := dca.EncodeMem(resp.Body, encodeOptions)
	if err != nil {
		return nil, err
	}
	defer encoder.Cleanup()

	f, err := os.Create(fmt.Sprintf(voiceFilePath, fname))
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

type DelVoice struct{}

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

func (d *DelVoice) IsOwnerOnly() bool {
	return false
}

func (d *DelVoice) Run(env *aoebot.Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild")
	}

	argString := strings.Join(args, " ")
	if !delvoiceCmdRegexp.MatchString(argString) {
		return errors.New(d.Usage())
	}
	submatches := delvoiceCmdRegexp.FindStringSubmatch(argString)

	filename := submatches[1]
	if len(filename) == 0 {
		return errors.New("Coudln't parse filename")
	}

	phrase := strings.ToLower(submatches[2])
	if len(phrase) == 0 {
		return errors.New("Couldn't parse phrase")
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: aoebot.NewActionEnvelope(&aoebot.VoiceAction{
			// TODO why does it need both ??
			Alias: filename,
			File:  fmt.Sprintf(voiceFilePath, filename),
		}),
	}

	return env.Bot.Driver.ConditionDelete(cond)
}

func (a *DelVoice) Ack(env *aoebot.Environment) string {
	return "🗑"
}
