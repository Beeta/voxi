package audio

// NewSimulatedRecorder is a Recorder over no device and no file: capture
// starts at once and delivers nothing, and the encoder discards what it is
// given. It is for tests of the code driving a Recorder — the record
// control, pause and stop — on machines without a microphone or the
// screen-recording permission.
func NewSimulatedRecorder() *Recorder {
	return &Recorder{
		state:      StateIdle,
		platform:   silentCapture{},
		newEncoder: func(string, int) (encoder, error) { return discardEncoder{}, nil },
	}
}

type silentCapture struct{}

func (silentCapture) available() (bool, string)                 { return true, "" }
func (silentCapture) start([]Source, func(Source, chunk)) error { return nil }
func (silentCapture) stop()                                     {}

type discardEncoder struct{}

func (discardEncoder) write([]float32) error { return nil }
func (discardEncoder) close() error          { return nil }
