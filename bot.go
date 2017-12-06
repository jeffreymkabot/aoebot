package aoebot

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	dgv "github.com/jeffreymkabot/discordvoice"
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
	Voice                      dgv.PlayerConfig
}

var DefaultConfig = Config{
	Prefix:                     "@!",
	MaxManagedConditions:       20,
	MaxManagedVoiceDuration:    5,
	MaxManagedChannels:         5,
	ManagedChannelPollInterval: 60,
	Voice: dgv.PlayerConfig{
		QueueLength: 100,
		SendTimeout: 1000,
		IdleTimeout: 300,
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
	routines   map[*func()]struct{}   // Set
	unhandlers map[*func()]struct{}   // Set
	voiceboxes map[string]*dgv.Player // TODO voiceboxes is vulnerable to concurrent read/write
	occupancy  map[string]string      // TODO occupancy is vulnerable to concurrent read/write
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
		routines:   make(map[*func()]struct{}),
		unhandlers: make(map[*func()]struct{}),
		voiceboxes: make(map[string]*dgv.Player),
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
	for f := range b.routines {
		if f != nil {
			(*f)()
		}
		delete(b.routines, f)
	}

	log.Printf("Closing voiceboxes...")
	for k, player := range b.voiceboxes {
		player.Quit()
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
// Returns a function to remove the reaction
func (b *Bot) React(channelID string, messageID string, emoji string) (unreact func() error, err error) {
	unreact = func() error {
		return nil
	}
	err = b.Session.MessageReactionAdd(channelID, messageID, emoji)
	if err == nil {
		unreact = func() error {
			return b.Session.MessageReactionRemove(channelID, messageID, emoji, "@me")
		}
	}
	return
}

// Say some audio frames to a channel in a guild
// Say drops the payload when the voicebox for that guild queue is full
func (b *Bot) Say(guildID string, channelID string, reader io.Reader) (err error) {
	if player, ok := b.voiceboxes[guildID]; ok && player != nil {
		err = player.Enqueue(channelID, "", dgv.PreEncoded(reader))
	} else {
		err = fmt.Errorf("No voicebox registered for guild %v", guildID)
	}
	return
}

// helper func
func (b *Bot) sayToUserInGuild(guild *discordgo.Guild, userID string, reader io.Reader) (err error) {
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return b.Say(guild.ID, vs.ChannelID, reader)
		}
	}
	err = fmt.Errorf("Couldn't find user %v in a voice channel in guild %v", userID, guild.ID)
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

// IsOwnEnvironment is true when an environment's seed is the result of the bot's own actions/behavior
// This is useful to prevent the bot from reacting to itself
func (b *Bot) IsOwnEnvironment(env *Environment) bool {
	return env.Author != nil && env.Author.ID == b.self.ID
}

type botroutine func(<-chan struct{})

func newRoutine(f botroutine) func() {
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
	return close
}

func (b *Bot) AddRoutine(f func(<-chan struct{})) func() {
	closer := newRoutine(f)
	b.routines[&closer] = struct{}{}
	return closer
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
func channelManager(ch channel, delete func(ch channel), isEmpty func(ch channel) bool, pollInterval time.Duration) botroutine {
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

// speakTo opens the conversation with a discord guild
func (b *Bot) speakTo(g *discordgo.Guild) {
	player, ok := b.voiceboxes[g.ID]
	if ok {
		player.Quit()
	}
	ql := dgv.QueueLength(b.Config.Voice.QueueLength)
	st := dgv.SendTimeout(b.Config.Voice.SendTimeout)
	at := dgv.IdleTimeout(b.Config.Voice.IdleTimeout)
	b.voiceboxes[g.ID] = dgv.Connect(b.Session, g.ID, g.AfkChannelID, ql, st, at)
}

func (b *Bot) command(args []string) (Command, []string) {
	if len(args) > 0 {
		candidate := strings.ToLower(args[0])
		args := args[1:]
		for _, cmd := range b.commands {
			if matchesNameOrAlias(cmd, candidate) {
				return cmd, args
			}
		}
	}
	return &Help{}, []string{}
}

func matchesNameOrAlias(cmd Command, candidate string) bool {
	if cmd.Name() == candidate {
		return true
	}
	if cmdAlias, ok := cmd.(CommandWithAliases); ok {
		for _, alias := range cmdAlias.Aliases() {
			if alias == candidate {
				return true
			}
		}
	}
	return false
}

func (b *Bot) exec(env *Environment, cmd Command, args []string) {
	if cmd.IsOwnerOnly() && env.Author.ID != b.owner {
		b.Write(env.TextChannel.ID, "I'm sorry, Dave.  I'm afraid I can't do that.  🔴", false)
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
		b.Write(env.TextChannel.ID, fmt.Sprintf("🤔...\n%v", err), false)
	} else if cmdAck, ok := cmd.(CommandWithAck); ok && cmdAck.Ack(env) != "" {
		b.React(env.TextChannel.ID, env.TextMessage.ID, cmdAck.Ack(env))
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
