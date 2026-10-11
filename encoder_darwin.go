//go:build darwin

package main

// AAC encoding on macOS through ExtAudioFile (AudioToolbox), in pure Go via
// purego: one API both encodes AAC and writes the .m4a container, and
// ExtAudioFileWrite can be called repeatedly, so audio is encoded as it is
// captured instead of buffered to the end. It reuses the Objective-C and
// C-call machinery (cCall, send, class, nsString, mustSym, dbg and the ASBD
// and AudioBufferList structs) from capture_darwin.go, which is the same
// package.

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// fourCC packs a four-character code the way CoreAudio constants are defined.
func fourCC(s string) uint32 {
	return uint32(s[0])<<24 | uint32(s[1])<<16 | uint32(s[2])<<8 | uint32(s[3])
}

// CoreAudio identifiers used to configure the encoder (four-character codes).
var (
	kAudioFileM4AType                      = fourCC("m4af")
	kAudioFormatMPEG4AAC                   = fourCC("aac ")
	kAudioFormatLinearPCM                  = fourCC("lpcm")
	kExtAudioFileProperty_ClientDataFormat = fourCC("cfmt")
	kExtAudioFileProperty_AudioConverter   = fourCC("acnv")
	kExtAudioFileProperty_ConverterConfig  = fourCC("accf")
	kAudioConverterEncodeBitRate           = fourCC("brat")
)

const (
	kAudioFileFlags_EraseFile = 1
	kAudioFormatFlagIsFloat   = 1 << 0
	kAudioFormatFlagIsPacked  = 1 << 3
)

var (
	loadEncOnce sync.Once
	loadEncErr  error

	extCreate, extSetProp, extGetProp, extWrite, extDispose uintptr
	acSetProp                                               uintptr
)

// loadEncoder opens AudioToolbox and resolves the ExtAudioFile functions once.
func loadEncoder() error {
	loadEncOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				loadEncErr = fmt.Errorf("%v", r)
			}
		}()
		h, err := purego.Dlopen("/System/Library/Frameworks/AudioToolbox.framework/AudioToolbox",
			purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if err != nil {
			loadEncErr = fmt.Errorf("cannot load AudioToolbox: %w", err)
			return
		}
		extCreate = mustSym(h, "ExtAudioFileCreateWithURL")
		extSetProp = mustSym(h, "ExtAudioFileSetProperty")
		extGetProp = mustSym(h, "ExtAudioFileGetProperty")
		extWrite = mustSym(h, "ExtAudioFileWrite")
		extDispose = mustSym(h, "ExtAudioFileDispose")
		acSetProp = mustSym(h, "AudioConverterSetProperty")
	})
	return loadEncErr
}

// aacEncoder streams interleaved stereo float32 frames to an AAC .m4a file.
type aacEncoder struct {
	ext uintptr
}

// newAACEncoder opens path for AAC output at the given bit rate (bits per
// second), expecting 48 kHz stereo float32 frames from write.
func newAACEncoder(path string, bitrate int) (encoder, error) {
	if err := load(); err != nil {
		return nil, err
	}
	if err := loadEncoder(); err != nil {
		return nil, err
	}

	fileFmt := AudioStreamBasicDescription{
		SampleRate:       sampleRate,
		FormatID:         kAudioFormatMPEG4AAC,
		ChannelsPerFrame: channels,
	}
	clientFmt := AudioStreamBasicDescription{
		SampleRate:       sampleRate,
		FormatID:         kAudioFormatLinearPCM,
		FormatFlags:      kAudioFormatFlagIsFloat | kAudioFormatFlagIsPacked,
		BytesPerPacket:   channels * 4,
		FramesPerPacket:  1,
		BytesPerFrame:    channels * 4,
		ChannelsPerFrame: channels,
		BitsPerChannel:   32,
	}

	url := send(class("NSURL"), "fileURLWithPath:", uintptr(nsString(path)))
	if url == 0 {
		return nil, errors.New("无法创建输出文件 URL")
	}

	var ext uintptr
	st := int32(cCall(extCreate, uintptr(url), uintptr(kAudioFileM4AType),
		uintptr(unsafe.Pointer(&fileFmt)), 0, kAudioFileFlags_EraseFile,
		uintptr(unsafe.Pointer(&ext))))
	if st != 0 || ext == 0 {
		return nil, fmt.Errorf("创建 AAC 文件失败（status=%d）", st)
	}
	e := &aacEncoder{ext: ext}

	st = int32(cCall(extSetProp, ext, uintptr(kExtAudioFileProperty_ClientDataFormat),
		unsafe.Sizeof(clientFmt), uintptr(unsafe.Pointer(&clientFmt))))
	if st != 0 {
		e.close()
		return nil, fmt.Errorf("设置客户端音频格式失败（status=%d）", st)
	}

	e.setBitRate(bitrate)
	dbg.note("encoder: opened %s (aac, client lpcm/f32), bitrate=%d", path, bitrate)
	return e, nil
}

// setBitRate sets the AAC bit rate on the underlying converter. Best effort:
// if any step fails the file still records at the encoder's default rate, so
// the failure is logged to the diagnostic trace rather than returned.
func (e *aacEncoder) setBitRate(bitrate int) {
	var conv uintptr
	size := uint32(unsafe.Sizeof(conv))
	st := int32(cCall(extGetProp, e.ext, uintptr(kExtAudioFileProperty_AudioConverter),
		uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&conv))))
	if st != 0 || conv == 0 {
		dbg.note("encoder: get converter failed status=%d", st)
		return
	}
	br := uint32(bitrate)
	st = int32(cCall(acSetProp, conv, uintptr(kAudioConverterEncodeBitRate),
		4, uintptr(unsafe.Pointer(&br))))
	if st != 0 {
		dbg.note("encoder: set bitrate failed status=%d", st)
		return
	}
	// Re-set ConverterConfig so ExtAudioFile re-reads the converter's new
	// bit rate. The data is a pointer to a NULL CFArray value, not a NULL
	// pointer — passing size 0 / NULL here crashes ExtAudioFile.
	var cfg uintptr
	st = int32(cCall(extSetProp, e.ext, uintptr(kExtAudioFileProperty_ConverterConfig),
		unsafe.Sizeof(cfg), uintptr(unsafe.Pointer(&cfg))))
	dbg.note("encoder: bitrate=%d applied status=%d", bitrate, st)
}

// write encodes one span of interleaved stereo float32 frames.
func (e *aacEncoder) write(samples []float32) error {
	if len(samples) < channels {
		return nil
	}
	frames := len(samples) / channels
	var abl audioBufferList
	abl.NumberBuffers = 1
	abl.Buffers[0] = audioBuffer{
		NumberChannels: channels,
		DataByteSize:   uint32(len(samples) * 4),
		Data:           unsafe.Pointer(&samples[0]),
	}
	st := int32(cCall(extWrite, e.ext, uintptr(frames), uintptr(unsafe.Pointer(&abl))))
	runtime.KeepAlive(samples) // abl holds a raw pointer into samples
	if st != 0 {
		return fmt.Errorf("写入 AAC 失败（status=%d）", st)
	}
	return nil
}

// close finalizes the file, writing the container's trailer (moov atom).
func (e *aacEncoder) close() error {
	if e.ext == 0 {
		return nil
	}
	st := int32(cCall(extDispose, e.ext))
	e.ext = 0
	if st != 0 {
		return fmt.Errorf("关闭 AAC 文件失败（status=%d）", st)
	}
	return nil
}
