package audiovad

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/wav"
	"github.com/guogeer/quasar/v2/audios"
	"github.com/streamer45/silero-vad-go/speech"
)

const vadSampleRate = 16000

var errVADStarted = errors.New("vad started")

type VAD struct {
	detector *speech.Detector

	format     string
	sampleRate int
	endAt      float64
	detectBuf  []float32
	audio      *bytes.Buffer
	stream     beep.Streamer
}

func NewVAD(model string, sampleRate int, format string) (*VAD, error) {
	if slices.Index([]string{"wav", "pcm", "mp3"}, format) < 0 {
		return nil, fmt.Errorf("invalid audio format %s", format)
	}

	sd, err := speech.NewDetector(speech.DetectorConfig{
		ModelPath:  model,
		SampleRate: vadSampleRate,
		Threshold:  0.5,
		// MinSilenceDurationMs: 100,
		SpeechPadMs: 30,
	})
	if err != nil {
		return nil, err
	}
	return &VAD{detector: sd, format: format, sampleRate: sampleRate, audio: &bytes.Buffer{}}, nil
}

func (vad *VAD) check() error {
	if vad.stream != nil {
		return errVADStarted
	}
	return nil
}

func (vad *VAD) SetSampleRate(sampleRate int) error {
	if err := vad.check(); err != nil {
		return err
	}
	vad.sampleRate = sampleRate
	return nil
}

func (vad *VAD) SetAudioFormat(format string) error {
	if err := vad.check(); err != nil {
		return err
	}
	vad.format = format
	return nil
}

func (vad *VAD) Write(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	if vad.stream == nil && vad.format == "pcm" {
		vad.audio.Write(audios.CreateWavHeader(vad.sampleRate, 16, 1))
	}

	vad.audio.Write(chunk)
	if vad.stream != nil {
		return nil
	}

	switch vad.format {
	case "wav", "pcm":
		streamCloser, _, err := wav.Decode(vad.audio)
		if err != nil {
			return err
		}
		vad.stream = streamCloser
	case "mp3":
		streamCloser, _, err := mp3.Decode(io.NopCloser(vad.audio))
		if err != nil {
			return err
		}
		vad.stream = streamCloser
	}

	if vad.sampleRate != vadSampleRate {
		vad.stream = beep.Resample(3, beep.SampleRate(vad.sampleRate), vadSampleRate, vad.stream)
	}

	return nil
}

func (vad *VAD) Detect() (float64, float64, float64, error) {
	endAt := vad.endAt
	totalAt := endAt + float64(len(vad.detectBuf))/vadSampleRate
	if vad.audio.Len() == 0 {
		return 0, endAt, totalAt, nil
	}
	if vad.stream == nil {
		return 0, 0, 0, errors.New("empty stream")
	}

	twoChannels := make([][2]float64, 3200)
	for {
		n, ok := vad.stream.Stream(twoChannels)
		if !ok {
			if err := vad.stream.Err(); err != nil {
				return 0, 0, 0, err
			}
		}
		for i := range n {
			vad.detectBuf = append(vad.detectBuf, float32(twoChannels[i][0]))
		}
		if n < len(twoChannels) {
			break
		}
	}

	endAt = vad.endAt
	totalAt = endAt + float64(len(vad.detectBuf))/vadSampleRate
	// 数据太短了
	if len(vad.detectBuf) <= 1600 {
		return 0, endAt, totalAt, nil
	}

	vad.detector.Reset()
	segments, err := vad.detector.Detect(vad.detectBuf)
	if err != nil {
		return 0, 0, 0, err
	}

	maxSilence := 0.0
	detectBuf := vad.detectBuf
	for i, s := range segments {
		silence := s.SpeechStartAt
		if i > 0 {
			silence = s.SpeechStartAt - segments[i-1].SpeechEndAt
		}
		maxSilence = max(maxSilence, silence)
		if s.SpeechEndAt > 0 {
			vad.endAt = endAt + s.SpeechEndAt
			vad.detectBuf = detectBuf[min(int(s.SpeechEndAt*vadSampleRate), len(detectBuf)):]
		}
	}
	// log.Debug("vad segments", segments)
	if len(segments) > 0 && segments[len(segments)-1].SpeechEndAt == 0 {
		return maxSilence, endAt, totalAt, nil
	}
	maxSilence = max(maxSilence, totalAt-vad.endAt)
	return maxSilence, endAt, totalAt, nil
}

func (vad *VAD) Close() {
	vad.detector.Destroy()
}

func (vad *VAD) Reset() {
	vad.audio.Reset()
	vad.stream = nil
	vad.endAt = 0
	vad.detectBuf = vad.detectBuf[:0]
	vad.detector.Reset()
}
