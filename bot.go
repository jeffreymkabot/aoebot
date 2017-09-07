package aoebot

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// ErrQuit is returned by Bot.Killer when the bot interprets a discord event as a signal to quit
var ErrQuit = errors.New("Dispatched a quit action")

// ErrForceQuit is returned by Bot.Killer when the bot interprets a discord event as a signal to quit immediately
var ErrForceQuit = errors.New("Dispatched a force quit action")

type Config struct {
	Prefix                     string
	HelpThumbnail              string `toml:"help_thumbnail"`
	MaxManagedConditions       int    `toml:"max_managed_conditions"`
	MaxManagedVoiceDuration    int    `toml:"max_managed_voice_duration"`
	MaxManagedChannels         int    `toml:"max_managed_channels"`
	ManagedChannelPollInterval int    `toml:"managed_channel_poll_interval"`
	Voice                      VoiceConfig
}

var DefaultConfig = Config{
	Prefix:                     "@!",
	MaxManagedConditions:       20,
	MaxManagedVoiceDuration:    5,
	MaxManagedChannels:         5,
	ManagedChannelPollInterval: 60,
	Voice: VoiceConfig{
		QueueLength: 100,
		SendTimeout: 1000,
		AfkTimeout:  300,
	},
}

// Bot represents a discord bot
type Bot struct {
	mu         sync.Mutex // TODO synchronize state and use of maps
	kill       chan struct{}
	killer     error
	Config     Config
	log        *log.Logger
	mongo      string
	owner      string
	commands   []Command
	Driver     *Driver
	Session    *discordgo.Session
	self       *discordgo.User
	routines   map[*botroutine]struct{} // Set
	unhandlers map[*func()]struct{}     // Set
	voiceboxes map[string]*voicebox     // TODO voiceboxes is vulnerable to concurrent read/write
	occupancy  map[string]string        // TODO occupancy is vulnerable to concurrent read/write
	aesthetic  bool
}

// New initializes a bot
func New(token string, mongo string, owner string, log *log.Logger) (b *Bot, err error) {
	b = &Bot{
		kill:       make(chan struct{}),
		Config:     DefaultConfig,
		log:        log,
		mongo:      mongo,
		owner:      owner,
		routines:   make(map[*botroutine]struct{}),
		unhandlers: make(map[*func()]struct{}),
		voiceboxes: make(map[string]*voicebox),
		occupancy:  make(map[string]string),
	}
	b.Session, err = discordgo.New("Bot " + token)
	if err != nil {
		return
	}
	b.commands = []Command{
		&Help{},
		&Reconnect{},
		&Restart{},
		&Shutdown{},
	}
	return
}

// WithConfig writes a new config struct
func (b *Bot) WithConfig(cfg Config) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Config = cfg
}

func (b *Bot) AddCommand(c Command) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.commands = append(b.commands, c)
}

// modeled after default package context
func (b *Bot) Die(err error) {
	if err == nil {
		panic("calls to Bot.die require a non-nil error")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.killer != nil {
		return
	}
	b.killer = err
	close(b.kill)
}

// Killed returns a channel that is closed when the bot recieves an internal signal to terminate.
// Clients using the bot *should* respect the signal and stop trying to use it
// modeled after default package context
func (b *Bot) Killed() <-chan struct{} {
	return b.kill
}

// Killer returns an error message that is non-nil once the bot receives an internal signal to terminate
// modeled after default package context
func (b *Bot) Killer() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.killer
}

// Start initiates a database session and a discord session
func (b *Bot) Start() (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Driver, err = newDriver(b.mongo)
	if err != nil {
		return
	}

	b.self, err = b.Session.User("@me")
	if err != nil {
		return
	}

	b.addHandlerOnce(b.onReady())

	// begin listen to discord websocket for events
	// invoking session.Open() triggers the discord ready event
	err = b.Session.Open()
	if err != nil {
		return
	}
	return
}

