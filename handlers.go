package aoebot

import (
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) onReady() func(s *discordgo.Session, r *discordgo.Ready) {
	// Function signature needs to be exact to be detected as the Ready handler by discordgo
	// Access b Bot through a closure
	return func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Got discord ready: %#v\n", r)
		for _, g := range r.Guilds {
			if !g.Unavailable {
				b.registerGuild(g)
			}
		}
		b.addHandler(b.onGuildCreate())
		b.addHandler(b.onMessageCreate())
		b.addHandler(b.onVoiceStateUpdate())
		b.Session.UpdateStatus(0, b.Config.Prefix+" "+(&Help{}).Name())
	}
}

func (b *Bot) onGuildCreate() func(*discordgo.Session, *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		// log.Printf("Got guild create %#v", g.Guild)
		b.registerGuild(g.Guild)
	}
}

func (b *Bot) registerGuild(g *discordgo.Guild) {
	log.Printf("Register guild %v", g.Name)
	b.speakTo(g)
	for _, vs := range g.VoiceStates {
		// TODO bots could be in a channel in multiple guilds
		b.occupancy[vs.UserID] = vs.ChannelID
	}
	// restore management of any voice channels recovered from db
	channels := b.Driver.ChannelsGuild(g.ID)
	if len(channels) > 0 {
		log.Printf("Restore management of channels %v", channels)
		delete := func(ch channel) {
			log.Printf("Deleting channel %v", ch.Channel.Name)
			b.Session.ChannelDelete(ch.Channel.ID)
			b.Driver.ChannelDelete(ch.Channel.ID)
		}
		isEmpty := func(ch channel) bool {
			for _, v := range g.VoiceStates {
				if v.ChannelID == ch.Channel.ID {
					return false
				}
			}
			return true
		}
		interval := time.Duration(b.Config.ManagedChannelPollInterval) * time.Second
		for _, ch := range channels {
			b.AddRoutine(channelManager(ch, delete, isEmpty, interval))
		}
	}
}

func (b *Bot) onMessageCreate() func(*discordgo.Session, *discordgo.MessageCreate) {
	// Create a context around a voice state when the bot sees a new text message
	// Perform any actions that match that contex
	// Function signature needs to be exact to be detected as the right event handler by discordgo
	// Access b Bot through a closure
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Message == nil {
			return
		}

		env, err := NewEnvironment(b, m.Message)
		if err != nil {
			log.Printf("Error resolving environment of new message: %v", err)
			return
		}
		log.Printf("Saw a new message (%v) by %s in channel %v", env.TextMessage.Content, env.Author, env.TextChannel.Name)
		if env.Author.Bot || b.IsOwnEnvironment(env) {
			return
		}

		if strings.HasPrefix(env.TextMessage.Content, b.Config.Prefix) {
			args := strings.Fields(strings.TrimSpace(strings.TrimPrefix(env.TextMessage.Content, b.Config.Prefix)))
			cmd, args := b.command(args)
			log.Printf("Exec cmd %v by %s with %v", cmd.Name(), env.Author, args)
			b.exec(env, cmd, args)
		} else {
			actions := b.Driver.actions(env)
			log.Printf("Dispatch actions %v", actions)
			b.dispatch(env, actions...)
		}
	}
}

func (b *Bot) onVoiceStateUpdate() func(*discordgo.Session, *discordgo.VoiceStateUpdate) {
	// Create a context around a voice state when the bot sees someone's voice channel change
	// Perform any actions that match that contex
	// Function signature needs to be exact to be detected as the right event handler by discordgo
	// Access b Bot through a closure
	return func(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
		userID := v.VoiceState.UserID
		channelID := v.VoiceState.ChannelID

		occupancy := b.occupancy[userID]
		if occupancy != channelID {
			b.occupancy[userID] = channelID
			if channelID == "" {
				return
			}

			env, err := NewEnvironment(b, v.VoiceState)
			if err != nil {
				log.Printf("Error resolving voice state context: %v", err)
				return
			}
			log.Printf("Saw user %s join the voice channel %v in guild %v", env.Author, env.VoiceChannel.Name, env.Guild.Name)
			if b.IsOwnEnvironment(env) {
				return
			}

			// %

			actions := b.Driver.actions(env)
			log.Printf("Found actions %v", actions)
			b.dispatch(env, actions...)
		}
	}
}
