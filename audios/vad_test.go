package audios_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/guogeer/quasar/v2/audios"
)

func checkVAD(t *testing.T, path string, sampleRate int) {
	checkFullFileVAD(t, path, sampleRate)

	v1, err := audios.NewVAD("../../bin/silero_vad.onnx", sampleRate, filepath.Ext(path)[1:])
	if err != nil {
		t.Fatal(err)
	}
	defer v1.Close()

	f1, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()

	chunk := make([]byte, 4096)
	for {
		n, err := f1.Read(chunk)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := v1.Write(chunk[:n]); err != nil {
			t.Fatal(err)
		}
		maxSilence, endAt, totalAt, err := v1.Detect()
		t.Log("max silence", maxSilence, endAt, totalAt, err)
	}
}

func checkFullFileVAD(t *testing.T, path string, sampleRate int) {
	v1, err := audios.NewVAD("../../bin/silero_vad.onnx", sampleRate, filepath.Ext(path)[1:])
	if err != nil {
		t.Fatal(err)
	}
	defer v1.Close()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v1.Write(content)
	maxSilence, endAt, totalAt, err := v1.Detect()
	t.Log("full file pos", maxSilence, endAt, totalAt, err)
}

func TestVAD(t *testing.T) {
	// checkVAD(t, "silence1.wav", 16000)
	// checkVAD(t, "test_audio.wav", 16000)
	// checkVAD(t, "西班牙语.wav", 24000)
	checkVAD(t, "es_10S.mp3", 16000)
	checkVAD(t, "silence_0_2.wav", 16000)
}
