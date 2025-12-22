/*
Package errs provides custom error types and application-level error code constants.

These error codes are used to clearly identify specific business or system errors
both internally within the server and in communication with clients.
*/
package errs

// 1xxx: General Request Handling Errors
const (
	// ErrInvalidParams indicates that request parameter validation failed.
	ErrInvalidParams = 1001

	// ErrUnsupportedMediaType indicates that the request header Content-Type is not supported.
	ErrUnsupportedMediaType = 1002

	// ErrInvalidJSONFormat indicates that the request body JSON format is incorrect (e.g., syntax error).
	ErrInvalidJSONFormat = 1003

	// ErrExtraContentInBody indicates that the request body contained extra content after valid JSON data.
	ErrExtraContentInBody = 1004

	// ErrFormParseFailed indicates failure to parse multipart or URL-encoded form data.
	ErrFormParseFailed = 1005

	// ErrRequestEntityTooLarge indicates that the request body size exceeded the server limit.
	ErrRequestEntityTooLarge = 1006

	// ErrRateLimitExceeded indicates that the request rate has exceeded the set limit.
	ErrRateLimitExceeded = 1007
)

// 2xxx: Room and Content Business Logic Errors
const (
	// ErrRoomTypeInvalid indicates that an invalid room type was provided during creation or joining.
	ErrRoomTypeInvalid = 2101

	// ErrRoomCodeExists indicates that the attempted room code for creation already exists.
	ErrRoomCodeExists = 2102

	// ErrRoomNotFound indicates that the attempted room code for operation does not exist.
	ErrRoomNotFound = 2103

	// ErrRoomIsFull indicates that the room being joined has reached its maximum user capacity.
	ErrRoomIsFull = 2104

	// ErrMessageContentTooLong indicates that the user's message content exceeded the maximum length limit.
	ErrMessageContentTooLong = 2201

	// ErrFileSizeTooLarge indicates that the requested file size exceeded the application's business logic limit.
	ErrFileSizeTooLarge = 2202

	// ErrAttachmentCountInvalid indicates that the number of attachments in a message is either zero or exceeds the allowed maximum.
	ErrAttachmentCountInvalid = 2203

	// ErrAttachmentKeyInvalid indicates that an attachment key does not belong to the expected room or user.
	ErrAttachmentKeyInvalid = 2204
)

// 3xxx: User, Session, and Security Errors
const (
	// ErrPowChallengeRequired indicates the client must complete a Proof-of-Work challenge first.
	ErrPowChallengeRequired = 3001

	// ErrPowChallengeInvalid indicates that the PoW proof provided by the client is invalid or incorrect.
	ErrPowChallengeInvalid = 3002

	// ErrPowChallengeInternal indicates an internal error occurred during the PoW challenge generation or validation process.
	ErrPowChallengeInternal = 3003

	// ErrSessionKicked indicates that the current client connection has been terminated.
	ErrSessionKicked = 3004

	// ErrUnauthorized indicates that the request requires valid authentication (e.g., missing or invalid JWT).
	ErrUnauthorized = 3005

	// ErrAlreadyLoggedIn indicates the user is already authenticated.
	ErrAlreadyLoggedIn = 3006

	// ErrInvalidUsername indicates the username does not meet requirements (4-20 chars, a-z0-9_).
	ErrInvalidUsername = 3007

	// ErrInvalidPassword indicates the password length is out of range (6-50 chars).
	ErrInvalidPassword = 3008

	// ErrUserAlreadyExists indicates the username has already been taken.
	ErrUserAlreadyExists = 3009

	// ErrInvalidCredentials indicates wrong username or password during login.
	ErrInvalidCredentials = 3010
)

// 5xxx: Internal System Errors
const (
	// ErrUnknown represents an unclassified, general server internal error.
	ErrUnknown = 5000

	// ErrFileStorageFailed indicates an internal server error related to file storage (e.g., S3/R2 API failure).
	ErrFileStorageFailed = 5001
)
