package dvb

import "errors"

var (
	ErrTuneTimeout   = errors.New("frontend lock timeout")
	ErrStreamTimeout = errors.New("stream I/O timeout")
)

type UnsafeTunerError struct{ Err error }

func (e *UnsafeTunerError) Error() string {
	return "tuner state uncertain; do not automatically reopen this tuner: " + e.Err.Error()
}
func (e *UnsafeTunerError) Unwrap() error { return e.Err }
