package audio

import (
	"context"
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
	"sync"
	"syscall"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"

	"github.com/clankercode/attn/internal/notify"
)

const (
	detachedPlaybackEnv    = "ATTN_DETACHED_PLAYBACK"
	detachedPlaybackLockFD = 3
)

// Test seams.
var (
	playFileFn            = playFile
	spawnDetachedPlayback = startDetachedPlayback
	connectNotify         = notify.Connect
	copyText              = notify.CopyText
	// reacquireLock never waits: Replay while other audio plays reports
	// busy on the notification instead of queueing.
	reacquireLock = func() (func(), error) {
		lock, err := AcquireLock()
		if err != nil {
			return nil, err
		}
		return func() { lock.Release() }, nil
	}
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

func playFile(ctx context.Context, path string) error {
	if sink, err := detectPCMSink(); err == nil {
		if perr := playFilePCM(ctx, path, sink); perr == nil {
			return nil
		} else if ctx.Err() != nil {
			// Stopped by the user: don't fall back to another player.
			return ctx.Err()
		} else if fileSink, ferr := detectFileSink(); ferr == nil {
			return playFileDirect(ctx, path, fileSink)
		} else {
			return perr
		}
	}
	if fileSink, err := detectFileSink(); err == nil {
		return playFileDirect(ctx, path, fileSink)
	}
	return fmt.Errorf("no supported playback program found (tried pw-play, pacat, paplay, ffplay, mpv)")
}

func playFilePCM(ctx context.Context, path, sink string) error {
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

	return streamToSink(ctx, streamer, format, sink)
}

func playFileDirect(ctx context.Context, path, sink string) error {
	name, args := fileSinkCommand(sink, path)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}

func fileSinkCommand(sink, path string) (string, []string) {
	switch sink {
	case "ffplay":
		return sink, []string{"-nodisp", "-autoexit", "-loglevel", "error", path}
	case "mpv":
		return sink, []string{"--no-video", "--really-quiet", path}
	default:
		return sink, []string{path}
	}
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

func detectPCMSink() (string, error) {
	for _, name := range []string{"pw-play", "pacat", "paplay"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no PCM playback sink found")
}

func detectFileSink() (string, error) {
	for _, name := range []string{"ffplay", "mpv"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no file-mode playback program found")
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

func streamToSink(ctx context.Context, streamer beep.Streamer, format beep.Format, sink string) error {
	name, args := playbackCommand(sink, format.SampleRate)
	cmd := exec.CommandContext(ctx, name, args...)
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

	writeErr := streamPCM(ctx, streamer, stdin)
	closeErr := stdin.Close()
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}

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
	if _, err := lockFile.Stat(); err != nil {
		return true, fmt.Errorf("invalid inherited playback lock: %w", err)
	}
	// The inherited lock fd is not close-on-exec. Without this, helpers we
	// spawn (the clipboard daemon forked by wl-copy in particular) would
	// hold the playback lock long after this process exits.
	syscall.CloseOnExec(detachedPlaybackLockFD)

	meta := notify.DecodeMeta(os.Getenv(notify.MetaEnv))
	os.Unsetenv(notify.MetaEnv)
	os.Unsetenv(detachedPlaybackEnv)
	// We may linger for a long time: don't pin the caller's directory (the
	// audio path is absolute and the labels were computed by the parent).
	_ = os.Chdir("/")

	var once sync.Once
	release := func() { once.Do(func() { lockFile.Close() }) }
	defer release()
	return true, playDetached(args[0], meta, release)
}

// playDetached plays path with its notification, then keeps the
// notification's Replay / Copy buttons live for meta.Linger.
func playDetached(path string, meta notify.Meta, release func()) error {
	return runWithNotification(path, meta, release, reacquireLock)
}

func runWithNotification(path string, meta notify.Meta, release func(), reacquire func() (func(), error)) error {
	var srv notify.Server
	if !meta.Disabled {
		if s, err := connectNotify(); err == nil {
			srv = s
		}
	}
	return notify.Run(meta, notify.Deps{
		Server:    srv,
		Play:      func(ctx context.Context) error { return playFileFn(ctx, path) },
		Release:   release,
		Reacquire: reacquire,
		Copy:      copyText,
	})
}

func startDetachedPlayback(path string, lock *lockState, meta notify.Meta) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	// The child changes to / so it doesn't pin our cwd.
	if path, err = filepath.Abs(path); err != nil {
		return fmt.Errorf("resolve audio path: %w", err)
	}

	cmd := exec.Command(exe, path)
	cmd.Env = append(os.Environ(), detachedPlaybackEnv+"=1", notify.MetaEnv+"="+meta.Encode())
	cmd.ExtraFiles = []*os.File{lock.file}
	// /dev/null, not pipes: the child outlives us (lingering notification),
	// and a write to a pipe whose reader has exited would SIGPIPE it.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Don't leak fds inherited from our caller into a long-lived child
	// (ExtraFiles, i.e. the lock, are still passed).
	closeInheritedFDsOnExec()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start detached playback: %w", err)
	}
	return nil
}

func streamPCM(ctx context.Context, streamer beep.Streamer, w io.Writer) error {
	buf := make([][2]float64, 2048)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
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

// PlayAndSave writes data to outputPath and, when doPlay, plays it with a
// desktop notification described by meta.
func PlayAndSave(data []byte, outputPath string, doPlay bool, fg bool, waitForLock bool, meta notify.Meta) error {
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
			// The caller is waiting on us, so no lingering after playback.
			meta.Linger = 0
			return runWithNotification(outputPath, meta, nil, nil)
		}
		return spawnDetachedPlayback(outputPath, lock, meta)
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
	return playFile(context.Background(), path)
}

func SuggestPlayback(path string) string {
	return path
}
