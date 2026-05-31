package music

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// OpusStream produces Opus frames from an audio source. Frames are delivered on
// Frames; Err (after Frames closes) reports any terminal error.
type OpusStream struct {
	Frames chan []byte
	errMu  chan error
	cancel context.CancelFunc
}

// Err returns the terminal error after the Frames channel has closed.
func (o *OpusStream) Err() error {
	select {
	case err := <-o.errMu:
		return err
	default:
		return nil
	}
}

// Stop terminates the underlying subprocesses.
func (o *OpusStream) Stop() {
	if o.cancel != nil {
		o.cancel()
	}
}

// EncodeStream pipes the source URL through ffmpeg (PCM) into the dca encoder
// and reads back length-prefixed Opus frames. This keeps the bot binary
// CGO-free: all Opus work happens in the dca subprocess.
//
// volume is 0..256 where 256 is unity gain (matches dca's --vol scale).
func EncodeStream(parent context.Context, ffmpegBin, dcaBin, streamURL string, volume int) (*OpusStream, error) {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	if dcaBin == "" {
		dcaBin = "dca"
	}
	if volume <= 0 {
		volume = 256
	}

	ctx, cancel := context.WithCancel(parent)

	ffmpeg := exec.CommandContext(ctx, ffmpegBin,
		"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5",
		"-i", streamURL,
		"-f", "s16le", "-ar", "48000", "-ac", "2",
		"-loglevel", "error",
		"pipe:1",
	)
	ffmpegOut, err := ffmpeg.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	dca := exec.CommandContext(ctx, dcaBin, "--vol", fmt.Sprintf("%d", volume))
	dca.Stdin = ffmpegOut
	dcaOut, err := dca.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("dca stdout pipe: %w", err)
	}

	if err := ffmpeg.Start(); err != nil {
		cancel()
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("ffmpeg is not installed; install it to enable music playback")
		}
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	if err := dca.Start(); err != nil {
		cancel()
		_ = ffmpeg.Process.Kill()
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("dca encoder is not installed; install dca-rs or dca to enable music playback")
		}
		return nil, fmt.Errorf("start dca: %w", err)
	}

	stream := &OpusStream{
		Frames: make(chan []byte, 64),
		errMu:  make(chan error, 1),
		cancel: cancel,
	}

	go func() {
		defer close(stream.Frames)
		defer cancel()
		err := readDCAFrames(dcaOut, stream.Frames)
		_ = ffmpeg.Wait()
		_ = dca.Wait()
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(ctx.Err(), context.Canceled) {
			stream.errMu <- err
		}
	}()

	return stream, nil
}

// readDCAFrames reads length-prefixed Opus frames from a dca stream. It skips an
// optional DCA1 JSON metadata header if present.
func readDCAFrames(r io.Reader, out chan<- []byte) error {
	br := bufio.NewReaderSize(r, 32*1024)

	// Detect and skip the optional DCA1 metadata header.
	magic, err := br.Peek(4)
	if err == nil && string(magic) == "DCA1" {
		if _, err := br.Discard(4); err != nil {
			return err
		}
		var metaLen int32
		if err := binary.Read(br, binary.LittleEndian, &metaLen); err != nil {
			return err
		}
		if metaLen > 0 {
			if _, err := br.Discard(int(metaLen)); err != nil {
				return err
			}
		}
	}

	for {
		var frameLen int16
		if err := binary.Read(br, binary.LittleEndian, &frameLen); err != nil {
			return err
		}
		if frameLen <= 0 {
			continue
		}
		frame := make([]byte, frameLen)
		if _, err := io.ReadFull(br, frame); err != nil {
			return err
		}
		out <- frame
	}
}
