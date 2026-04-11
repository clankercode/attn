package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

var (
	speakerMu         sync.Mutex
	speakerSampleRate beep.SampleRate
)

func Duration(data []byte) (string, error) {
	if len(data) < 44 {
		return "", fmt.Errorf("data too short for WAV header")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return "", fmt.Errorf("not a valid WAV file")
	}

	offset := 12
	var byteRate uint32
	var dataOffset int

	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := binary.LittleEndian.Uint32(data[offset+4 : offset+8])

		if chunkID == "fmt " && chunkSize >= 16 {
			sampleRate := binary.LittleEndian.Uint32(data[offset+12 : offset+16])
			numChannels := binary.LittleEndian.Uint16(data[offset+10 : offset+12])
			bitsPerSample := binary.LittleEndian.Uint16(data[offset+22 : offset+24])
			byteRate = uint32(sampleRate) * uint32(numChannels) * uint32(bitsPerSample) / 8
		} else if chunkID == "data" {
			dataOffset = offset + 8
			break
		}

		offset += 8 + int(chunkSize)
		if offset%2 == 1 && offset < len(data) {
			offset++
		}
	}

	if dataOffset == 0 || byteRate == 0 {
		return "", fmt.Errorf("could not find audio data or format info")
	}

	totalAudioBytes := len(data) - dataOffset
	if byteRate > 0 && totalAudioBytes > 0 {
		sec := float64(totalAudioBytes) / float64(byteRate)
		ms := int(sec*1000) % 1000
		s := int(sec)
		if s > 0 {
			return fmt.Sprintf("%ds %dms", s, ms), nil
		}
		return fmt.Sprintf("%dms", ms), nil
	}

	return "", fmt.Errorf("could not determine audio duration")
}

func playFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(f)
	case ".wav":
		streamer, format, err = wav.Decode(f)
	default:
		f.Close()
		return fmt.Errorf("unsupported audio format: %s", ext)
	}

	if err != nil {
		f.Close()
		return fmt.Errorf("decode: %w", err)
	}

	if err := initSpeaker(format); err != nil {
		streamer.Close()
		f.Close()
		return fmt.Errorf("speaker init: %w", err)
	}

	done := make(chan struct{})
	speaker.Play(streamer)
	speaker.Play(beep.Callback(func() { close(done) }))
	<-done

	streamer.Close()
	f.Close()

	return nil
}

func initSpeaker(format beep.Format) error {
	speakerMu.Lock()
	defer speakerMu.Unlock()

	if speakerSampleRate == format.SampleRate {
		return nil
	}
	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/2)); err != nil {
		return err
	}
	speakerSampleRate = format.SampleRate
	return nil
}

func streamerWithDone(streamer beep.Streamer, done chan<- struct{}) beep.Streamer {
	return beep.Seq(streamer, beep.Callback(func() {
		close(done)
	}))
}

func PlayAndSave(data []byte, outputPath string, doPlay bool, fg bool, waitForLock bool) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if doPlay {
		var lock *lockState
		var lockErr error

		if !fg {
			if waitForLock {
				lock, lockErr = WaitForLock(60000)
			} else {
				lock, lockErr = AcquireLock()
			}
			if lockErr != nil {
				if errors.Is(lockErr, ErrAlreadyPlaying) {
					fmt.Printf("Audio already playing, skipping.\n")
					return nil
				}
				return fmt.Errorf("lock: %w", lockErr)
			}
		}
		defer func() {
			if lock != nil {
				lock.Release()
			}
		}()

		dur, _ := Duration(data)
		player := "Playing"
		if fg {
			player = "Playing (fg)"
		}
		if dur != "" {
			fmt.Printf("%s audio [%s]\n", player, dur)
		} else {
			fmt.Printf("%s audio\n", player)
		}

		return playFile(outputPath)
	}
	return nil
}

func Save(data []byte, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	return os.WriteFile(outputPath, data, 0644)
}

func FormatBytes(n int) string {
	if n >= 1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(n)/1024/1024)
	}
	if n >= 1024 {
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	}
	return fmt.Sprintf("%dB", n)
}

func Play(path string) error {
	return playFile(path)
}

func SuggestPlayback(path string) string {
	return path
}
