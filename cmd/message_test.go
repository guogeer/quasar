package cmd_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/guogeer/quasar/v2/cmd"
)

func TestEncode(t *testing.T) {
	key := "helloworld!"
	tempSign := "12345678"
	ref := []int{0, 3, 4, 8, 10, 11, 13, 14}

	pkg := &cmd.Package{}
	buf, err := cmd.Encode("test", map[string]any{"s": "hello", "n": 1})
	if err != nil {
		t.Fatalf("cmd.Encode Fail %v", err)
	}
	if err := json.Unmarshal(buf, pkg); err != nil {
		t.Fatalf("cmd.Encode json.Unmarshal Fail %v", err)
	}
	startKey := `"sign":"`
	startIndex := bytes.Index(buf, []byte(startKey))
	if startIndex < 0 {
		t.Fatal("cmd.Encode bytes.Index sign fail")
	}
	startIndex += len(startKey)
	copy(buf[startIndex:], []byte(tempSign))

	md5Buf := make([]byte, len(key)+len(buf))
	copy(md5Buf, key)
	copy(md5Buf[len(key):], buf)

	md5Sum := md5.Sum(md5Buf)
	sign := hex.EncodeToString(md5Sum[:])

	var shortSign []byte
	for _, v := range ref {
		shortSign = append(shortSign, sign[v])
	}
	if pkg.Sign != string(shortSign) {
		t.Fatalf("cmd.Encode Signature Fail %s != %s", pkg.Sign, shortSign)
	}
}
