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

const (
	MaxGuildVoiceActionDuration = 3 * time.Second
	MaxGuildCustomConditions    = 50
	// MaxGuildManagedChannels is the maximum number of ad hoc channels per guild that aoebot is allowed to have created at any given time
	MaxGuildManagedChannels = 3
	// ManagedChannelTimeout is how frequently the bot will poll a managed channel to see if it shoudl be deleted
	ManagedChannelTimeout = 60 * time.Second
)

// Bot represents a discord bot
type Bot struct {
	mu         sync.Mutex // TODO synchronize state and use of maps
	kill       chan struct{}
	killer     error
	token      string
	owner      string
	prefix     string
	commands   []*command
	driver     Driver
	session    *discordgo.Session
	self       *discordgo.User
	routines   map[*botroutine]struct{} // Set
	unhandlers map[*func()]struct{}     // Set
	voiceboxes map[string]*voicebox     // TODO voiceboxes is vulnerable to concurrent read/write
	occupancy  map[string]string        // TODO occupancy is vulnerable to concurrent read/write
	aesthetic  bool
}

// New initializes a bot
func New(token string, owner string, driver Driver) (b *Bot, err error) {
	b = &Bot{
		kill:       make(chan struct{}),
		token:      token,
		owner:      owner,
		prefix:     `@!`,
		driver:     driver,
		routines:   make(map[*botroutine]struct{}),
		unhandlers: make(map[*func()]struct{}),
		voiceboxes: make(map[string]*voicebox),
		occupancy:  make(map[string]string),
	}
	b.session, err = discordgo.New("Bot " + b.token)
	if err != nil {
		return
	}
	// b.session.LogLevel = discordgo.LogWarning
	b.commands = []*command{
		help,
		addchannel,
		getmemes,
		addreact,
		delreact,
		addwrite,
		delwrite,
		addvoice,
		delvoice,
		stats,
		source,
		testwrite,
		testreact,
		testvoice,
		reconnect,
		restart,
		shutdown,
	}
	return
}

// SetDriver assigns a new Driver to the Bot
func (b *Bot) SetDriver(d Driver) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.driver = d
}

// modeled after default package context
func (b *Bot) die(err error) {
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

// Start initiates a and discord session
func (b *Bot) Start() (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		return
	}

	b.self, err = b.session.User("@me")
	if err != nil {
		return
	}

	b.addHandlerOnce(b.onReady())
	// begin listen to discord websocket for events
	// invoking session.Open() triggers the discord ready event
	err = b.session.Open()
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

	for f := range b.unhandlers {
		if f != nil {
			(*f)()
		}
		delete(b.unhandlers, f)
	}

	for r := range b.routines {
		r.close()
		delete(b.routines, r)
	}

	for k, vb := range b.voiceboxes {
		vb.close()
		delete(b.voiceboxes, k)
	}

	// close the session after closing voice boxes since closing voiceboxes attempts graceful disconnect using discord session
	b.session.Close()

	log.Printf("...closed session.")
}

// Write a message to a channel in a guild
func (b *Bot) Write(channelID string, message string, tts bool) (err error) {
	if tts {
		_, err = b.session.ChannelMessageSendTTS(channelID, message)
	} else {
		_, err = b.session.ChannelMessageSend(channelID, message)
	}
	return
}

// React with an emoji to a message in a channel in a guild
func (b *Bot) React(channelID string, messageID string, emoji string) (err error) {
	err = b.session.MessageReactionAdd(channelID, messageID, emoji)
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
	unhandler := b.session.AddHandler(handler)
	b.unhandlers[&unhandler] = struct{}{}
}

