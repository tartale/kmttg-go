package tivolibre

type TransportStreamInstance struct {
	*Stream
	TuringDecoder       *TuringDecoder
	lastPesHeaderOffset int
}

func NewTransportStreamInstance(decoder *TuringDecoder) *TransportStreamInstance {
	return &TransportStreamInstance{Stream: NewStream(), TuringDecoder: decoder}
}

func (ts *TransportStreamInstance) SetKey(key []byte) {
	ts.Stream.SetKey(key)
}

func (ts *TransportStreamInstance) PauseDecrypting()  {}
func (ts *TransportStreamInstance) ResumeDecrypting() {}

// ProcessPacket decrypts payload using the Turing decoder when appropriate and returns full packet bytes.
func (ts *TransportStreamInstance) ProcessPacket(p *TransportPacket) []byte {
	if !p.NeedsDecoding() {
		return p.GetBytes()
	}

	if ts.TuringDecoder == nil || !ts.Stream.DoHeader() {
		return p.GetBytes()
	}

	data := p.Raw[4:]
	turingStream := ts.TuringDecoder.PrepareFrameWithKey(ts.Stream.TuringKey, ts.Stream.StreamId, ts.Stream.TuringBlockNumber)
	ts.TuringDecoder.DecryptBytes(turingStream, data)
	p.ClearScrambled()

	return p.GetBytes()
}

func (ts *TransportStreamInstance) calculatePesHeaderOffset(data []byte) int {
	if len(data) < 6 {
		return 0
	}
	if pes, offset, err := ParsePesHeaderAt(data, 0); err == nil && pes != nil {
		return offset
	}
	return 0
}
