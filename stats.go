package aoebot

import (
	"bytes"
	"fmt"

	"github.com/dustin/go-humanize"
	// "github.com/shirou/gopsutil/cpu"
	"os"
	"runtime"
	"text/tabwriter"
)

// Stats collects runtime information about the bot
// TODO system stats (cpu model, system memory use)
type Stats struct {
	host string
	os   string
	arch string
	// cpu        string
	cpus       int
	memory     uint64
	goroutines int
	goversion  string
	guilds     int
	voices     int
}

// Stats gets up-to-date runtime information about the bot
func (b *Bot) Stats() *Stats {
	s := &Stats{}
	s.host, _ = os.Hostname()
	s.os = runtime.GOOS
	s.arch = runtime.GOARCH
	// cpu, _ := cpu.Info()
	// s.cpu = cpu[0].Model
	s.cpus = runtime.NumCPU()
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)
	s.memory = m.Sys
	s.goroutines = runtime.NumGoroutine()
	s.goversion = runtime.Version()
	s.guilds = len(b.Session.State.Guilds)
	s.voices = len(b.voiceboxes)
	return s
}

func (s Stats) String() string {
	b := &bytes.Buffer{}
	w := tabwriter.NewWriter(b, 0, 4, 0, ' ', 0)
	fmt.Fprintf(w, "```\n")
	fmt.Fprintf(w, "Host: \t%v\n", s.host)
	fmt.Fprintf(w, "OS: \t%v %v\n", s.os, s.arch)
	fmt.Fprintf(w, "Go: \t%v\n", s.goversion)
	// fmt.Fprintf(w, "CPU: \t%v\n", s.cpu)
	fmt.Fprintf(w, "CPUs: \t%v\n", s.cpus)
	fmt.Fprintf(w, "Memory: \t%v\n", humanize.Bytes(s.memory))
	fmt.Fprintf(w, "Routines: \t%v\n", s.goroutines)
	fmt.Fprintf(w, "Guilds: \t%v\n", s.guilds)
	fmt.Fprintf(w, "Voices: \t%v\n", s.voices)
	fmt.Fprintf(w, "```\n")
	w.Flush()
	return b.String()
}
