package tivolibre

import (
    "io"
)

type TransportStreamDecoder struct {
    turingDecoder *TuringDecoder
    mpegOffset int
    r *CountingReader
    w io.Writer
    streams map[int]*TransportStreamInstance
}

func NewTransportStreamDecoder(td *TuringDecoder, mpegOffset int, r io.Reader, w io.Writer) *TransportStreamDecoder {
    return &TransportStreamDecoder{turingDecoder: td, mpegOffset: mpegOffset, r: NewCountingReader(r), w: w, streams: make(map[int]*TransportStreamInstance)}
}

func (d *TransportStreamDecoder) Process() error {
    // advance to mpegOffset
    if d.mpegOffset > 0 {
        _, _ = d.r.SkipBytes(d.mpegOffset)
    }
    buf := make([]byte, TSFrameSize)
    for {
        _, err := d.r.Read(buf)
        if err != nil {
            return nil
        }
        packet, err := ParseTransportPacket(buf)
        if err != nil {
            // skip
            continue
        }
        // check private data 'TiVo' signature in payload
        data := packet.GetData()
        if len(data) >= 8 && string(data[0:4]) == "TiVo" {
            // parse entries: validator(2), skip 3, streamLength
            // simple parser to set keys
            if len(data) > 10 {
                // find stream entries
                // streamLength at offset 7
                streamLen := int(data[7])
                idx := 8
                for streamLen > 0 && idx+2 < len(data) {
                    packetId := int(data[idx])<<8 | int(data[idx+1])
                    idx += 2
                    if idx >= len(data) { break }
                    streamId := int(data[idx])
                    idx++
                    idx++ // reserved
                    if idx+16 <= len(data) {
                        key := data[idx:idx+16]
                        idx += 16
                        stream := d.streams[packetId]
                        if stream == nil {
                            stream = NewTransportStreamInstance(d.turingDecoder)
                            d.streams[packetId] = stream
                        }
                        stream.SetStreamId(streamId)
                        stream.SetKey(key)
                    } else {
                        break
                    }
                    streamLen -= 2 + 1 + 1 + 16
                }
            }
        }
        // route packet to stream (decrypt if key present)
        stream := d.streams[packet.GetPID()]
        if stream == nil {
            // create placeholder
            stream = NewTransportStreamInstance(d.turingDecoder)
            d.streams[packet.GetPID()] = stream
        }
        out := stream.ProcessPacket(packet)
        d.w.Write(out)
    }
}
