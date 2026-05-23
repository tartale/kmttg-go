package tivolibre

import (
    "bytes"
    "testing"
)

func TestTuringEncryptDecryptRoundtrip(t *testing.T) {
    // sample 20-byte key (like Java usage)
    key := make([]byte, 20)
    for i := 0; i < 20; i++ {
        key[i] = byte(i)
    }

    td := NewTuringDecoder(key)
    stream := td.PrepareFrame(1, 123456)

    // sample plaintext
    plain := []byte("The quick brown fox jumps over the lazy dog")
    enc := make([]byte, len(plain))
    copy(enc, plain)

    // encrypt in-place
    td.DecryptBytesOffset(stream, enc, 0, len(enc))

    // reset position to decrypt
    stream.SetCipherPos(0)
    td.DecryptBytesOffset(stream, enc, 0, len(enc))

    if !bytes.Equal(enc, plain) {
        t.Fatalf("roundtrip failed: got=%x want=%x", enc, plain)
    }
}
