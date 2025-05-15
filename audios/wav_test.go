package audios_test

import (
	"os"
	"testing"

	"github.com/guogeer/quasar/v2/audios"
)

func TestCreateWavHeader(t *testing.T) {
	body, err := os.ReadFile("testdata/input.pcm")
	if err != nil {
		t.Fatal(err)
	}
	header := audios.CreateWavHeader(16000, 16, 1)
	wav := append(header, body...)
	os.WriteFile("temp_output.wav", wav, 0644)
}