// Stop removes event handlers, stops all workers, closes the discord session, and closes the db session.
func (b *Bot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	log.Printf("Closing session...")

	log.Printf("Disabling event handlers...")
	for f := range b.unhandlers {
		if f != nil {
			(*f)()
		}
		delete(b.unhandlers, f)
	}

	log.Printf("Closing botroutines...")
	for r := range b.routines {
		r.close()
		delete(b.routines, r)
	}

	log.Printf("Closing voiceboxes...")
	for k, vb := range b.voiceboxes {
		vb.close()
		delete(b.voiceboxes, k)
	}

	// close the session after closing voice boxes since closing voiceboxes attempts graceful voiceconnection disconnect using discord session
	log.Printf("Closing discord...")
	b.Session.Close()

	log.Printf("Closing mongo...")
	b.Driver.Close()

	log.Printf("...closed session.")
}

// Write a message to a channel in a guild
func (b *Bot) Write(channelID string, message string, tts bool) (err error) {
	if tts {
		_, err = b.Session.ChannelMessageSendTTS(channelID, message)
	} else {
		_, err = b.Session.ChannelMessageSend(channelID, message)
	}
	return
}

// React with an emoji to a message in a channel in a guild
func (b *Bot) React(channelID string, messageID string, emoji string) (err error) {
	err = b.Session.MessageReactionAdd(channelID, messageID, emoji)
	return
}

// Say some audio frames to a channel in a guild
// Say drops the payload when the voicebox for that guild queue is full
func (b *Bot) Say(guildID string, channelID string, audio [][]byte) (err error) {
	if vb, ok := b.voiceboxes[guildID]; ok && vb != nil && vb.queue != nil {
		vp := &voicePayload{
			buffer:    audio,
			channelID: channelID,
		}
		select {
		case vb.queue <- vp:
		default:
			err = fmt.Errorf("Full voice queue in guild %v", guildID)
		}
	} else {
		err = fmt.Errorf("No voicebox registered for guild %v", guildID)
	}
	return
}

// helper func
func (b *Bot) sayToUserInGuild(guild *discordgo.Guild, userID string, audio [][]byte) (err error) {
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return b.Say(guild.ID, vs.ChannelID, audio)
		}
	}
	err = fmt.Errorf("Couldn't find user %v in a voice channel in guild %v", userID, guild.ID)
	return
}

// Listen to some audio frames in a guild
// TODO
func (b *Bot) Listen(guildID string, channelID string, duration time.Duration) (err error) {
	return
}

// Add an event handler to the discord session and retain a reference to the handler remover
func (b *Bot) addHandler(handler interface{}) {
	unhandler := b.Session.AddHandler(handler)
	b.unhandlers[&unhandler] = struct{}{}
}

// Add a one-time event handler to the discord session and retain a reference to the handler remover
func (b *Bot) addHandlerOnce(handler interface{}) {
	unhandler := b.Session.AddHandlerOnce(handler)
	b.unhandlers[&unhandler] = struct{}{}
}

type botroutine struct {
	close func()
}

func newRoutine(f func(<-chan struct{})) *botroutine {
	quit := make(chan struct{})
	close := func() {
		select {
		case <-quit:
			return
		default:
			close(quit)
		}
	}
	go f(quit)
	return &botroutine{
		close: close,
	}
}

func (b *Bot) AddRoutine(f func(<-chan struct{})) {
	r := newRoutine(f)
	b.routines[r] = struct{}{}
}

// IsOwnEnvironment is true when an environment's seed is the result of the bot's own actions/behavior
// This is useful to prevent the bot from reacting to itself
func (b *Bot) IsOwnEnvironment(env *Environment) bool {
	return env.Author != nil && env.Author.ID == b.self.ID
}

// channel's fields must be exported to be visible to bson.Marshal
// currently only needs ID and GuildID from *discordgo.Channel, but may be convenient to just take everything
// channel itself does not need to be exported
type channel struct {
	IsOpen  bool
	Users   int
	Channel *discordgo.Channel
}

// ChannelOption is a functional option used as a variadic parameter to AddManagedVoiceChannel
type ChannelOption func(*channel)

// ChannelOpenMic sets whether the discord channel will override the UserVoiceActivity permission
func ChannelOpenMic(b bool) ChannelOption {
	return func(ch *channel) {
		ch.IsOpen = b
	}
}

// ChannelUsers sets whether the discord channel will have a user limit
// values for n that are less than 1 or greater than 99 will have no effect
func ChannelUsers(n int) ChannelOption {
	return func(ch *channel) {
		if 0 < n && n < 100 {
			ch.Users = n
		}
	}
}

