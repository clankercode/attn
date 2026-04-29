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

var VoiceListMimo = []string{
	"mimo_default",
	"default_zh",
	"default_en",
}

var MimoStylePresets = []string{
	"开心",
	"生气",
	"温柔",
	"悄悄话",
	"东北话",
	"四川话",
	"河南话",
	"粤语",
	"台湾腔",
	"夹子音",
	"焦急",
	"悲伤",
	"紧张",
	"虚弱",
	"激昂慷慨",
	"慵懒",
	"变快",
	"变慢",
	"唱歌",
	"孙悟空",
	"林黛玉",
}

var MimoStylePresetsEnglish = []string{
	"Happy",
	"Angry",
	"Gentle",
	"Whisper",
	"Northeastern accent",
	"Sichuan accent",
	"Henan accent",
	"Cantonese",
	"Taiwanese accent",
	"Breathy/soft voice",
	"Anxious",
	"Sad",
	"Nervous",
	"Weak",
	"Passionate",
	"Lazy",
	"Faster",
	"Slower",
	"Singing",
	"Sun Wukong (Monkey King)",
	"Lin Daiyu (from Dream of Red Chamber)",
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
	case ProviderMimo:
		for _, v := range VoiceListMimo {
			if v == voice {
				return true
			}
		}
		return false
	}
	return true
}

func RandomVoice(provider ProviderType) string {
	switch provider {
	case ProviderGroq:
		return VoiceListGroq[randomSource.Intn(len(VoiceListGroq))]
	case ProviderMimo:
		return VoiceListMimo[randomSource.Intn(len(VoiceListMimo))]
	default:
		return VoiceListMinimax[randomSource.Intn(len(VoiceListMinimax))]
	}
}

func ValidateStyle(style string) bool {
	for _, v := range MimoStylePresets {
		if v == style {
			return true
		}
	}
	for _, v := range MimoStylePresetsEnglish {
		if v == style {
			return true
		}
	}
	return false
}

func ResolveStyle(style string) string {
	for _, v := range MimoStylePresets {
		if v == style {
			return v
		}
	}
	for idx, v := range MimoStylePresetsEnglish {
		if v == style {
			return MimoStylePresets[idx]
		}
	}
	return style
}