// Add a one-time event handler to the discord session and retain a reference to the handler remover
func (b *Bot) addHandlerOnce(handler interface{}) {
	unhandler := b.session.AddHandlerOnce(handler)
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

func (b *Bot) addRoutine(f func(<-chan struct{})) {
	r := newRoutine(f)
	b.routines[r] = struct{}{}
}

// IsOwnEnvironment is true when an environment references the bot's own actions/behavior
// This is useful to prevent the bot from reacting to itself
func (b *Bot) IsOwnEnvironment(env *Environment) bool {
	return env.Author != nil && env.Author.ID == b.self.ID
}

func (b *Bot) onReady() func(s *discordgo.Session, r *discordgo.Ready) {
	// Function signature needs to be exact to be detected as the Ready handler by discordgo
	// Access b Bot through a closure
	return func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Got discord ready: %#v\n", r)
		for _, g := range r.Guilds {
			b.registerGuild(g)
		}
		b.addHandler(b.onGuildCreate())
		b.addHandler(b.onMessageCreate())
		b.addHandler(b.onVoiceStateUpdate())
	}
}

func (b *Bot) registerGuild(g *discordgo.Guild) {
	b.speakTo(g)
	for _, vs := range g.VoiceStates {
		// TODO bots could be in a channel in multiple guilds
		b.occupancy[vs.UserID] = vs.ChannelID
	}
	channels := b.driver.ChannelsGuild(g.ID)
	if len(channels) > 0 {
		delete := func(ch Channel) {
			log.Printf("Deleting channel %s", ch.Name)
			_, _ = b.session.ChannelDelete(ch.ID)
			_ = b.driver.ChannelDelete(ch.ID)
		}
		isEmpty := func(ch Channel) bool {
			for _, v := range g.VoiceStates {
				if v.ChannelID == ch.ID {
					return false
				}
			}
			return true
		}
		for _, ch := range channels {
			b.addRoutine(channelManager(ch, delete, isEmpty))
		}
	}
}

func (b *Bot) onGuildCreate() func(*discordgo.Session, *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		if g.Guild == nil {
			return
		}
		b.registerGuild(g.Guild)
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

		env, err := NewEnvironment(s, m.Message)
		if err != nil {
			log.Printf("Error resolving environment of new message: %v", err)
			return
		}
		log.Printf("Saw a new message (%v) by %s in channel %v in guild %v", env.TextMessage.Content, env.Author, env.TextChannel.Name, env.Guild.Name)
		if b.IsOwnEnvironment(env) {
			return
		}

		if strings.HasPrefix(env.TextMessage.Content, b.prefix) {
			args := strings.Fields(strings.TrimSpace(strings.TrimPrefix(env.TextMessage.Content, b.prefix)))
			cmd, args := b.command(args)
			log.Printf("Exec cmd %v by %s with %v", cmd.name(), env.Author, args)
			b.exec(env, cmd, args)
		} else {
			actions := b.driver.Actions(env)
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

			env, err := NewEnvironment(s, v.VoiceState)
			if err != nil {
				log.Printf("Error resolving voice state context: %v", err)
				return
			}
			log.Printf("Saw user %s join the voice channel %v in guild %v", env.Author, env.VoiceChannel.Name, env.Guild.Name)
			if b.IsOwnEnvironment(env) {
				return
			}

			// %

			actions := b.driver.Actions(env)
			log.Printf("Found actions %v", actions)
			b.dispatch(env, actions...)
		}
	}
}

func (b *Bot) command(args []string) (*command, []string) {
	if len(args) > 0 {
		cmd := strings.ToLower(args[0])
		args := args[1:]
		for _, c := range b.commands {
			if c.name() == cmd {
				return c, args
			}
		}
	}
	return help, []string{}
}

func (b *Bot) exec(env *Environment, cmd *command, args []string) {
	if cmd.isProtected && env.Author.ID != b.owner {
		_ = b.Write(env.TextChannel.ID, "Sorry, only dad can use that one 🙃", false)
		return
	}
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Recovered from panic in exec %v with %v: %v", cmd.name(), args, err)
		}
	}()

	err := cmd.run(b, env, args)
	if err != nil {
		log.Printf("Error in exec %v with %v: %v", cmd.name(), args, err)
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
			err := (a.performFunc(env))(b)
			if err != nil {
				log.Printf("Error in perform %T on %v: %v", a, env.Type, err)
			}
		}(a)
	}
}
