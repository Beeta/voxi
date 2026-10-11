//go:build darwin

package main

// Audio capture on macOS, in pure Go through ScreenCaptureKit (macOS 15+):
// one SCStream captures both the system's audio output and the microphone,
// delivering CMSampleBuffers of 48 kHz float PCM on a background queue,
// which this file turns into interleaved stereo frames for the recorder.
//
// No cgo: Objective-C is driven through the runtime with purego, and the
// CoreMedia C functions are called by address, like the rest of MyGo.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// SCStreamOutputType values (SCStreamOutputType).
const (
	scOutputScreen     = 0
	scOutputAudio      = 1 // the system's audio output
	scOutputMicrophone = 2 // the microphone (macOS 15+)
)

// AudioStreamBasicDescription mirrors the CoreAudio struct describing a
// sample buffer's format.
type AudioStreamBasicDescription struct {
	SampleRate       float64
	FormatID         uint32
	FormatFlags      uint32
	BytesPerPacket   uint32
	FramesPerPacket  uint32
	BytesPerFrame    uint32
	ChannelsPerFrame uint32
	BitsPerChannel   uint32
	Reserved         uint32
}

// Format flags of an ASBD (kAudioFormatFlag*).
const (
	formatFlagIsFloat          = 1 << 0
	formatFlagIsNonInterleaved = 1 << 5
)

// audioBuffer mirrors struct AudioBuffer.
type audioBuffer struct {
	NumberChannels uint32
	DataByteSize   uint32
	Data           unsafe.Pointer
}

// audioBufferList mirrors struct AudioBufferList with room for up to eight
// buffers (non-interleaved audio uses one per channel). The padding puts
// Buffers at offset 8, as C does after the 4-byte count.
type audioBufferList struct {
	NumberBuffers uint32
	_             uint32
	Buffers       [8]audioBuffer
}

// CMTime mirrors the CoreMedia time struct (24 bytes).
type CMTime struct {
	Value     int64
	Timescale int32
	Flags     uint32
	Epoch     int64
}

// Seconds converts a CMTime to seconds, 0 when it is invalid.
func (t CMTime) Seconds() float64 {
	if t.Timescale == 0 {
		return 0
	}
	return float64(t.Value) / float64(t.Timescale)
}

var (
	loadOnce sync.Once
	loadErr  error

	libObjC, libCM, libCF, libSCK uintptr
	msgSendAddr                   uintptr

	// CoreMedia C functions, called by address.
	cmGetFormatDesc  uintptr
	cmGetNumSamples  uintptr
	cmCopyABL        uintptr
	cmAudioFmtASBD   uintptr
	cfReleaseAddr    uintptr
	cmGetPTS         func(sbuf uintptr) CMTime
	dispatchNewQueue func(label, attr uintptr) uintptr

	outputClass id // the registered SCStreamOutput/SCStreamDelegate class
)

type id = objc.ID

// dbg collects a short trace of what the audio callback saw, written to a
// log file when a session ends, so the capture can be diagnosed without a
// debugger. Enabled when VOXI_DEBUG is set.
var dbg debugTrace

type debugTrace struct {
	mu      sync.Mutex
	enabled bool
	counts  map[string]int
	samples map[string]int
	seen    map[string]bool
}

func (d *debugTrace) reset() {
	d.mu.Lock()
	// Opt-in: set VOXI_DEBUG=1 to append the trace to ~/voxi-debug.log.
	// Off by default, so a released app does not write to the user's home
	// directory.
	d.enabled = os.Getenv("VOXI_DEBUG") != ""
	d.counts = map[string]int{}
	d.samples = map[string]int{}
	d.seen = map[string]bool{}
	d.mu.Unlock()
	d.write("=== session start ===")
}

// once logs a line only the first time a given key is seen in a session, so
// per-buffer facts (the sample format) are recorded without flooding.
func (d *debugTrace) once(key, line string) {
	d.mu.Lock()
	first := d.enabled && !d.seen[key]
	if first {
		d.seen[key] = true
	}
	d.mu.Unlock()
	if first {
		d.write(key + ": " + line)
	}
}

func (d *debugTrace) add(src Source, status string, n int) {
	d.mu.Lock()
	if d.enabled {
		key := string(src) + "/" + status
		d.counts[key]++
		d.samples[key] += n
	}
	d.mu.Unlock()
}

