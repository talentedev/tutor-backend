package utils

import (
	"github.com/olebedev/emitter"
)

var bus = &emitter.Emitter{}

const (

	// EvLessonCreated : lesson *store.LessonMgo, user *store.UserMgo
	EvLessonCreated = "lesson-created"

	// EvLessonNoteCreated : note *store.LessonNote, user *store.UserMgo
	EvLessonNoteCreated = "lesson-note-created"
)

// Bus retrieve bus event emitter
func Bus() *emitter.Emitter {
	return bus
}
