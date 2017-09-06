package commands

import (
	"errors"
	"flag"
	"fmt"
	"github.com/jeffreymkabot/aoebot"
	"strings"
)

type AddChannel struct{}

func (ac *AddChannel) Name() string {
	return strings.Fields(ac.Usage())[0]
}

func (ac *AddChannel) Usage() string {
	return `addchannel [-openmic] [-users n]`
}

func (ac *AddChannel) Short() string {
	return `Create a temporary voice channel`
}

func (ac *AddChannel) Long() string {
	return `Create an ad hoc voice channel in this guild.
Use the "openmic" flag to override the channel's "Use Voice Activity" permission.
Use the "users" flag to limit the number of users that can join the channel.
I will automatically delete voice channels when I see they are vacant.
I will only create so many voice channels for each guild.`
}

func (ac *AddChannel) IsOwnerOnly() bool {
	return false
}

func (ac *AddChannel) Run(env *aoebot.Environment, args []string) error {
	f := flag.NewFlagSet("addchannel", flag.ContinueOnError)
	isOpen := f.Bool("openmic", false, "permit voice activity")
	userLimit := f.Int("users", 0, "limit users to `n`")
	err := f.Parse(args)
	if err != nil {
		return err
	}

	if env.Guild == nil {
		return errors.New("No guild")
	}
	if len(env.Bot.Driver.ChannelsGuild(env.Guild.ID)) >= env.Bot.Config.MaxManagedChannels {
		return errors.New("I'm not allowed to make any more channels in this guild 😦")
	}

	chName := fmt.Sprintf("@!%s", env.Author)
	if *isOpen {
		chName = "open" + chName
	}

	return env.Bot.AddManagedVoiceChannel(env.Guild.ID, chName, aoebot.ChannelOpenMic(*isOpen), aoebot.ChannelUsers(*userLimit))
}