# Errors

## Service Errors

- Services return `*tidcommon.ServiceError`, never a bare `error`.
- Only `ServiceError` crosses a package boundary. Sentinel errors stay private and are mapped before they leave the package.
- Packages define client errors only. For a server failure, return `&tidcommon.InternalServerError`. Do not define a new error type.

## Error Constants

```go
// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
ErrorInvalidRequestFormat = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "USR-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.userservice.invalid_request_format",
		DefaultValue: "Invalid request format",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.userservice.invalid_request_format_description",
		DefaultValue: "The request body is malformed or contains invalid data",
	},
}
```

- In `error_constants.go`, with a doc comment.
- Code: the package's prefix, a hyphen, 4 digits. Client errors are `1xxx`. A new package picks an unused prefix.
- Keep the constants in ascending code order. A new error takes the next free number and goes at the end of the block, never in between. Check the code is not used anywhere else.
- Both i18n keys, following the package's key prefix, with `_description` on the second.
- The description adds the cause or what to do. It does not repeat the error.
- Placeholders are `{{param(name)}}`, filled with `ErrorX.WithParams(...)`. Any other form reaches the client verbatim.
- Run `make generate_i18n` after adding or changing an `I18nMessage`.
- Reuse an existing error that already describes the failure.
- An error that changes an API response needs the matching `api/*.yaml` change (`resource.md`, API Spec).
