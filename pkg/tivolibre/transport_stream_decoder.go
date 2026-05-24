package tivolibre

import (
	"io"
)

type TransportStreamDecoder struct {
	turingDecoder *TuringDecoder
	mpegOffset    int
	r             *CountingReader
	w             io.Writer
	streams       map[int]*TransportStreamInstance
}

func NewTransportStreamDecoder(td *TuringDecoder, mpegOffset int, r io.Reader, w io.Writer) *TransportStreamDecoder {
	cr, ok := r.(*CountingReader)
	if !ok {
		cr = NewCountingReader(r)
	}
	return &TransportStreamDecoder{turingDecoder: td, mpegOffset: mpegOffset, r: cr, w: w, streams: make(map[int]*TransportStreamInstance)}
}

func (d *TransportStreamDecoder) Process() error {
	// advance to the absolute mpegOffset from the start of the TiVo file,
	// taking into account that the wrapped reader may already have consumed
	// metadata before this decoder is invoked.
	if d.mpegOffset > 0 {
		current := int(d.r.Position())
		if current < d.mpegOffset {
			if _, err := d.r.SkipBytes(d.mpegOffset - current); err != nil {
				return err
			}
		}
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
			if len(data) > 11 {
				// find stream entries
				// streamLength at offset 9, entries start at offset 10
				streamLen := int(data[9])
				idx := 10
				for streamLen > 0 && idx+2 < len(data) {
					packetId := int(data[idx])<<8 | int(data[idx+1])
					idx += 2
					if idx >= len(data) {
						break
					}
					streamId := int(data[idx])
					idx++
					idx++ // reserved
					if idx+16 <= len(data) {
						key := data[idx : idx+16]
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
