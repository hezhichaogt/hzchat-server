/*
Package errs provides custom error types and application-level error code constants.

This file defines the map from error codes to the CustomError struct, used to standardize
HTTP responses and internal error handling.
*/
package errs

import "net/http"

// errorMap stores the detailed CustomError struct corresponding to every application error code.
// The key is the error code (int), and the value contains the user message and HTTP status code.
var errorMap = map[int]CustomError{
	// 1xxx: General Request Handling Errors
	ErrInvalidParams:         {Code: ErrInvalidParams, Message: "Invalid request parameters."},
	ErrUnsupportedMediaType:  {Code: ErrUnsupportedMediaType, Message: "Unsupported request format."},
	ErrInvalidJSONFormat:     {Code: ErrInvalidJSONFormat, Message: "Unsupported request format."},
	ErrExtraContentInBody:    {Code: ErrExtraContentInBody, Message: "Request contains unexpected data."},
	ErrFormParseFailed:       {Code: ErrFormParseFailed, Message: "Failed to process uploaded data."},
	ErrRequestEntityTooLarge: {Code: ErrRequestEntityTooLarge, Message: "Request size is too large."},
	ErrRateLimitExceeded:     {Code: ErrRateLimitExceeded, Message: "Too many requests. Please try again later.", Status: http.StatusTooManyRequests},

	// 2xxx: Room and Content Business Logic Errors
	ErrRoomTypeInvalid:        {Code: ErrRoomTypeInvalid, Message: "Invalid chat type."},
	ErrRoomCodeExists:         {Code: ErrRoomCodeExists, Message: "Chat code already exists."},
	ErrRoomNotFound:           {Code: ErrRoomNotFound, Message: "Chat room not found."},
	ErrRoomIsFull:             {Code: ErrRoomIsFull, Message: "This chat room is full."},
	ErrMessageContentTooLong:  {Code: ErrMessageContentTooLong, Message: "Message is too long."},
	ErrFileSizeTooLarge:       {Code: ErrFileSizeTooLarge, Message: "File is too large."},
	ErrAttachmentCountInvalid: {Code: ErrAttachmentCountInvalid, Message: "Invalid number of attachments."},
	ErrAttachmentKeyInvalid:   {Code: ErrAttachmentKeyInvalid, Message: "Invalid attachment."},

	// 3xxx: User, Session, and Security Errors
	ErrPowChallengeRequired: {Code: ErrPowChallengeRequired, Message: "Verification required. Please try again."},
	ErrPowChallengeInvalid:  {Code: ErrPowChallengeInvalid, Message: "Verification failed. Please try again."},
	ErrPowChallengeInternal: {Code: ErrPowChallengeInternal, Message: "Verification service error. Please try again later."},
	ErrSessionKicked:        {Code: ErrSessionKicked, Message: "You were signed in on another device."},
	ErrAlreadyLoggedIn:      {Code: ErrAlreadyLoggedIn, Message: "You are already signed in."},
	ErrInvalidUsername:      {Code: ErrInvalidUsername, Message: "Invalid username."},
	ErrInvalidPassword:      {Code: ErrInvalidPassword, Message: "Invalid password."},
	ErrUserAlreadyExists:    {Code: ErrUserAlreadyExists, Message: "Username is already taken."},
	ErrInvalidCredentials:   {Code: ErrInvalidCredentials, Message: "Incorrect username or password."},
	ErrUserNotFound:         {Code: ErrUserNotFound, Message: "Account not found."},
	ErrOldPasswordInvalid:   {Code: ErrOldPasswordInvalid, Message: "Current password is incorrect."},

	ErrUnauthorized: {Code: ErrUnauthorized, Message: "Please sign in to continue.", Status: http.StatusUnauthorized},

	// 5xxx: Internal System Errors
	ErrUnknown:           {Code: ErrUnknown, Message: "Something went wrong. Please try again.", Status: http.StatusInternalServerError},
	ErrFileStorageFailed: {Code: ErrFileStorageFailed, Message: "File upload failed. Please try again."},
}
