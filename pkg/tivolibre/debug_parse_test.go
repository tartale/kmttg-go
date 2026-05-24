package tivolibre

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestDebugParseTiVoPrivateData(t *testing.T) {
	file, err := os.Open("../../test/data/Odd Squad - Odd Way Round Strictly Odd Dancing (05_22_2026).TiVo")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	ts := NewTivoStream(file, os.Getenv("KMTTG_MEDIA_ACCESS_KEY"))
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
