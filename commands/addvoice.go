package commands

import (
	"errors"
	"flag"
	"fmt"
	"github.com/jeffreymkabot/aoebot"
	"github.com/jonas747/dca"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var addvoiceCmdRegexp = regexp.MustCompile(`^on (?:"(\S.*)"|(\S.*))$`)

type AddVoice struct{}

func (a *AddVoice) Name() string {
	return strings.Fields(a.Usage())[0]
}

func (a *AddVoice) Usage() string {
	return `addvoice on [phrase]`
}

func (a *AddVoice) Short() string {
	return `Associate a sound clip with a phrase`
}

func (a *AddVoice) Long() string {
	return `Upload an audio file that can be played in response to a phrase.
You need to attach the audio file to the message that invokes this command.
I will only take the first couple of seconds from the audio file.
Phrase is not case-sensitive and needs to match the entire message content to trigger the response.
This is the inverse of delvoice.`
}

func (a *AddVoice) IsOwnerOnly() bool {
	return false
}

func (a *AddVoice) Run(env *aoebot.Environment, args []string) error {
	f := flag.NewFlagSet(a.Name(), flag.ContinueOnError)
	vol := f.Int("vol", dca.StdEncodeOptions.Volume, "volume")
	filters := f.String("af", dca.StdEncodeOptions.AudioFilter, "ffmpeg filters")
	err := f.Parse(args)
	if err != nil {
		return err
	}

	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild?
	}
	if len(env.Bot.Driver.ConditionsGuild(env.Guild.ID)) >= env.Bot.Config.MaxManagedConditions {
		return errors.New("I'm not allowed make any more memes in this guild")
	}
	if len(env.TextMessage.Attachments) == 0 {
		return errors.New("No attached file")
	}

	argString := strings.Join(f.Args(), " ")
	if !addvoiceCmdRegexp.MatchString(argString) {
		return (&aoebot.Help{}).Run(env, []string{"addvoice"})
	}
	submatches := addvoiceCmdRegexp.FindStringSubmatch(argString)

	var phrase string
	if len(submatches[1]) > 0 {
		phrase = strings.ToLower(submatches[1])
	} else {
		phrase = strings.ToLower(submatches[2])
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	url := env.TextMessage.Attachments[0].URL
	filename := env.TextMessage.Attachments[0].Filename
	duration := time.Duration(env.Bot.Config.MaxManagedVoiceDuration)*time.Second
	file, err := dcaFromURL(url, filename, duration, withVolume(*vol), withFilters(*filters))
	if err != nil {
		return err
	}
	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: aoebot.NewActionEnvelope(&aoebot.VoiceAction{
			File: file.Name(),
		}),
	}

	err = env.Bot.Driver.ConditionAdd(cond, env.Author.String())
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `+`, false)
	return nil
}

type encodeOption func(*dca.EncodeOptions)

func withVolume(vol int) encodeOption {
	return func(enc *dca.EncodeOptions) {
		enc.Volume = vol
	}
}

func withFilters(filters string) encodeOption {
	return func(enc *dca.EncodeOptions) {
		enc.AudioFilter = filters
	}
}

func dcaFromURL(url string, fname string, maxDuration time.Duration, options ...encodeOption) (f *os.File, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var encodeOptions = &dca.EncodeOptions{
		Volume:           256,
		Channels:         2,
		FrameRate:        48000,
		FrameDuration:    20,
		Bitrate:          64,
		RawOutput:        true,
		Application:      dca.AudioApplicationAudio,
		CompressionLevel: 10,
		PacketLoss:       1,
		BufferedFrames:   100,
		VBR:              true,
	}
	for _, opt := range options{
		opt(encodeOptions)
	}

	encoder, err := dca.EncodeMem(resp.Body, encodeOptions)
	if err != nil {
		return
	}
	defer encoder.Cleanup()

	f, err = os.Create(fmt.Sprintf("./media/audio/%s.dca", fname))
	if err != nil {
		return
	}
	defer f.Close()

	frameDuration := encoder.FrameDuration()
	fileDuration := time.Duration(0)

	// count frames to make sure we do not exceed the maximum allowed file size
	var frame []byte
	for ; fileDuration < maxDuration; fileDuration += frameDuration {
		frame, err = encoder.ReadFrame()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		_, err = f.Write(frame)
		if err != nil {
			return
		}
	}
	return
}

var delvoiceCmdRegexp = regexp.MustCompile(`^(?:"(\S.*)"|(\S.*)) on (?:"(\S.*)"|(\S.*))$`)

type DelVoice struct{}

func (d *DelVoice) Name() string {
	return strings.Fields(d.Usage())[0]
}

func (d *DelVoice) Usage() string {
	return `delvoice [filename] on [phrase]`
}

func (d *DelVoice) Short() string {
	return `Unassociate a sound file with a phrase`
}

func (d *DelVoice) Long() string {
	return `Remove an existing association between sound clip and a phrase.
This is the inverse of addvoice.
Files uploaded with addvoice are saved to a relative path and with a new file extension.
The relative path and new file extension can be discovered with the getmemes command.
For example, suppose an assocation is created by uploading the file "greenhillzone.wav" with the command "addvoice on gotta go fast".
The "getmemes" command will show: say ./media/audio/greenhillzone.wav.dca on "gotta go fast".
This assocation can be deleted with "delvoice ./media/audio/greenhillzone.wav.dca on gotta go fast".`
}

func (d *DelVoice) IsOwnerOnly() bool {
	return false
}

func (d *DelVoice) Run(env *aoebot.Environment, args []string) error {
	if env.Guild == nil {
		return errors.New("No guild") // ErrNoGuild
	}

	argString := strings.Join(args, " ")
	if !delvoiceCmdRegexp.MatchString(argString) {
		return (&aoebot.Help{}).Run(env, []string{"delvoice"})
	}
	submatches := delvoiceCmdRegexp.FindStringSubmatch(argString)

	var filename string
	if len(submatches[1]) > 0 {
		filename = strings.ToLower(submatches[1])
	} else {
		filename = strings.ToLower(submatches[2])
	}
	if len(filename) == 0 {
		return errors.New("Bad filename")
	}

	var phrase string
	if len(submatches[3]) > 0 {
		phrase = strings.ToLower(submatches[3])
	} else {
		phrase = strings.ToLower(submatches[4])
	}
	if len(phrase) == 0 {
		return errors.New("Bad phrase")
	}

	cond := &aoebot.Condition{
		EnvironmentType: aoebot.Message,
		GuildID:         env.Guild.ID,
		Phrase:          phrase,
		Action: aoebot.NewActionEnvelope(&aoebot.VoiceAction{
			File: filename,
		}),
	}

	err := env.Bot.Driver.ConditionDelete(cond)
	if err != nil {
		return err
	}
	_ = env.Bot.Write(env.TextChannel.ID, `🗑️`, false)
	return nil
}
