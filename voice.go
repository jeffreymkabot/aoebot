package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"sync"
	"time"
)

const (
	// MaxVoiceQueue is the maximum number of voice payloads that can wait to be processed for a particular guild
	MaxVoiceQueue = 100
	// VoiceSendTimeout is the amount of time to wait on the voice send channel before giving up on a voice payload
	VoiceSendTimeout = 1 * time.Second
	// AfkTimeout is the amount of time to wait for another voice payload before joining the guild's afk channel
	AfkTimeout = 300 * time.Millisecond
)

// Voicebox
// Creating a voicebox with newVoiceBox launches a goroutine that listens to payloads on the voicebox's queue channel
// Bot should create
type voicebox struct {
	queue chan<- *voicePayload
	quit  chan<- struct{}
	wait  *sync.WaitGroup
}

type voicePayload struct {
	buffer    [][]byte
	channelID string
}

// SpeakTo opens the conversation with a discord guild
func (b *Bot) SpeakTo(g *discordgo.Guild) {
	vb, ok := b.voiceboxes[g.ID]
	if ok && vb.quit != nil {
		close(vb.quit)
		vb.quit = nil
		// Use a wait group so the bot can finish disconnecting the old voice connection before making a new worker
		vb.wait.Wait()
	}
	b.voiceboxes[g.ID] = newVoiceBox(b.session, g)
}

func newVoiceBox(s *discordgo.Session, g *discordgo.Guild) *voicebox {
	queue := make(chan *voicePayload, MaxVoiceQueue)
	// close quit channel and all go routines that receive on it will receive it without blocking
	quit := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go payloadSender(s, g, queue, quit, wg)
	return &voicebox{
		queue: queue,
		quit:  quit,
		wait:  wg,
	}
}

func (vb *voicebox) close() {
	if vb.quit != nil {
		close(vb.quit)
		vb.quit = nil
		// Wait for the voicebox's payloadSender goroutine to return
		vb.wait.Wait()
	}
}

func payloadSender(s *discordgo.Session, g *discordgo.Guild, queue <-chan *voicePayload, quit <-chan struct{}, wg *sync.WaitGroup) {
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
			// TODO this could be a memory leak if we keep making new timers
			case <-time.After(VoiceSendTimeout):
				log.Printf("Opus send timeout in guild %v", g.ID)
				break FrameLoop
			}
		}
		_ = vc.Speaking(false)
		// TODO this could be a memory leak if we keep making new timers
		afkTimer = time.NewTimer(AfkTimeout).C
	}
}

// TODO voice worker pipeline instead of voicebox god function?
func (b *Bot) newVoiceWorker(g *discordgo.Guild) *voicebox {
	return nil
}
