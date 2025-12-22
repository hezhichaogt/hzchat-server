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
	ErrInvalidParams:         {Code: ErrInvalidParams, Message: "Invalid or missing parameters.", Status: http.StatusBadRequest},
	ErrUnsupportedMediaType:  {Code: ErrUnsupportedMediaType, Message: "Content-Type must be application/json", Status: http.StatusUnsupportedMediaType},
	ErrInvalidJSONFormat:     {Code: ErrInvalidJSONFormat, Message: "Invalid JSON format or incorrect field types.", Status: http.StatusBadRequest},
	ErrExtraContentInBody:    {Code: ErrExtraContentInBody, Message: "Request body contains extra content.", Status: http.StatusBadRequest},
	ErrFormParseFailed:       {Code: ErrFormParseFailed, Message: "Form data or file upload failed to parse", Status: http.StatusBadRequest},
	ErrRequestEntityTooLarge: {Code: ErrRequestEntityTooLarge, Message: "Request entity size exceeds the limit", Status: http.StatusRequestEntityTooLarge},
	ErrRateLimitExceeded:     {Code: ErrRateLimitExceeded, Message: "Request rate limit exceeded. Please try again later.", Status: http.StatusTooManyRequests},

	// 2xxx: Room and Content Business Logic Errors
	ErrRoomTypeInvalid:        {Code: ErrRoomTypeInvalid, Message: "chat type is invalid, must be 'private' or 'group'", Status: http.StatusBadRequest},
	ErrRoomCodeExists:         {Code: ErrRoomCodeExists, Message: "The generated chat code already exists.", Status: http.StatusConflict},
	ErrRoomNotFound:           {Code: ErrRoomNotFound, Message: "The requested chat does not exist", Status: http.StatusNotFound},
	ErrRoomIsFull:             {Code: ErrRoomIsFull, Message: "The chat has reached its maximum client capacity", Status: http.StatusForbidden},
	ErrMessageContentTooLong:  {Code: ErrMessageContentTooLong, Message: "The message content exceeds the maximum allowed length.", Status: http.StatusRequestEntityTooLarge},
	ErrFileSizeTooLarge:       {Code: ErrFileSizeTooLarge, Message: "The file size exceeds the maximum allowed limit for this chat.", Status: http.StatusRequestEntityTooLarge},
	ErrAttachmentCountInvalid: {Code: ErrAttachmentCountInvalid, Message: "The number of attachments is outside the allowed range (1 to %d).", Status: http.StatusBadRequest},
	ErrAttachmentKeyInvalid:   {Code: ErrAttachmentKeyInvalid, Message: "Attachment file key is invalid or does not belong to this room.", Status: http.StatusForbidden},

	// 3xxx: User, Session, and Security Errors
	ErrPowChallengeRequired: {Code: ErrPowChallengeRequired, Message: "Proof-of-Work challenge is required to proceed.", Status: http.StatusForbidden},
	ErrPowChallengeInvalid:  {Code: ErrPowChallengeInvalid, Message: "Proof-of-Work proof is invalid or has expired.", Status: http.StatusForbidden},
	ErrPowChallengeInternal: {Code: ErrPowChallengeInternal, Message: "Internal PoW service error. Please try again.", Status: http.StatusInternalServerError},
	ErrSessionKicked:        {Code: ErrSessionKicked, Message: "Session replaced by new connection. Please check other tabs.", Status: http.StatusForbidden},
	ErrUnauthorized:         {Code: ErrUnauthorized, Message: "Authentication failed. Missing or invalid token.", Status: http.StatusUnauthorized},
	ErrAlreadyLoggedIn:      {Code: ErrAlreadyLoggedIn, Message: "Already logged in.", Status: http.StatusConflict},
	ErrInvalidUsername:      {Code: ErrInvalidUsername, Message: "Username: 4-20 alphanumeric characters.", Status: http.StatusBadRequest},
	ErrInvalidPassword:      {Code: ErrInvalidPassword, Message: "Password must be 6-50 characters.", Status: http.StatusBadRequest},
	ErrUserAlreadyExists:    {Code: ErrUserAlreadyExists, Message: "Username is already taken.", Status: http.StatusConflict},
	ErrInvalidCredentials:   {Code: ErrInvalidCredentials, Message: "Invalid username or password.", Status: http.StatusUnauthorized},

	// 5xxx: Internal System Errors
	ErrUnknown:           {Code: ErrUnknown, Message: "An unexpected server error occurred.", Status: http.StatusInternalServerError},
	ErrFileStorageFailed: {Code: ErrFileStorageFailed, Message: "Internal file storage service failed. Please try again.", Status: http.StatusInternalServerError},
}
