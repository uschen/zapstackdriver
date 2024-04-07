package zapstackdriver

import (
	"errors"

	"cloud.google.com/go/logging"
	"github.com/google/uuid"
	"go.uber.org/zap/zapcore"
	structpb "google.golang.org/protobuf/types/known/structpb"

	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
)

// Core is the core implements zapcore.Core
type Core struct {
	zapcore.LevelEnabler
	clogger *logging.Logger
	encoder *StructEncoder
}

// CoreOptionFunc -
type CoreOptionFunc func(*Core) error

// New -
func New(enab zapcore.LevelEnabler, cloudLogger *logging.Logger, options ...CoreOptionFunc) (*Core, error) {
	c := &Core{
		LevelEnabler: enab,
		clogger:      cloudLogger,
		encoder:      NewStructEncoder(),
	}
	if c.clogger == nil {
		return nil, errors.New("Cloud Logger is required")
	}

	// Run the options on it
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// With -
func (c *Core) With(fields []zapcore.Field) zapcore.Core {
	clone := c.clone()
	addFields(clone.encoder, fields)
	return clone
}

// Check -
func (c *Core) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

// Write -
func (c *Core) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	e2, err := c.encoder.encodeEntry(ent, fields)
	if err != nil {
		return err
	}
	if ent.Stack != "" {
		e2.AddString("stack", ent.Stack)

		// Add the context for error reporting if this is an error level message
		if ent.Level >= zapcore.ErrorLevel {
			addContext(e2, ent)
			addServiceContext(e2)
		}
	}
	e2.AddString("message", ent.Message)

	entry := logging.Entry{
		Timestamp: ent.Time,
		Severity:  zapLevelToSeverity(ent.Level),
		InsertID:  uuid.New().String(),
		Payload:   e2.Struct,
	}
	if e2.req != nil {
		entry.HTTPRequest = &logging.HTTPRequest{
			Request: e2.req,
		}
	}

	if ent.Caller.Defined {
		// e2.AddString("caller", ent.Caller.String())
		entry.SourceLocation = &logpb.LogEntrySourceLocation{
			File:     ent.Caller.File,
			Line:     int64(ent.Caller.Line),
			Function: ent.Caller.TrimmedPath(),
		}
	}

	c.clogger.Log(entry)

	return nil
}

// Sync - call stackdriver logger to 'Flush'
func (c *Core) Sync() error {
	return c.clogger.Flush()
}

// addContext - Add's context to the encoder
func addContext(e2 *StructEncoder, ent zapcore.Entry) {
	reportLocation := map[string]*structpb.Value{}

	// If caller is defined, add the file, line & function
	if ent.Caller.Defined {
		contextFields := map[string]*structpb.Value{}
		contextFields["filePath"] = &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: ent.Caller.File,
			},
		}
		contextFields["lineNumber"] = &structpb.Value{
			Kind: &structpb.Value_NumberValue{
				NumberValue: float64(ent.Caller.Line),
			},
		}
		contextFields["functionName"] = &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: ent.Caller.Function,
			},
		}

		// Add fields to reportLocation
		reportLocation["reportLocation"] = &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: contextFields,
				},
			},
		}
	}

	// Add it to context
	e2.Struct.Fields["context"] = &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: reportLocation,
			},
		},
	}
}

// addServiceContext - Add's service context to the encoder
func addServiceContext(e2 *StructEncoder) {
	contextFields := map[string]*structpb.Value{}

	contextFields["service"] = &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: "GO",
		},
	}

	// Add it to context
	e2.Struct.Fields["serviceContext"] = &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: contextFields,
			},
		},
	}
}

func (c *Core) clone() *Core {
	return &Core{
		LevelEnabler: c.LevelEnabler,
		encoder:      c.encoder.clone(),
		clogger:      c.clogger,
	}
}

func zapLevelToSeverity(level zapcore.Level) logging.Severity {
	switch level {
	case zapcore.DebugLevel:
		return logging.Debug
	case zapcore.InfoLevel:
		return logging.Info
	case zapcore.WarnLevel:
		return logging.Warning
	case zapcore.ErrorLevel:
		return logging.Error
	case zapcore.DPanicLevel:
		return logging.Critical
	case zapcore.PanicLevel:
		return logging.Alert
	case zapcore.FatalLevel:
		return logging.Emergency
	default:
		return logging.Info
	}
}
