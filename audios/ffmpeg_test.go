package audios_test

import (
	"bytes"
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