func (d *debugTrace) note(format string, args ...any) {
	d.mu.Lock()
	on := d.enabled
	d.mu.Unlock()
	if on {
		d.write(fmt.Sprintf(format, args...))
	}
}

func (d *debugTrace) write(line string) {
	home, _ := os.UserHomeDir()
	f, err := os.OpenFile(filepath.Join(home, "voxi-debug.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05"), line)
}

func (d *debugTrace) dump() {
	d.mu.Lock()
	on := d.enabled
	var b strings.Builder
	for k, c := range d.counts {
		fmt.Fprintf(&b, "%s: %d buffers, %d samples; ", k, c, d.samples[k])
	}
	d.mu.Unlock()
	if on {
		d.write("callback summary: " + b.String())
	}
}

// load opens the frameworks and prepares the message and C functions once.
func load() error {
	loadOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				loadErr = fmt.Errorf("%v", r)
			}
		}()
		var err error
		open := func(path string) uintptr {
			h, e := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_NOW)
			if e != nil && err == nil {
				err = fmt.Errorf("cannot load %s: %w", path, e)
			}
			return h
		}
		libObjC = open("/usr/lib/libobjc.A.dylib")
		open("/System/Library/Frameworks/Foundation.framework/Foundation")
		libCF = open("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		libCM = open("/System/Library/Frameworks/CoreMedia.framework/CoreMedia")
		open("/System/Library/Frameworks/AVFoundation.framework/AVFoundation")
		libSCK = open("/System/Library/Frameworks/ScreenCaptureKit.framework/ScreenCaptureKit")
		if err != nil {
			loadErr = err
			return
		}
		msgSendAddr = mustSym(libObjC, "objc_msgSend")
		cmGetFormatDesc = mustSym(libCM, "CMSampleBufferGetFormatDescription")
		cmGetNumSamples = mustSym(libCM, "CMSampleBufferGetNumSamples")
		cmCopyABL = mustSym(libCM, "CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer")
		cmAudioFmtASBD = mustSym(libCM, "CMAudioFormatDescriptionGetStreamBasicDescription")
		cfReleaseAddr = mustSym(libCF, "CFRelease")
		purego.RegisterFunc(&cmGetPTS, mustSym(libCM, "CMSampleBufferGetPresentationTimeStamp"))
		purego.RegisterFunc(&dispatchNewQueue, mustSym(purego.RTLD_DEFAULT, "dispatch_queue_create"))
		registerOutputClass()
	})
	return loadErr
}

func mustSym(lib uintptr, name string) uintptr {
	p, err := purego.Dlsym(lib, name)
	if err != nil {
		panic(fmt.Sprintf("voxi: %s: %v", name, err))
	}
	return p
}

// cCall calls a C function by address with integer/pointer arguments. The
// go:uintptrescapes directive keeps uintptr(unsafe.Pointer(&x)) arguments
// alive across the call, as the darwin backend does for objc_msgSend.
//
//go:uintptrescapes
func cCall(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}

//go:uintptrescapes
func send(obj id, selector string, args ...uintptr) id {
	var a [10]uintptr
	a[0], a[1] = uintptr(obj), uintptr(objc.RegisterName(selector))
	n := copy(a[2:], args)
	r, _, _ := purego.SyscallN(msgSendAddr, a[:n+2]...)
	return id(r)
}

func sendBool(obj id, selector string, args ...uintptr) bool {
	return byte(send(obj, selector, args...)) != 0
}

func class(name string) id { return id(objc.GetClass(name)) }

func nsString(s string) id {
	b := unsafe.StringData(s)
	str := send(id(objc.GetClass("NSString")), "alloc")
	str = sendBytes(str, b, len(s))
	return str
}

//go:uintptrescapes
func sendBytes(str id, b *byte, n int) id {
	const utf8 = 4
	a := []uintptr{uintptr(str), uintptr(objc.RegisterName("initWithBytes:length:encoding:")),
		uintptr(unsafe.Pointer(b)), uintptr(n), utf8}
	r, _, _ := purego.SyscallN(msgSendAddr, a...)
	return id(r)
}

