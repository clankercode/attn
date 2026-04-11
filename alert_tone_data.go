package main

import _ "embed"

//go:embed alert_tone.wav
var alertTone []byte

func AlertTone() []byte {
	return alertTone
}
