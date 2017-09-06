package commands

import (
	"github.com/jeffreymkabot/aoebot"
	"strings"
)

type TestReact struct{}

func (tr *TestReact) Name() string {
	return strings.Fields(tr.Usage())[0]
}

func (tr *TestReact) Usage() string {
	return `testreact`
}

func (tr *TestReact) Short() string {
	return `Test that react actions can be dispatched`
}

func (tr *TestReact) Long() string {
	return tr.Short()
}

func (tr *TestReact) IsOwnerOnly() bool {
	return false
}

func (tr *TestReact) Run(env *aoebot.Environment, args []string) error {
	return (&aoebot.ReactAction{
		Emoji: `🤖`,
	}).Perform(env)
}

type TestWrite struct{}

func (tw *TestWrite) Name() string {
	return strings.Fields(tw.Usage())[0]
}

func (tw *TestWrite) Usage() string {
	return `testwrite`
}

func (tw *TestWrite) Short() string {
	return `Test that write actions can be dispatched`
}

func (tw *TestWrite) Long() string {
	return tw.Short()
}

func (tw *TestWrite) IsOwnerOnly() bool {
	return false
}

func (tw *TestWrite) Run(env *aoebot.Environment, args []string) error {
	return (&aoebot.WriteAction{
		Content: `Hello World`,
		TTS:     false,
	}).Perform(env)
}

type TestVoice struct{}

func (tv *TestVoice) Name() string {
	return strings.Fields(tv.Usage())[0]
}

func (tv *TestVoice) Usage() string {
	return `testvoice`
}

func (tv *TestVoice) Short() string {
	return `Test that voice actions can be dispatched`
}

func (tv *TestVoice) Long() string {
	return tv.Short()
}

func (tv *TestVoice) IsOwnerOnly() bool {
	return false
}

func (tv *TestVoice) Run(env *aoebot.Environment, args []string) error {
	return (&aoebot.VoiceAction{
		File: `media/audio/40 enemy.dca`,
	}).Perform(env)
}
