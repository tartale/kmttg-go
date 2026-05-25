package tivolibre

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDebugParseTiVoPrivateData(t *testing.T) {
	mediaAccessKey := os.Getenv("KMTTG_MEDIA_ACCESS_KEY")
	testDataDir := os.Getenv("KMTTG_TEST_DATA_DIR")
	if mediaAccessKey == "" || testDataDir == "" {
		t.Skip("Skipping test: requires KMTTG_MEDIA_ACCESS_KEY and KMTTG_TEST_DATA_DIR environment variables to be set")
	}
	file, err := os.Open(filepath.Join(testDataDir, "encrypted.TiVo"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	ts := NewTivoStream(file, mediaAccessKey)
	_, err = ts.ProcessMetadata()
	if err != nil {
		t.Fatal(err)
	}
	header := ts.GetHeader()
	if header == nil {
		t.Fatal("no header")
	}
	if header.Format() != FormatTransport {
		t.Fatalf("unexpected format %d", header.Format())
	}
	metadataKey := ts.GetChunks()[0].GetMetadataKey(ts.mak)
	if metadataKey == nil {
		t.Fatal("no metadata key")
	}
	td := NewTuringDecoder(metadataKey)
	out := &bytes.Buffer{}
	decoder := NewTransportStreamDecoder(td, header.MpegOffset(), ts.GetCountingReader(), out)
	if err := decoder.Process(); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("decoded bytes %d\n", out.Len())
	for pid, stream := range decoder.streams {
		if stream == nil {
			continue
		}
		if stream.Stream == nil {
			continue
		}
		if stream.Stream.DoHeader() {
			fmt.Printf("pid=%03d streamId=%02x block=%d crypted=%d key=%x\n", pid, stream.Stream.StreamId, stream.Stream.TuringBlockNumber, stream.Stream.TuringCrypted, stream.Stream.TuringKey)
		}
	}
}
