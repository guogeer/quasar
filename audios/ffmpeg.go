package audios

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

/*
ffmpeg -i slow.mp3 -filter:a "volume=5dB, atempo=2.0, rubberband=pitch=1" fast.mp3
调整范围为[0.5, 2.0]
*/

/*
volume	float	音量。取值范围[0,10]
speed	float	语速。1为正常速率，2为两倍速
pitch	float	音调。实际测试[-20,20]范围较为合理
*/

type Command struct {
	speed  float64
	pitch  float64
	volume float64

	outputFormat string
}

func NewCommand() *Command {
	return &Command{
		speed:        1.0,
		pitch:        0,
		volume:       1,
		outputFormat: "wav",
	}
}

func (c *Command) SetSpeed(speed float64) {
	c.speed = min(max(speed, 0.5), 5)
}

func (c *Command) SetPitch(pitch float64) {
	pitch = min(max(pitch, -20), 20)
	if pitch <= 0 {
		pitch = max(1+pitch/20, 0.01)
	} else {
		pitch = pitch/10 + 1
	}
	c.pitch = pitch
}

func (c *Command) SetVolume(volume float64) {
	c.volume = min(max(volume, 0), 10)
}

func (c *Command) SetOutputFormat(format string) {
	allowFormats := []string{"mp3", "wav"}
	if slices.Contains(allowFormats, format) {
		c.outputFormat = format
	}
}

func (c *Command) Format(input io.Reader) ([]byte, error) {
	params := []string{}
	params = append(params, fmt.Sprintf("atempo=%v", c.speed))
	params = append(params, fmt.Sprintf("volume=%v", c.volume))
	params = append(params, fmt.Sprintf("rubberband=pitch=%v", c.pitch))
	// params = append(params, "silenceremove=start_periods=0:start_duration=0.2:start_threshold=-50dB")

	cmd := exec.Command("ffmpeg", "-i", "pipe:", "-ar", "44100", "-filter:a", strings.Join(params, ", "), "-f", c.outputFormat, "pipe:")
	cmd.Stdin = input

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return outBuf.Bytes(), nil
}

type Audio struct {
	Duration float64 `json:"duration"` // 音频时长，单位秒
}

// 获取音频信息。支持opus, webm, mp3等格式，不支持pcm格式
// ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 C2S.wav
func GetAudioInfo(path string) (*Audio, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe error: %s, stderr: %s", err.Error(), errBuf.String())
	}

	audioInfo := &Audio{}
	fmt.Sscanf(outBuf.String(), "%f", &audioInfo.Duration)
	return audioInfo, nil
}
