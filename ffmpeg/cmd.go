package ffmpeg

import (
	"io"
	"os/exec"
)

type Ffmpeg struct {
	*exec.Cmd
}

func New(red io.Reader) (*Ffmpeg, error) {
	cmd, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}

	ffmpeg := new(Ffmpeg)

	ffmpeg.Cmd = exec.Command(
		cmd,
		"-i", "pipe:0",
		"-c:a", "libopus",
		"-f", "ogg",
		"-ac", "2",
		"-application", "lowdelay",
		"-b:a", "64000",
		"-cutoff", "12000",
		"-compression_level", "0",
		"-packet_loss", "2",
		"-ar", "48000",
		"pipe:1",
	)

	ffmpeg.Stdin = red

	return ffmpeg, nil
}

func (ffmpeg *Ffmpeg) Run() (io.Reader, error) {
	out, err := ffmpeg.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = ffmpeg.Start()
	if err != nil {
		return nil, err
	}

	return out, nil
}
