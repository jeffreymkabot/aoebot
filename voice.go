package aoebot

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"sync"
	"time"
)

type VoiceConfig struct {
	QueueLength int `toml:"queue_length"`
	SendTimeout int `toml:"send_timeout"`
	AfkTimeout  int `toml:"afk_timeout"`
}

// Voicebox
// Creating a voicebox with newVoiceBox launches a goroutine that listens to payloads on the voicebox's queue channel
// Since discord permits only one voice connection per guild,
// Bot should create exactly one voicebox for each guild
// Voicebox is like a specialized botroutine
type voicebox struct {
	queue chan<- *voicePayload
	close func()
}

type voicePayload struct {
	buffer    [][]byte
	channelID string
}

// speakTo opens the conversation with a discord guild
func (b *Bot) speakTo(g *discordgo.Guild) {
	vb, ok := b.voiceboxes[g.ID]
	if ok {
		vb.close()
	}
	b.voiceboxes[g.ID] = newVoiceBox(b.session, g, b.config.Voice)
}

func newVoiceBox(s *discordgo.Session, g *discordgo.Guild, cfg VoiceConfig) *voicebox {
	queue := make(chan *voicePayload, cfg.QueueLength)
	// close quit channel and all attempts to receive it will receive it without blocking
	// quit channel is hidden from the outside world
	// accessed only through closure for voicebox.close
	quit := make(chan struct{})
	// Use a wait group so the bot can finish disconnecting the old voice connection before e.g. making a new worker
	wg := &sync.WaitGroup{}
	close := func() {
		select {
		case <-quit:
			// already closed, don't close a closed channel
			return
		default:
			close(quit)
			// wait for payloadSender to return and hopefully disconnect from voice channel
			wg.Wait()
		}
	}
	wg.Add(1)
	go payloadSender(s, g, queue, quit, cfg.SendTimeout, cfg.AfkTimeout, wg)
	return &voicebox{
		queue: queue,
		close: close,
	}
}

func payloadSender(s *discordgo.Session, g *discordgo.Guild, queue <-chan *voicePayload, quit <-chan struct{}, sendTimeout int, afkTimeout int, wg *sync.WaitGroup) {
	defer wg.Done()
	if queue == nil || quit == nil {
		return
	}

	var vc *discordgo.VoiceConnection
	var err error

	var vp *voicePayload
	var ok bool

	var frame []byte

	var afkTimer <-chan time.Time

	disconnect := func() {
		if vc != nil && vc.Ready {
			_ = vc.Speaking(false)
			_ = vc.Disconnect()
			vc = nil
		}
	}

	defer disconnect()

	vc, _ = s.ChannelVoiceJoin(g.ID, g.AfkChannelID, true, true)

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
		case vp, ok = <-queue:
			if !ok {
				return
			}
		case <-afkTimer:
			log.Printf("Afk timeout in guild %v", g.ID)
			// if vc != nil && vc.ChannelID == g.AfkChannelID {
			// 	continue PayloadLoop
			// }
			vc, err = s.ChannelVoiceJoin(g.ID, g.AfkChannelID, true, true)
			if err != nil {
				disconnect()
			}
			continue PayloadLoop
		}

		if vp.channelID == g.AfkChannelID {
			continue PayloadLoop
		}
		vc, err = s.ChannelVoiceJoin(g.ID, vp.channelID, false, true)
		if err != nil {
			continue PayloadLoop
		}

		_ = vc.Speaking(true)
	FrameLoop:
		for _, frame = range vp.buffer {
			// check quit signal between every frame
			// when multiple cases in a select are ready at same time a case is selected randomly
			// otherwise it would be possible for a quit signal to go ignored for an unacceptable amount of time if there are a lot of frames in the buffer
			// and vc.OpusSend is ready for every send
			select {
			case <-quit:
				return
			default:
			}

			select {
			case <-quit:
				return
			case vc.OpusSend <- frame:
			// TODO this could be a memory leak if we keep making new timers?
			case <-time.After(time.Duration(sendTimeout) * time.Millisecond):
				log.Printf("Opus send timeout in guild %v", g.ID)
				break FrameLoop
			}
		}
		_ = vc.Speaking(false)
		// TODO this could be a memory leak if we keep making new timers?
		afkTimer = time.NewTimer(time.Duration(afkTimeout) * time.Millisecond).C
	}
}

// TODO voice worker pipeline instead of voicebox god function?
func (b *Bot) newVoiceWorker(g *discordgo.Guild) *voicebox {
	return nil
}
