package tivolibre

import (
    "io"
    "bytes"
)

// ProgramStreamDecoder parses a program stream in memory and decrypts PES payloads
// when a matching stream key is set on the per-stream `Stream` entry.
type ProgramStreamDecoder struct {
    td *TuringDecoder
    streams map[byte]*Stream
}

func NewProgramStreamDecoder(td *TuringDecoder) *ProgramStreamDecoder {
    return &ProgramStreamDecoder{td: td, streams: make(map[byte]*Stream)}
}

// Process reads the entire program stream from `r`, parses PES packets,
// decrypts payloads when appropriate, and writes the resulting stream to `w`.
// This implementation is memory-backed and intended as a faithful, simple
// port of the Java ProgramStreamDecoder for core functionality.
func (d *ProgramStreamDecoder) Process(r io.Reader, w io.Writer) error {
    buf, err := io.ReadAll(r)
    if err != nil { return err }

    i := 0
    n := len(buf)
    for i < n {
        // find next PES start code 0x000001
        idx := bytes.Index(buf[i:], []byte{0x00,0x00,0x01})
        if idx < 0 { break }
        idx += i
        // ensure there is at least stream id and length
        if idx+6 > n { break }
        // parse PES header
        pes, payloadStart, perr := ParsePesHeaderAt(buf, idx)
        if perr != nil {
            // advance past this marker and continue
            i = idx + 3
            continue
        }

        // find next PES start to determine payload end; if none, use file end
        next := bytes.Index(buf[payloadStart:], []byte{0x00,0x00,0x01})
        payloadEnd := n
        if next >= 0 { payloadEnd = payloadStart + next }

        payload := make([]byte, payloadEnd-payloadStart)
        copy(payload, buf[payloadStart:payloadEnd])

        // decrypt payload if we have a key for this stream
        s := d.streams[pes.StreamId]
        if s != nil && d.td != nil && s.DoHeader() {
            turing := d.td.PrepareFrame(s.StreamId, s.TuringBlockNumber)
            d.td.DecryptBytes(turing, payload)
            // write header (from idx to payloadStart) + decrypted payload
            w.Write(buf[idx:payloadStart])
            w.Write(payload)
        } else {
            // write original bytes
            w.Write(buf[idx:payloadEnd])
        }

        // advance
        i = payloadEnd
    }
    return nil
}

// RegisterStream registers a stream id with an associated `Stream` object so
// that keys and turing state can be configured externally prior to decoding.
func (d *ProgramStreamDecoder) RegisterStream(streamId byte, s *Stream) {
    d.streams[streamId] = s
}
