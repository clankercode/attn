package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"
)

const (
	detachedPlaybackEnv    = "ATTN_DETACHED_PLAYBACK"
	detachedPlaybackLockFD = 3
)

var (
	playFileFn            = playFile
	spawnDetachedPlayback = startDetachedPlayback
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
	defer f.Close()

	streamer, format, err := decodeAudio(f, path)
	if err != nil {
		return err
	}
	defer streamer.Close()

	sink, err := detectPlaybackSink()
	if err != nil {
		return err
	}

	return streamToSink(streamer, format, sink)
}

func decodeAudio(r io.ReadCloser, path string) (beep.StreamSeekCloser, beep.Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		streamer, format, err := mp3.Decode(r)
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("decode: %w", err)
		}
		return streamer, format, nil
	case ".wav":
		streamer, format, err := wav.Decode(r)
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("decode: %w", err)
		}
		return streamer, format, nil
	default:
		return nil, beep.Format{}, fmt.Errorf("unsupported audio format: %s", ext)
	}
}

func detectPlaybackSink() (string, error) {
	for _, name := range []string{"pw-play", "pacat", "paplay"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no supported playback sink found (tried pw-play, pacat, paplay)")
}

func playbackCommand(sink string, sampleRate beep.SampleRate) (string, []string) {
	rate := strconv.Itoa(int(sampleRate))
	switch sink {
	case "pw-play":
		return sink, []string{"--raw", "--format", "s16", "--rate", rate, "--channels", "2", "--latency", "50ms", "-"}
	case "paplay":
		return sink, []string{"--raw", "--format=s16le", "--rate=" + rate, "--channels=2", "--latency-msec=50", "-"}
	default:
		return sink, []string{"--raw", "--format=s16le", "--rate=" + rate, "--channels=2", "--latency-msec=50", "-"}
	}
}

func streamToSink(streamer beep.Streamer, format beep.Format, sink string) error {
	name, args := playbackCommand(sink, format.SampleRate)
	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open playback stdin: %w", err)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("start %s: %w", name, err)
	}

	writeErr := streamPCM(streamer, stdin)
	closeErr := stdin.Close()
	waitErr := cmd.Wait()

	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("close playback stdin: %w", closeErr)
	}
	if waitErr != nil {
		return fmt.Errorf("wait for %s: %w", name, waitErr)
	}
	return nil
}

func HandleDetachedPlayback(args []string) (bool, error) {
	if os.Getenv(detachedPlaybackEnv) != "1" {
		return false, nil
	}
	if len(args) != 1 {
		return true, fmt.Errorf("detached playback expects exactly one path")
	}

	lockFile := os.NewFile(uintptr(detachedPlaybackLockFD), "attn-playback-lock")
	if lockFile == nil {
		return true, fmt.Errorf("missing inherited playback lock")
	}
	defer lockFile.Close()

	if _, err := lockFile.Stat(); err != nil {
		return true, fmt.Errorf("invalid inherited playback lock: %w", err)
	}

	return true, playFile(args[0])
}

func startDetachedPlayback(path string, lock *lockState) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	cmd := exec.Command(exe, path)
	cmd.Env = append(os.Environ(), detachedPlaybackEnv+"=1")
	cmd.ExtraFiles = []*os.File{lock.file}
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start detached playback: %w", err)
	}
	return nil
}

func streamPCM(streamer beep.Streamer, w io.Writer) error {
	buf := make([][2]float64, 2048)
	for {
		n, ok := streamer.Stream(buf)
		if n > 0 {
			if _, err := w.Write(samplesToPCM16LE(buf[:n])); err != nil {
				return fmt.Errorf("write PCM: %w", err)
			}
		}
		if !ok {
			return nil
		}
	}
}

func samplesToPCM16LE(samples [][2]float64) []byte {
	out := make([]byte, len(samples)*4)
	for i, sample := range samples {
		left := pcm16(sample[0])
		right := pcm16(sample[1])
		binary.LittleEndian.PutUint16(out[i*4:], uint16(left))
		binary.LittleEndian.PutUint16(out[i*4+2:], uint16(right))
	}
	return out
}

func pcm16(v float64) int16 {
	v = math.Max(-1, math.Min(1, v))
	if v <= -1 {
		return -32768
	}
	return int16(math.Round(v * 32767))
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

		if fg {
			return playFileFn(outputPath)
		}
		return spawnDetachedPlayback(outputPath, lock)
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
