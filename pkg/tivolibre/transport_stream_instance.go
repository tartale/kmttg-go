package tivolibre

type TransportStreamInstance struct {
    *Stream
    TuringDecoder *TuringDecoder
}

func NewTransportStreamInstance(decoder *TuringDecoder) *TransportStreamInstance {
    return &TransportStreamInstance{Stream: NewStream(), TuringDecoder: decoder}
}

func (ts *TransportStreamInstance) SetKey(key []byte) {
    ts.Stream.SetKey(key)
}

func (ts *TransportStreamInstance) PauseDecrypting() {}
func (ts *TransportStreamInstance) ResumeDecrypting() {}

// ProcessPacket decrypts payload using turing decoder when appropriate and returns full packet bytes
func (ts *TransportStreamInstance) ProcessPacket(p *TransportPacket) []byte {
    data := p.GetData()
    if ts.TuringDecoder != nil && ts.Stream.DoHeader() {
        // get turing stream and decrypt whole data
        turingStream := ts.TuringDecoder.PrepareFrame(ts.Stream.StreamId, ts.Stream.TuringBlockNumber)
        // decrypt in place
        ts.TuringDecoder.DecryptBytes(turingStream, data)
        p.SetPayload(data)
    }
    return p.GetBytes()
}
