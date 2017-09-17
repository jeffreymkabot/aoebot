package discordvoice

import (
	"io"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
)

// VoiceConfig
type VoiceConfig struct {
	QueueLength int `toml:"queue_length"`
	SendTimeout int `toml:"send_timeout"`
	IdleTimeout  int `toml:"afk_timeout"`
}

var DefaultConfig = VoiceConfig{
	QueueLength: 100,
	SendTimeout: 1000,
	IdleTimeout:  300,
}

// Payload
type Payload struct {
	ChannelID string
	Reader    io.Reader
}

type VoiceOption func(*VoiceConfig)

func QueueLength(n int) VoiceOption {
	return func(cfg *VoiceConfig) {
		if n > 0 {
			cfg.QueueLength = n
		}
	}
}

func SendTimeout(n int) VoiceOption {
	return func(cfg *VoiceConfig) {
		if n > 0 {
			cfg.SendTimeout = n
		}
	}
}

func IdleTimeout(n int) VoiceOption {
	return func(cfg *VoiceConfig) {
		if n > 0 {
			cfg.IdleTimeout = n
		}
	}
}

// Connect launches a goroutine that dispatches voice to a discord guild
// Queue
// Close
// Since discord allows only one voice connection per guild, you should call close before calling connect again for the same guild
func Connect(s *discordgo.Session, guildID string, idleChannelID string, opts ...VoiceOption) (chan<- *Payload, func()) {
	cfg := DefaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	// close quit channel and all attempts to receive it will receive it without blocking
	// quit channel is hidden from the outside world
	// accessed only through closure for close func
	quit := make(chan struct{})
	queue := make(chan *Payload, cfg.QueueLength)
	close := func() {
		select {
		case <-quit:
			// already closed, don't close a closed channel
			return
		default:
			close(quit)
		}
	}

	join := func(channelID string) (*discordgo.VoiceConnection, error) {
		return s.ChannelVoiceJoin(guildID, channelID, false, true)
	}
	// coerce queue and quit to receieve-only in payloadSender
	go payloadSender(quit, queue, join, idleChannelID, cfg.SendTimeout, cfg.IdleTimeout)
	// coerce queue to send-only in voicebox
	return queue, close
}

func payloadSender(quit <-chan struct{}, queue <-chan *Payload, join func(cID string) (*discordgo.VoiceConnection, error), idleChannelID string, sendTimeout int, idleTimeout int) {
	if queue == nil || quit == nil {
		return
	}

	var vc *discordgo.VoiceConnection
	var err error

	var vp *Payload
	var ok bool

	var reader dca.OpusReader
	var frame []byte

	var idleTimer <-chan time.Time

	disconnect := func() {
		if vc != nil {
			_ = vc.Disconnect()
			vc = nil
		}
	}

	defer disconnect()

	vc, _ = join(idleChannelID)

PayloadLoop:
	for {
		// check quit signal between every payload without blocking
		// otherwise it would be possible for a quit signal to go ignored at least once if there is a continuous stream of voice payloads ready in queue
		// since when multiple cases in a select are ready at same time a case is selected randomly
		select {
		case <-quit:
			return
		default:
		}

		select {
		case <-quit:
			return
		// idletimer is started only once after each payload
		// not every time we enter this select, to prevent repeatedly rejoining idle channel
		case <-idleTimer:
			log.Printf("idle timeout in guild %v", vc.GuildID)
			vc, err = join(idleChannelID)
			if err != nil {
				disconnect()
			}
			continue PayloadLoop
		case vp, ok = <-queue:
			if !ok {
				return
			}
		}

		if vp.ChannelID == idleChannelID {
			continue PayloadLoop
		}
		reader = dca.NewDecoder(vp.Reader)
		vc, err = join(vp.ChannelID)
		if err != nil {
			continue PayloadLoop
		}

		_ = vc.Speaking(true)
	FrameLoop:
		for {
			// check quit signal between every frame
			// when multiple cases in a select are ready at same time a case is selected randomly
			// otherwise it would be possible for a quit signal to go ignored for an unacceptable amount of time if there are a lot of frames in the buffer
			// and vc.OpusSend is ready for every send
			select {
			case <-quit:
				return
			default:
			}

			frame, err = reader.OpusFrame()
			// underlying impl is encoding/binary.Read
			// err is EOF iff no bytes were read
			// err is UnexpectedEOF if partial frame is read
			if err != nil {
				break FrameLoop
			}

			select {
			case <-quit:
				return
			case vc.OpusSend <- frame:
			case <-time.After(time.Duration(sendTimeout) * time.Millisecond):
				log.Printf("Opus send timeout in guild %v", vc.GuildID)
				break FrameLoop
			}
		}
		_ = vc.Speaking(false)
		idleTimer = time.NewTimer(time.Duration(idleTimeout) * time.Millisecond).C
	}
}
