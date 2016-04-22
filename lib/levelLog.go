package lib

import (
	"io"
	"log"
	"os"
)

type Level uint8

const (
	PanicLevel Level = iota
	FatalLevel
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
)

type LevelLogger struct {
	Level  Level
	Logger *log.Logger
}

// usage
/*
	lib.Debugf("%s", "DEBUG")
	lib.Debugln("DEBUG")
	lib.Errorf("%s", "ERROR")
	lib.Errorln("ERROR")
	lib.Infof("%s", "INFO")
	lib.Infoln("INFO")
*/
var (
	loggingLevel = InfoLevel

	ERROR = NewLevelLogger(ErrorLevel, os.Stderr, "[ERRO]  ", log.Ldate|log.Ltime|log.Lshortfile)
	INFO  = NewLevelLogger(InfoLevel, os.Stdout, "[INFO]  ", log.Ldate|log.Ltime|log.Lshortfile)
	DEBUG = NewLevelLogger(DebugLevel, os.Stdout, "[DEBU]  ", log.Ldate|log.Ltime|log.Lshortfile)

	Errorf  = ERROR.Printf
	Errorln = ERROR.Println
	Infof   = INFO.Printf
	Infoln  = INFO.Println
	Debugf  = DEBUG.Printf
	Debugln = DEBUG.Println
)

func NewLevelLogger(level Level, handle io.Writer, prefix string, flag int) *LevelLogger {
	return &LevelLogger{
		Level:  level,
		Logger: log.New(handle, prefix, flag),
	}
}

func SetLevel(level Level) {
	loggingLevel = level
}

func (l *LevelLogger) Printf(format string, args ...interface{}) {
	if l.Level <= loggingLevel {
		l.Logger.Printf(format, args...)
	}
}

func (l *LevelLogger) Println(args ...interface{}) {
	if l.Level <= loggingLevel {
		l.Logger.Println(args...)
	}
}