// registerOutputClass defines the Objective-C class that receives sample
// buffers and stop notifications. Its methods run on the stream's own
// background queue; they route to the active capture by a package global,
// since the app records one session at a time.
func registerOutputClass() {
	cls, err := objc.RegisterClass("VoxiAudioOutput", objc.GetClass("NSObject"),
		protocols("SCStreamOutput", "SCStreamDelegate"), nil, []objc.MethodDef{
			{Cmd: objc.RegisterName("stream:didOutputSampleBuffer:ofType:"),
				Fn: func(self id, _ objc.SEL, stream, sbuf id, typ int) { onSample(sbuf, typ) }},
			{Cmd: objc.RegisterName("stream:didStopWithError:"),
				Fn: func(self id, _ objc.SEL, stream, nsErr id) { onStop(nsErr) }},
		})
	if err != nil {
		panic(err)
	}
	outputClass = id(cls)
}

func protocols(names ...string) []*objc.Protocol {
	var ps []*objc.Protocol
	for _, n := range names {
		if p := objc.GetProtocol(n); p != nil {
			ps = append(ps, p)
		}
	}
	return ps
}

// darwinCapture is the live ScreenCaptureKit session.
type darwinCapture struct {
	stream  id
	output  id
	queue   uintptr
	deliver func(Source, chunk)
}

var (
	captureMu sync.Mutex
	active    *darwinCapture
)

func newCapture() capture { return &darwinCapture{} }

func (c *darwinCapture) available() (bool, string) {
	if err := load(); err != nil {
		return false, "无法加载系统框架：" + err.Error()
	}
	if class("SCStream") == 0 {
		return false, "系统不支持 ScreenCaptureKit，需要 macOS 13 及以上。"
	}
	// Microphone capture in SCStream is a macOS 15 addition.
	cfg := class("SCStreamConfiguration")
	if cfg == 0 || !sendBool(cfg, "instancesRespondToSelector:", uintptr(objc.RegisterName("setCaptureMicrophone:"))) {
		return false, "麦克风混录需要 macOS 15 及以上。"
	}
	return true, ""
}

// onSample converts one delivered buffer and hands it to the recorder. It
// runs on the stream's background queue.
func onSample(sbuf id, typ int) {
	captureMu.Lock()
	c := active
	captureMu.Unlock()
	if c == nil || sbuf == 0 {
		return
	}
	var src Source
	switch typ {
	case scOutputAudio:
		src = SourceSystem
	case scOutputMicrophone:
		src = SourceMic
	default:
		return // a screen frame we do not consume
	}
	samples, reason := decodeSamples(sbuf)
	if samples == nil {
		dbg.add(src, "fail:"+reason, 0)
		return
	}
	dbg.add(src, "ok", len(samples))
	c.deliver(src, chunk{samples: samples})
}

func onStop(nsErr id) {
	// The stream stopped on its own (an error, or the user revoked
	// permission). Nothing to do here: the recorder keeps what it has and
	// writes it when the user clicks stop.
	_ = nsErr
}

