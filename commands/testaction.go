package commands

import (
	"github.com/jeffreymkabot/aoebot"
	"strings"
)

type TestAction struct{}

func (t *TestAction) Name() string {
	return strings.Fields(t.Usage())[0]
}

func (t *TestAction) Usage() string {
	return `testaction`
}

func (t *TestAction) Short() string {
	return `Test that actions can be dispatched`
}

func (t *TestAction) Long() string {
	return t.Short() + "."
}

func (t *TestAction) IsOwnerOnly() bool {
	return false
}

func (t *TestAction) Run(env *aoebot.Environment, args []string) error {
	var err error
	err = (&aoebot.ReactAction{
		Emoji: `🤖`,
	}).Perform(env)
	if err != nil {
		return err
	}
	err = (&aoebot.WriteAction{
		Content: `Hello World`,
		TTS:     false,
	}).Perform(env)
	if err != nil {
		return err
	}
	return (&aoebot.VoiceAction{
		File: `media/audio/40 enemy.dca`,
	}).Perform(env)
}