// AddManagedVoiceChannel creates a new voice channel in a guild
// The voice channel will be polled periodically and deleted if it is found to be empty
func (b *Bot) AddManagedVoiceChannel(guildID string, name string, options ...ChannelOption) (err error) {
	var ch channel
	for _, opt := range options {
		opt(&ch)
	}

	ch.Channel, err = b.Session.GuildChannelCreate(guildID, name, "voice")
	if err != nil {
		return
	}
	log.Printf("created new discord channel %#v", ch.Channel)

	delete := func(ch channel) {
		log.Printf("Deleting channel %v", ch.Channel.Name)
		b.Session.ChannelDelete(ch.Channel.ID)
		b.Driver.ChannelDelete(ch.Channel.ID)
	}
	err = b.Driver.ChannelAdd(ch)
	if err != nil {
		delete(ch)
		return
	}

	isEmpty := func(ch channel) bool {
		g, err := b.Session.State.Guild(ch.Channel.GuildID)
		if err == nil {
			for _, v := range g.VoiceStates {
				if v.ChannelID == ch.Channel.ID {
					return false
				}
			}
		}
		return true
	}
	interval := time.Duration(b.Config.ManagedChannelPollInterval) * time.Second

	b.AddRoutine(channelManager(ch, delete, isEmpty, interval))

	if ch.IsOpen {
		err = b.Session.ChannelPermissionSet(ch.Channel.ID, ch.Channel.GuildID, "role", discordgo.PermissionVoiceUseVAD, 0)
		if err != nil {
			delete(ch)
			return
		}
	}
	if ch.Users > 0 {
		data := struct {
			UserLimit int `json:"user_limit"`
		}{ch.Users}
		_, err = b.Session.RequestWithBucketID("PATCH", discordgo.EndpointChannel(ch.Channel.ID), data, discordgo.EndpointChannel(ch.Channel.ID))
		if err != nil {
			delete(ch)
			return
		}
	}
	return
}

// channelManager returns a botroutine that periodically polls a channel and deletes it if its empty
// isEmpty must return true if the channel is already deleted
// delete should not panic if the channel is already deleted
func channelManager(ch channel, delete func(ch channel), isEmpty func(ch channel) bool, pollInterval time.Duration) func(quit <-chan struct{}) {
	return func(quit <-chan struct{}) {
		for {
			select {
			case <-quit:
				return
			case <-time.After(pollInterval):
				if isEmpty(ch) {
					delete(ch)
					return
				}
			}
		}
	}
}

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
		b.Session.UpdateStatus(0, fmt.Sprintf("%s %s", b.Config.Prefix, (&Help{}).Name()))
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
		log.Printf("Saw a new message (%v) by %s in channel %v in guild %v", env.TextMessage.Content, env.Author, env.TextChannel.Name, env.Guild.Name)
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

func (b *Bot) command(args []string) (Command, []string) {
	if len(args) > 0 {
		cmd := strings.ToLower(args[0])
		args := args[1:]
		for _, c := range b.commands {
			if c.Name() == cmd {
				return c, args
			}
		}
	}
	return &Help{}, []string{}
}

func (b *Bot) exec(env *Environment, cmd Command, args []string) {
	if cmd.IsOwnerOnly() && env.Author.ID != b.owner {
		_ = b.Write(env.TextChannel.ID, "Sorry, only dad can use that one 🙃", false)
		return
	}
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Recovered from panic in exec %v with %v: %v", cmd.Name(), args, err)
		}
	}()

	err := cmd.Run(env, args)
	if err != nil {
		log.Printf("Error in exec %v with %v: %v", cmd.Name(), args, err)
		_ = b.Write(env.TextChannel.ID, fmt.Sprintf("🤔...\n%v", err), false)
		return
	}
}

func (b *Bot) dispatch(env *Environment, actions ...Action) {
	for _, a := range actions {
		// shadow a in the goroutine
		// a iterates through for loop goroutine would otherwise try to use it in closure asynchronously
		go func(a Action) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("Recovered from panic in perform %T on %v: %v", a, env.Type, err)
				}
			}()
			log.Printf("Perform %T on %v: %v", a, env.Type, a)
			err := a.Perform(env)
			if err != nil {
				log.Printf("Error in perform %T on %v: %v", a, env.Type, err)
			}
		}(a)
	}
}
