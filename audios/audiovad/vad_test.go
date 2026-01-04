package audiovad_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/guogeer/quasar/v2/audios/audiovad"
)

func checkVAD(t *testing.T, v1 *audiovad.VAD, path string, sampleRate int) {
	t.Logf("vad path %s, sample rate %d format %s", path, sampleRate, filepath.Ext(path)[1:])

	v1.Reset()
	v1.SetSampleRate(sampleRate)
	v1.SetAudioFormat(filepath.Ext(path)[1:])
	checkFullFileVAD(t, v1, path)

	v1.Reset()
	f1, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()

	chunk := make([]byte, 3200)
	for {
		n, err := f1.Read(chunk)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := v1.Write(chunk[:n]); err != nil {
			t.Error(err)
			return
		}
		maxSilence, endAt, totalAt, err := v1.Detect()
		t.Log("max silence", maxSilence, endAt, totalAt, err)
	}
}

func checkFullFileVAD(t *testing.T, v1 *audiovad.VAD, path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v1.Write(content)
	maxSilence, endAt, totalAt, err := v1.Detect()
	t.Log("full file pos", maxSilence, endAt, totalAt, err)
}

func TestVAD(t *testing.T) {
	v1, err := audiovad.NewVAD("../testdata/silero_vad.onnx", 16000, "wav")
	if err != nil {
		t.Fatal(err)
	}

	checkVAD(t, v1, "../testdata/silence1.wav", 16000)
	checkVAD(t, v1, "../testdata/test_audio.wav", 16000)
	checkVAD(t, v1, "../testdata/西班牙语.wav", 24000)
	checkVAD(t, v1, "../testdata/C6S.pcm", 16000)
	checkVAD(t, v1, "../testdata/C6S_24kHz.pcm", 24000)
}