// decodeSamples extracts interleaved stereo float32 frames from a sample
// buffer, duplicating a mono source to both channels. On failure it returns
// a nil slice and a short reason for the diagnostic log.
func decodeSamples(sbuf id) ([]float32, string) {
	fd := cCall(cmGetFormatDesc, uintptr(sbuf))
	if fd == 0 {
		return nil, "no-formatdesc"
	}
	asbdPtr := cCall(cmAudioFmtASBD, fd)
	if asbdPtr == 0 {
		return nil, "no-asbd"
	}
	asbd := *(**AudioStreamBasicDescription)(unsafe.Pointer(&asbdPtr))
	float := asbd.FormatFlags&formatFlagIsFloat != 0
	frames := int(cCall(cmGetNumSamples, uintptr(sbuf)))
	dbg.once("asbd", fmt.Sprintf("rate=%g ch=%d bits=%d flags=%#x float=%v frames=%d",
		asbd.SampleRate, asbd.ChannelsPerFrame, asbd.BitsPerChannel, asbd.FormatFlags, float, frames))
	if !float || asbd.BitsPerChannel != 32 {
		return nil, fmt.Sprintf("fmt bits=%d float=%v", asbd.BitsPerChannel, float)
	}
	if frames <= 0 {
		return nil, "no-frames"
	}

	// Fill an AudioBufferList the documented way: ask how many bytes it
	// needs, allocate exactly that, then let CoreMedia populate it and
	// hand back a retained block buffer that backs the data.
	var needed uintptr
	st1 := int32(cCall(cmCopyABL, uintptr(sbuf),
		uintptr(unsafe.Pointer(&needed)), 0, 0,
		0, 0, 0, 0))
	dbg.once("ablsize", fmt.Sprintf("sizeQuery status=%d needed=%d", st1, needed))
	if needed == 0 {
		needed = unsafe.Sizeof(audioBufferList{})
	}
	backing := make([]byte, needed)
	ablPtr := unsafe.Pointer(&backing[0])
	var blockBuf uintptr
	status := int32(cCall(cmCopyABL, uintptr(sbuf),
		0, uintptr(ablPtr), needed,
		0, 0, 0, uintptr(unsafe.Pointer(&blockBuf))))
	if status != 0 {
		return nil, fmt.Sprintf("copyABL status=%d (needed=%d)", status, needed)
	}
	abl := (*audioBufferList)(ablPtr)
	// The retained block buffer backs abl's data; release it after copying.
	defer func() {
		if blockBuf != 0 {
			cCall(cfReleaseAddr, blockBuf)
		}
	}()

	out := make([]float32, frames*channels)
	nb := int(abl.NumberBuffers)
	dbg.once("abl", fmt.Sprintf("buffers=%d buf0.ch=%d buf0.bytes=%d",
		nb, abl.Buffers[0].NumberChannels, abl.Buffers[0].DataByteSize))
	if nb == 0 {
		return nil, "zero-buffers"
	}
	if asbd.FormatFlags&formatFlagIsNonInterleaved != 0 {
		// One mono buffer per channel.
		left := bufferFloats(abl.Buffers[0], frames)
		right := left
		if nb > 1 {
			right = bufferFloats(abl.Buffers[1], frames)
		}
		for i := 0; i < frames; i++ {
			out[i*2] = left[i]
			out[i*2+1] = right[i]
		}
		return out, "ok"
	}
	// Interleaved: one buffer of frames*ch samples.
	ch := int(asbd.ChannelsPerFrame)
	if ch == 0 {
		ch = 1
	}
	data := bufferFloats(abl.Buffers[0], frames*ch)
	for i := 0; i < frames; i++ {
		l := data[i*ch]
		r := l
		if ch > 1 {
			r = data[i*ch+1]
		}
		out[i*2] = l
		out[i*2+1] = r
	}
	return out, "ok"
}

// bufferFloats views an AudioBuffer's data as n float32 samples, copied so
// the slice outlives the block buffer.
func bufferFloats(b audioBuffer, n int) []float32 {
	avail := int(b.DataByteSize) / 4
	if avail < n {
		n = avail
	}
	if n <= 0 || b.Data == nil {
		return make([]float32, n)
	}
	src := unsafe.Slice((*float32)(b.Data), n)
	return append([]float32(nil), src...)
}

func (c *darwinCapture) start(sources []Source, deliver func(Source, chunk)) error {
	if err := load(); err != nil {
		return err
	}
	c.deliver = deliver
	dbg.reset()
	dbg.note("start sources=%v", sources)

	wantMic, wantSystem := false, false
	for _, s := range sources {
		switch s {
		case SourceMic:
			wantMic = true
		case SourceSystem:
			wantSystem = true
		}
	}

	filter, err := shareableFilter()
	if err != nil {
		return err
	}

	cfg := send(send(class("SCStreamConfiguration"), "alloc"), "init")
	// Audio-only: capture a tiny screen we never read, to keep overhead low.
	send(cfg, "setWidth:", 2)
	send(cfg, "setHeight:", 2)
	send(cfg, "setCapturesAudio:", boolArg(wantSystem))
	send(cfg, "setSampleRate:", uintptr(sampleRate))
	send(cfg, "setChannelCount:", uintptr(channels))
	send(cfg, "setExcludesCurrentProcessAudio:", 1)
	if wantMic {
		send(cfg, "setCaptureMicrophone:", 1)
	}

	c.output = send(outputClass, "alloc")
	c.output = send(c.output, "init")
	c.stream = send(send(class("SCStream"), "alloc"),
		"initWithFilter:configuration:delegate:", uintptr(filter), uintptr(cfg), uintptr(c.output))
	if c.stream == 0 {
		return errors.New("无法创建捕获流")
	}

	c.queue = dispatchNewQueue(0, 0)
	add := func(typ int) error {
		var nsErr id
		ok := addOutput(c.stream, c.output, typ, c.queue, &nsErr)
		if !ok {
			dbg.note("addStreamOutput type=%d failed: %s", typ, nsErrorString(nsErr))
			return fmt.Errorf("添加音频输出失败：%s", nsErrorString(nsErr))
		}
		dbg.note("addStreamOutput type=%d ok", typ)
		return nil
	}
	if wantSystem {
		if err := add(scOutputAudio); err != nil {
			return err
		}
	}
	if wantMic {
		if err := add(scOutputMicrophone); err != nil {
			return err
		}
	}

	// Make this the active capture before frames can arrive.
	captureMu.Lock()
	active = c
	captureMu.Unlock()

	if err := c.startCapture(); err != nil {
		captureMu.Lock()
		active = nil
		captureMu.Unlock()
		return err
	}
	return nil
}

