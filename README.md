# zapstackdriver

## Example

```go
package main

import (
  "context"
  "fmt"

  "cloud.google.com/go/logging"
  "github.com/uschen/zapstackdriver"
  "go.uber.org/zap"
  "go.uber.org/zap/zapcore"
)

func main() {

  ctx := context.Background()
	projectID := "YOUR_PROJECT_ID"
	logID := "YOUR_LOG_ID"
	client, err := logging.NewClient(ctx, projectID)
	if err != nil {
		// TODO: Handle error.
	}
	defer func() {
		// Use client to manage logs, metrics and sinks.
		// Close the client when finished.
		if err := client.Close(); err != nil {
			// TODO: Handle error.
		}
	}()
	cLogger := client.Logger(logID)
	l := zap.NewAtomicLevelAt(zapcore.DebugLevel)
	core, err := zapstackdriver.New(l, cLogger)
	if err != nil {
		panic(fmt.Errorf)
	}

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	logger.Info("your log message", zap.String("string_field", "string_value"))

	if err := logger.Sync(); err != nil {
		// TODO: Handle error.
	}
}
```
