package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func PlayMpvBg(path string) (waitForFinish func(), err error) {
	cmd := exec.Command("mpv", "--quiet", "--no-terminal", "--idle=no", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	return func() { cmd.Wait() }, nil
}

func PlayMpvFg(path string) error {
	cmd := exec.Command("mpv", "--quiet", "--no-terminal", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
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

		_ = lock // suppress unused warning when fg=true
		var playErr error
		if fg {
			playErr = PlayMpvFg(outputPath)
		} else {
			wait, bgErr := PlayMpvBg(outputPath)
			if bgErr == nil {
				wait()
				if lock != nil {
					lock.Release()
				}
			}
		}
		return playErr
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

func SuggestPlayback(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return "mpv --no-terminal " + path
	case ".wav":
		return "mpv --no-terminal " + path
	case ".flac":
		return "mpv --no-terminal " + path
	default:
		return "mpv --no-terminal " + path
	}
}
