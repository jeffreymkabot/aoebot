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
	AfkTimeout  int `toml:"afk_timeout"`
}

// Payload
type Payload struct {
	ChannelID string
	Reader    io.Reader
}

// Connect launches a goroutine that dispatches voice to a discord guild
// Queue
// Close
// Since discord allows only one voice connection per guild, you should call close before calling connect again for the same guild
func Connect(s *discordgo.Session, g *discordgo.Guild, cfg VoiceConfig) (chan<- *Payload, func()) {
	queue := make(chan *Payload, cfg.QueueLength)
	// close quit channel and all attempts to receive it will receive it without blocking
	// quit channel is hidden from the outside world
	// accessed only through closure for voicebox.close
	quit := make(chan struct{})
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
		return s.ChannelVoiceJoin(g.ID, channelID, false, true)
	}
	// coerce queue and quit to receieve-only in payloadSender
	go payloadSender(join, g.AfkChannelID, queue, quit, cfg.SendTimeout, cfg.AfkTimeout)
	// coerce queue to send-only in voicebox
	return queue, close
}

func payloadSender(join func(channelID string) (*discordgo.VoiceConnection, error), afkChannelID string, queue <-chan *Payload, quit <-chan struct{}, sendTimeout int, afkTimeout int) {
	if queue == nil || quit == nil {
		return
	}

	var vc *discordgo.VoiceConnection
	var err error

	var vp *Payload
	var ok bool

	var reader dca.OpusReader
	var frame []byte

	var afkTimer <-chan time.Time

	disconnect := func() {
		if vc != nil {
			_ = vc.Disconnect()
			vc = nil
		}
	}

	defer disconnect()

	vc, _ = join(afkChannelID)

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
		// afktimer is started only once after each payload
		// not every time we enter this select, to prevent repeatedly rejoining afk
		case <-afkTimer:
			log.Printf("Afk timeout in guild %v", vc.GuildID)
			vc, err = join(afkChannelID)
			if err != nil {
				disconnect()
			}
			continue PayloadLoop
		case vp, ok = <-queue:
			if !ok {
				return
			}
		}

		if vp.ChannelID == afkChannelID {
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
		afkTimer = time.NewTimer(time.Duration(afkTimeout) * time.Millisecond).C
	}
}
