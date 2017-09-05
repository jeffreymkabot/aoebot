package commands

import (
	"errors"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jeffreymkabot/aoebot"
	"log"
	"strings"
	"time"
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

	ch, err := env.Bot.Session.GuildChannelCreate(env.Guild.ID, chName, `voice`)
	if err != nil {
		return err
	}
	log.Printf("Created channel %s", ch.Name)

	delete := func(ch aoebot.Channel) {
		log.Printf("Deleting channel %s", ch.Name)
		_, _ = env.Bot.Session.ChannelDelete(ch.ID)
		_ = env.Bot.Driver.ChannelDelete(ch.ID)
	}
	err = env.Bot.Driver.ChannelAdd(aoebot.Channel(ch))
	if err != nil {
		delete(ch)
		return err
	}

	isEmpty := func(ch aoebot.Channel) bool {
		for _, v := range env.Guild.VoiceStates {
			if v.ChannelID == ch.ID {
				return false
			}
		}
		return true
	}
	interval := time.Duration(env.Bot.Config.ManagedChannelPollInterval) * time.Second
	env.Bot.AddRoutine(aoebot.ChannelManager(ch, delete, isEmpty, interval))

	if *isOpen {
		err = env.Bot.Session.ChannelPermissionSet(ch.ID, env.Guild.ID, `role`, discordgo.PermissionVoiceUseVAD, 0)
		if err != nil {
			delete(ch)
			return err
		}
	}
	if userLimit != nil {
		data := struct {
			UserLimit int `json:"user_limit"`
		}{*userLimit}
		_, err = env.Bot.Session.RequestWithBucketID("PATCH", discordgo.EndpointChannel(ch.ID), data, discordgo.EndpointChannel(ch.ID))
		if err != nil {
			delete(ch)
			return err
		}
	}
	return nil
}
