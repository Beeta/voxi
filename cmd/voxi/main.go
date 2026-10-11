// Command voxi runs Voxi (有听有记), a macOS meeting recorder: it captures
// the microphone and the system output, transcribes the recording, and builds
// summaries, mind maps, Q&A and notes on top of the transcript.
package main

import (
	"log"

	"github.com/zhaorongcun/voxi/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
