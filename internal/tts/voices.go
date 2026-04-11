package tts

import (
	"math/rand"
	"time"
)

var randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

var VoiceListGroq = []string{
	"autumn",
	"diana",
	"hannah",
	"austin",
	"daniel",
	"troy",
}

var VoiceListMinimax = []string{
	"Wise_Woman",
	"Friendly_Person",
	"Deep_Voice_Man",
	"Calm_Woman",
	"Casual_Guy",
	"Lively_Girl",
	"Patient_Man",
	"Young_Knight",
	"Determined_Man",
	"Lovely_Girl",
	"Decent_Boy",
	"Imposing_Manner",
	"Elegant_Man",
	"Abbess",
	"Sweet_Girl_2",
	"Inspirational_girl",
}

func ValidateVoice(provider ProviderType, voice string) bool {
	switch provider {
	case ProviderGroq:
		for _, v := range VoiceListGroq {
			if v == voice {
				return true
			}
		}
		return false
	case ProviderMinimax:
		for _, v := range VoiceListMinimax {
			if v == voice {
				return true
			}
		}
		return false
	}
	return true
}

func RandomVoice(provider ProviderType) string {
	voices := VoiceListMinimax
	if provider == ProviderGroq {
		voices = VoiceListGroq
	}
	if len(voices) == 0 {
		return ""
	}
	return voices[randomSource.Intn(len(voices))]
}
