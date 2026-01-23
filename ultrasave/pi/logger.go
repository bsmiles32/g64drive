package pi

// Interface and adapter func that can be used to log messages
type Logger interface {
	Log(s string, args ...interface{})
}

type LoggerFunc func(string, ...interface{})

func (l LoggerFunc) Log(s string, args ...interface{}) {
	l(s, args...)
}

func log(l Logger, s string, args ...interface{}) {
	if l == nil {
		return
	}

	l.Log(s, args...)
}