// addOutput calls addStreamOutput:type:sampleHandlerQueue:error:.
//
//go:uintptrescapes
func addOutput(stream, output id, typ int, queue uintptr, nsErr *id) bool {
	a := []uintptr{uintptr(stream), uintptr(objc.RegisterName("addStreamOutput:type:sampleHandlerQueue:error:")),
		uintptr(output), uintptr(typ), queue, uintptr(unsafe.Pointer(nsErr))}
	r, _, _ := purego.SyscallN(msgSendAddr, a...)
	return byte(r) != 0
}

// shareableFilter asks for the shareable content and builds a filter over
// the first display. It triggers the screen-recording permission prompt the
// first time, and errors when it is denied.
func shareableFilter() (id, error) {
	type result struct {
		content id
		err     error
	}
	done := make(chan result, 1)
	blk := objc.NewBlock(func(_ objc.Block, content id, nsErr id) {
		if nsErr != 0 {
			done <- result{err: errors.New(nsErrorString(nsErr))}
			return
		}
		done <- result{content: send(content, "retain")}
	})
	defer blk.Release()
	send(class("SCShareableContent"), "getShareableContentWithCompletionHandler:", uintptr(blk))

	var res result
	select {
	case res = <-done:
	case <-time.After(20 * time.Second):
		return 0, errors.New("获取屏幕内容超时，请在系统设置中授予“屏幕录制”权限")
	}
	if res.err != nil {
		return 0, errors.New("无法访问屏幕内容（需要“屏幕录制”权限）：" + res.err.Error())
	}
	displays := send(res.content, "displays")
	if displays == 0 || send(displays, "count") == 0 {
		return 0, errors.New("找不到可用的显示器")
	}
	display := send(displays, "objectAtIndex:", 0)
	empty := send(class("NSArray"), "array")
	filter := send(send(class("SCContentFilter"), "alloc"),
		"initWithDisplay:excludingWindows:", uintptr(display), uintptr(empty))
	if filter == 0 {
		return 0, errors.New("无法创建内容过滤器")
	}
	return filter, nil
}

// startCapture starts the stream and waits for its completion handler.
func (c *darwinCapture) startCapture() error {
	done := make(chan error, 1)
	blk := objc.NewBlock(func(_ objc.Block, nsErr id) {
		if nsErr != 0 {
			done <- errors.New(nsErrorString(nsErr))
			return
		}
		done <- nil
	})
	defer blk.Release()
	send(c.stream, "startCaptureWithCompletionHandler:", uintptr(blk))
	select {
	case err := <-done:
		if err != nil {
			return errors.New("启动捕获失败：" + err.Error())
		}
		return nil
	case <-time.After(20 * time.Second):
		return errors.New("启动捕获超时")
	}
}

func (c *darwinCapture) stop() {
	captureMu.Lock()
	if active == c {
		active = nil
	}
	captureMu.Unlock()
	if c.stream == 0 {
		return
	}
	done := make(chan struct{}, 1)
	blk := objc.NewBlock(func(_ objc.Block, nsErr id) { done <- struct{}{} })
	defer blk.Release()
	send(c.stream, "stopCaptureWithCompletionHandler:", uintptr(blk))
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
	dbg.dump()
	c.stream = 0
}

func boolArg(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// nsErrorString returns an NSError's localized description.
func nsErrorString(err id) string {
	if err == 0 {
		return "未知错误"
	}
	desc := send(err, "localizedDescription")
	if desc == 0 {
		return "未知错误"
	}
	p := send(desc, "UTF8String")
	if p == 0 {
		return "未知错误"
	}
	return goStringPtr(unsafe.Pointer(&p))
}

// goStringPtr copies a NUL-terminated C string. pp points at the pointer
// value returned from Objective-C, which keeps go vet from flagging a bare
// uintptr→Pointer conversion.
func goStringPtr(pp unsafe.Pointer) string {
	p := *(*unsafe.Pointer)(pp)
	if p == nil {
		return ""
	}
	var n int
	for *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(p), n))
}
