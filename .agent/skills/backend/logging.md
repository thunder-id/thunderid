# Logging

## Logger

Build the logger once in the constructor and keep it on the struct:

```go
logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ExampleService")),
```

- No logger component name constants.
- No new logger per method.

## What to Log

- No Info logs for routine operations. Use Debug.
- An expected condition (Ex: a user with no email attribute) is Debug or Warn, never Error.
- A failure log carries its cause: `log.Error(err)`.
- Guard a Debug log with `IsDebugEnabled()` only when building its fields is costly (masking a map, building a slice).
- Do not log client errors. They go back to the caller.
- Log a server failure once, in the layer that decides what to do with it, usually the service. A function that returns an error does not also log it.
- Executors do not log errors. For a server failure, return a Go error: the task execution node logs it.
- No PII: usernames, emails, phone numbers, attribute values, subjects, tokens, credentials. Treat user and entity IDs as PII too, and log one only when needed.
- Mask a value that must appear with `log.MaskedString`, `log.MaskedStrings`, or `log.MaskedMap`.
- Use the field key spelling the rest of the codebase uses.
