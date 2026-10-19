package asr

import (
	"sync"
	"time"
)

// Step is one stage of a transcription, in the order they run.
type Step int

// The steps of Transcribe.
const (
	StepServe  Step = iota // serve the file locally
	StepTunnel             // start the cloudflared quick tunnel
	StepProbe              // wait for the tunnel to serve the file
	StepSubmit             // submit the task to 豆包语音
	StepWait               // poll until the result is ready
	StepParse              // turn the result into segments
	stepCount
)

// StepNames are what the view calls the steps.
var StepNames = [stepCount]string{
	"本地文件服务",
	"建立临时隧道",
	"隧道可达",
	"提交到豆包语音",
	"等待识别结果",
	"解析结果",
}

// StepState is one step's part of a Progress.
type StepState struct {
	Started, Ended time.Time // zero until the step starts / ends
	Detail         string    // what the step found or is doing
	Err            string    // set when the step failed
}

// Progress is a snapshot of where a transcription is: each step's times and
// detail, the task id once submitted, and how much of the audio the service
// has fetched. A step is under way while it has Started and no Ended.
type Progress struct {
	Started time.Time
	Steps   [stepCount]StepState
	TaskID  string
	// Served and Size are the audio bytes the service has fetched through
	// the tunnel (the reachability probes not counted), and the file size.
	Served, Size int64
	// Queued is true while the service reports the task waiting in its
	// queue, false once it is processing it; Polls counts the queries.
	Queued bool
	Polls  int
}

// Current is the step under way, or the one that failed; -1 when none has
// started, and stepCount once every step is done.
func (p Progress) Current() Step {
	for i, s := range p.Steps {
		if s.Err != "" || (!s.Started.IsZero() && s.Ended.IsZero()) {
			return Step(i)
		}
		if s.Started.IsZero() {
			if i == 0 {
				return -1
			}
			return Step(i)
		}
	}
	return stepCount
}

// tracker builds a Progress as a transcription runs, handing a copy to
// report after every change. It is safe for concurrent use: the file
// server counts bytes on its own goroutines.
type tracker struct {
	mu     sync.Mutex
	p      Progress
	report func(Progress)
	now    func() time.Time
}

func newTracker(report func(Progress)) *tracker {
	if report == nil {
		report = func(Progress) {}
	}
	t := &tracker{report: report, now: time.Now}
	t.p.Started = t.now()
	return t
}

// update changes the progress under the lock and reports a copy.
func (t *tracker) update(fn func(p *Progress)) {
	t.mu.Lock()
	fn(&t.p)
	snap := t.p
	t.mu.Unlock()
	t.report(snap)
}

// begin starts step s.
func (t *tracker) begin(s Step) {
	t.update(func(p *Progress) { p.Steps[s].Started = t.now() })
}

// detail sets step s's detail.
func (t *tracker) detail(s Step, d string) {
	t.update(func(p *Progress) { p.Steps[s].Detail = d })
}

// end finishes step s, failed when err is not nil, and returns err.
func (t *tracker) end(s Step, err error) error {
	t.update(func(p *Progress) {
		p.Steps[s].Ended = t.now()
		if err != nil {
			p.Steps[s].Err = err.Error()
		}
	})
	return err
}

// snapshot is the progress as it stands.
func (t *tracker) snapshot() Progress {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.p
}
