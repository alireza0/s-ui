package core

import (
	"context"
	"io"
	"os"
	"time"

	suiLog "github.com/alireza0/s-ui/logger"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/service/filemanager"
)

type PlatformWriter struct{}

func (p PlatformWriter) DisableColors() bool {
	return true
}
func (p PlatformWriter) WriteMessage(level log.Level, message string) {
	switch level {
	case log.LevelInfo:
		suiLog.Info(message)
	case log.LevelWarn:
		suiLog.Warning(message)
	case log.LevelPanic:
	case log.LevelFatal:
	case log.LevelError:
		suiLog.Error(message)
	default:
		suiLog.Debug(message)
	}
}

func NewFactory(options log.Options) (log.Factory, error) {
	logOptions := options.Options

	if logOptions.Disabled {
		return log.NewNOPFactory(), nil
	}

	var logWriter io.Writer
	var logFilePath string

	switch logOptions.Output {
	case "":
		logWriter = options.DefaultWriter
		if logWriter == nil {
			logWriter = os.Stderr
		}
	case "stderr":
		logWriter = os.Stderr
	case "stdout":
		logWriter = os.Stdout
	default:
		logFilePath = logOptions.Output
	}
	logFormatter := log.Formatter{
		BaseTime:         options.BaseTime,
		DisableColors:    logOptions.DisableColor || logFilePath != "",
		DisableTimestamp: !logOptions.Timestamp && logFilePath != "",
		FullTimestamp:    logOptions.Timestamp,
		TimestampFormat:  "-0700 2006-01-02 15:04:05",
	}
	factory := NewDefaultFactory(
		options.Context,
		logFormatter,
		logWriter,
		logFilePath,
	)
	if logOptions.Level != "" {
		logLevel, err := log.ParseLevel(logOptions.Level)
		if err != nil {
			return nil, common.Error("parse log level", err)
		}
		factory.SetLevel(logLevel)
	} else {
		factory.SetLevel(log.LevelTrace)
	}
	return factory, nil
}

var _ log.Factory = (*defaultFactory)(nil)

type defaultFactory struct {
	ctx        context.Context
	formatter  log.Formatter
	writer     io.Writer
	file       *os.File
	filePath   string
	level      log.Level
	subscriber *observable.Subscriber[log.Entry]
	observer   *observable.Observer[log.Entry]
}

func NewDefaultFactory(
	ctx context.Context,
	formatter log.Formatter,
	writer io.Writer,
	filePath string,
) log.ObservableFactory {
	factory := &defaultFactory{
		ctx:        ctx,
		formatter:  formatter,
		writer:     writer,
		filePath:   filePath,
		level:      log.LevelTrace,
		subscriber: observable.NewSubscriber[log.Entry](128),
	}
	return factory
}

func (f *defaultFactory) Start() error {
	if f.filePath != "" {
		logFile, err := filemanager.OpenFile(f.ctx, f.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		f.writer = logFile
		f.file = logFile
	}
	return nil
}

func (f *defaultFactory) Close() error {
	return common.Close(
		common.PtrOrNil(f.file),
		f.subscriber,
	)
}

func (f *defaultFactory) Level() log.Level {
	return f.level
}

func (f *defaultFactory) SetLevel(level log.Level) {
	f.level = level
}

func (f *defaultFactory) Logger() log.ContextLogger {
	return f.NewLogger("")
}

func (f *defaultFactory) NewLogger(tag string) log.ContextLogger {
	return &observableLogger{f, tag}
}

func (f *defaultFactory) Subscribe() (subscription observable.Subscription[log.Entry], done <-chan struct{}, err error) {
	return f.observer.Subscribe()
}

func (f *defaultFactory) UnSubscribe(sub observable.Subscription[log.Entry]) {
	f.observer.UnSubscribe(sub)
}

type observableLogger struct {
	*defaultFactory
	tag string
}

func (l *observableLogger) Log(ctx context.Context, level log.Level, args []any) {
	level = log.OverrideLevelFromContext(level, ctx)
	if level > l.level {
		return
	}
	msg := F.ToString(args...)
	switch level {
	case log.LevelInfo:
		suiLog.Info(l.tag, msg)
	case log.LevelWarn:
		suiLog.Warning(l.tag, msg)
	case log.LevelPanic:
	case log.LevelFatal:
	case log.LevelError:
		suiLog.Error(l.tag, msg)
	default:
		suiLog.Debug(l.tag, msg)
	}
	if (l.filePath != "" || l.writer != os.Stderr) && l.writer != nil {
		message := l.formatter.Format(ctx, level, l.tag, msg, time.Now())
		l.writer.Write([]byte(message))
	}
}

func (l *observableLogger) Trace(args ...any) {
	l.TraceContext(context.Background(), args...)
}

func (l *observableLogger) Debug(args ...any) {
	l.DebugContext(context.Background(), args...)
}

func (l *observableLogger) Info(args ...any) {
	l.InfoContext(context.Background(), args...)
}

func (l *observableLogger) Warn(args ...any) {
	l.WarnContext(context.Background(), args...)
}

func (l *observableLogger) Error(args ...any) {
	l.ErrorContext(context.Background(), args...)
}

func (l *observableLogger) Fatal(args ...any) {
	l.FatalContext(context.Background(), args...)
}

func (l *observableLogger) Panic(args ...any) {
	l.PanicContext(context.Background(), args...)
}

func (l *observableLogger) TraceContext(ctx context.Context, args ...any) {
	l.Log(ctx, log.LevelTrace, args)
}

func (l *observableLogger) DebugContext(ctx context.Context, args ...any) {
	l.Log(ctx, log.LevelDebug, args)
}

func (l *observableLogger) InfoContext(ctx context.Context, args ...any) {
	l.Log(ctx, log.LevelInfo, args)
}

func (l *observableLogger) WarnContext(ctx context.Context, args ...any) {
	l.Log(ctx, log.LevelWarn, args)
}

func (l *observableLogger) ErrorContext(ctx context.Context, args ...any) {
	l.Log(ctx, log.LevelError, args)
}

func (l *observableLogger) FatalContext(ctx context.Context, args ...any) {
	l.Log(ctx, log.LevelFatal, args)
}

func (l *observableLogger) PanicContext(ctx context.Context, args ...any) {
	l.Log(ctx, log.LevelPanic, args)
}
