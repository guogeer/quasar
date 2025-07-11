package audios_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/guogeer/quasar/v2/audios"
)

func TestFormat(t *testing.T) {
	cmd := audios.NewCommand()
	cmd.SetPitch(-10)
	cmd.SetVolume(5)

	buf, err := os.ReadFile("testdata/test_audio.wav")
	if err != nil {
		t.Error("read file", err)
		return
	}

	out, err := cmd.Format(bytes.NewBuffer(buf))
	if err != nil {
		t.Error("format audio", err)
		return
	}
	os.WriteFile("test_audio.mp3", out, 0664)
}

func TestGetAudioInfo(t *testing.T) {
	for _, path := range []string{"testdata/C2S.wav", "testdata/C2S.opus", "testdata/C2S.webm"} {
		info, err := audios.GetAudioInfo(path)
		if err != nil {
			t.Error("count duration", err)
			return
		}
		audioBuf, _ := json.MarshalIndent(info, "", "  ")
		t.Logf("audio info: %s", audioBuf)
	}
}
