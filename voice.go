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
// voice data for a particular guild gets sent to the queue in its corresponding voicebox
type voicebox struct {
	guild *discordgo.Guild
	queue chan<- *voicePayload
	quit  chan<- struct{}
	wait  *sync.WaitGroup
}

type voicePayload struct {
	buffer    [][]byte
	channelID string
}

// dispatch voice data to a particular discord guild
// listen to a queue of voicePayloads for that guild
// voicePayloads provide data meant for a voice channel in a discord guild
// we can remain connected to the same channel while we process a relatively contiguous stream of voicePayloads
// for that channel
func (b *Bot) connectVoicebox(g *discordgo.Guild) *voicebox {
	var vc *discordgo.VoiceConnection
	var err error

	// afk after a certain amount of time not talking
	var afkTimer *time.Timer
	// disconnect voice after a certain amount of time afk
	var dcTimer *time.Timer

	// disconnect() and goAfk() get invoked as the function arg in time.AfterFunc()
	// need to use closures so they can manipulate same VoiceConnection vc used in connectVoicebox()
	disconnect := func() {
		if vc != nil {
			log.Printf("Disconnect voice in guild %v %v", g.Name, g.ID)
			_ = vc.Speaking(false)
			_ = vc.Disconnect()
			vc = nil
		}
	}
	goAfk := func() {
		log.Printf("Join afk channel %v in guild %v %v", g.AfkChannelID, g.Name, g.ID)
		vc, err = b.session.ChannelVoiceJoin(g.ID, g.AfkChannelID, true, true)
		if err != nil {
			log.Printf("Error join afk: %#v", err)
			disconnect()
		} else {
			dcTimer = time.AfterFunc(1*time.Minute, disconnect)
		}
	}
	// defer goAfk()
	queue := make(chan *voicePayload, MaxVoiceQueue)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case vp := <-queue:
				if afkTimer != nil {
					afkTimer.Stop()
				}
				if dcTimer != nil {
					dcTimer.Stop()
				}
				log.Printf("Speak to channel %v in guild %v %v", vp.channelID, g.Name, g.ID)
				vc, err = b.session.ChannelVoiceJoin(g.ID, vp.channelID, false, true)
				if err != nil {
					log.Printf("Error join channel: %#v\n", err)
					afkTimer = time.AfterFunc(300*time.Millisecond, goAfk)
					break
				}
				_ = vc.Speaking(true)
				// time.Sleep(100 * time.Millisecond)
				for _, sample := range vp.buffer {
					vc.OpusSend <- sample
				}
				// time.Sleep(100 * time.Millisecond)
				_ = vc.Speaking(false)
				afkTimer = time.AfterFunc(300*time.Millisecond, goAfk)
			case <-quit:
				if afkTimer != nil {
					afkTimer.Stop()
				}
				if dcTimer != nil {
					dcTimer.Stop()
				}
				log.Printf("Quit voice in guild %v %v", g.Name, g.ID)
				disconnect()
				return
			}
		}
	}()

	return &voicebox{
		guild: g,
		queue: queue,
		quit:  quit,
	}
}

func (b *Bot) reconnectVoicebox(g *discordgo.Guild) (err error) {
	// TODO synchronize connect with end of quit
	vb, ok := b.voiceboxes[g.ID]
	if ok {
		vb.quit <- struct{}{}
	}
	b.voiceboxes[g.ID] = b.connectVoicebox(g)
	return
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
	// close quit chammel and all go routines that receive on it will receive without blocking
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
