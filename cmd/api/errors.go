package main

import (
	"fmt"
	"net/http"
)

/*********************************************************************************************************************/
//LOG ERRORS FROM BEING UNABLE TO SEND ERROR RESPONSES
// The logError() method is a generic helper for logging an error message along
// with the current request method and URL as attributes in the log entry.
func (appPtr *application) logError(r *http.Request, err error) {

	//Get method and uri from request
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)

	//Log the error using our structured logger to indicate what went wrong
	appPtr.logger.Error(err.Error(), "method", method, "uri", uri)
}

/*********************************************************************************************************************/
/*
SEND ERROR IN JSON FORMAT USING WRITE JSON HELPER
This function helps us send JSON-formatted error responses to clients
if there was an error processing the request. It uses the writeJSON helper to achieve this.
If it was unable to send this error response to the client using writeJSON, writeJSON returns
an error and we log this error with our logError method.
*/
func (appPtr *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	//wrap message in envelope
	env := envelope{
		"error": message,
	}

	//write json-formatted error response to client
	err := appPtr.writeJSON(w, status, env, nil)

	// log errors, if writeJSON unable to send the error to client
	// in JSON format and fall back to sending the client an empty response with a
	// 500 Internal Server Error status code.
	if err != nil {
		appPtr.logError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

/*********************************************************************************************************************/
// SERVER ERROR RESPONSE
/*
The serverErrorResponse() method will be used when our application encounters an
unexpected problem at runtime. It logs the detailed error message, then uses the
errorResponse() helper to send a 500 Internal Server Error status code and JSON
response (containing a generic error message) to the client.
*/
func (appPtr *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {

	appPtr.logError(r, err)

	appPtr.errorResponse(w, r, http.StatusInternalServerError, "we encountered a problem in our server")
}

/*********************************************************************************************************************/
/*
NOT FOUND ERROR RESPONSE
Called notFoundResponse by author
The notFoundResponse() method will be used to send a 404 Not Found status code and
JSON response to the client. Notice that it implements the http.Handlerfunc type
This is intentional, because, we will pass this to the router object created with httprouter.new
in our app.router method.
*/
func (appPtr *application) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	// send an error explaining we could not find the requested resource
	appPtr.errorResponse(w, r, http.StatusNotFound, "Could not find the requested resource")
}

/*********************************************************************************************************************/
/*
METHOD NOT ALLOWED ERROR RESPONSE
Called methodNotAllowedResponse by author
The methodNotAllowedResponse() method will be used to send a 405 Method Not Allowed
status code and JSON response to the client. Notice that it implements the http.HandlerFunc type.
This is intentional, because, we will pass this to the router object created with httprouter.new
in our app.router method.
*/
func (appPtr *application) methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	// send an error explaining we could not find the requested resource
	msg := fmt.Sprintf("the %s method is not allowed for this resource", r.Method)
	appPtr.errorResponse(w, r, http.StatusMethodNotAllowed, msg)
}

/*********************************************************************************************************************/
//BAD REQUEST RESPONSE
/*
This is merely a wrapper round the error response, but we know that we are sending a badrequest response
when we use this.
*/
func (appPtr *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {

	appPtr.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

/*********************************************************************************************************************/
/*
FAILED VALIDATION RESPONSE
writes a 422 Unprocessable Entity and the contents of the errors map from our new Validator type as a JSON response
body.
*/
func (appPtr *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, validationErrors map[string]string) {
	appPtr.errorResponse(w, r, http.StatusUnprocessableEntity, validationErrors)
}

/*********************************************************************************************************************/
/*
EDIT CONFLICT RESPONSE
writes a status conflict header to the client who is trying to update a record which has either been deleted or updated
since it was last retrieved (read) from the db, this is part of our optimistic concurrency controls - check ch8.2
let's go further for further explanation.
*/
func (appPtr *application) editConflictResponse(w http.ResponseWriter, r *http.Request) {
	// send an error explaining we could not find the requested resource
	appPtr.errorResponse(w, r, http.StatusConflict, "trying to update a changed or deleted movie - try again!")
}

/*********************************************************************************************************************/
/*
GLOBAL RATE LIMIT EXCEEDED RESPONSE
This is when we have received too much load on our server.
*/
func (appPtr *application) globalRateLimitExceededResponse(w http.ResponseWriter, r *http.Request) {
	//send an error to try again shortly
	appPtr.errorResponse(w, r, http.StatusTooManyRequests, "our servers are currently handling a lot of requests - try again shortly")
}

/*********************************************************************************************************************/
/*
RATE LIMIT EXCEEDED RESPONSE
This is for individualized rate-limit responses i.e. when a particular client
has sent too many requests
*/
func (appPtr *application) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request) {
	// send an error to try again later
	appPtr.errorResponse(w, r, http.StatusTooManyRequests, "too many requests - try again later")
}

/*********************************************************************************************************************/
/*
INVALID CREDENTIALS RESPONSE
This is for whenever a user submits invalid email or password for whatever
reason including to get an auth-token or to log in.
*/
func (appPtr *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	// send an error to try again later
	appPtr.errorResponse(w, r, http.StatusUnauthorized, "invalid credentials")
}

/*********************************************************************************************************************/
/*
INVALID AUTHENTICATION TOKEN RESPONSE
This is for whenever a user submits invalid email or password for whatever
reason including to get an auth-token or to log in.
*/
func (appPtr *application) invalidAuthenticationTokenResponse(w http.ResponseWriter, r *http.Request) {
	// send an error to try again later
	w.Header().Set("WWW-Authenticate", "Bearer")
	appPtr.errorResponse(w, r, http.StatusUnauthorized, "invalid or missing authentication token")
}

/*********************************************************************************************************************/
/*
AUTHENTICATION REQUIRED RESPONSE
This is for when an anonymous (unactivated and unauthenticated) user tries to access an endpoint which requires
activation and authentication - These explanations need refining
*/
func (app *application) authenticationRequiredResponse(w http.ResponseWriter, r *http.Request) {
	message := "you must be authenticated to access this resource"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

/*********************************************************************************************************************/
/*
ACTIVATION REQUIRED RESPONSE
This is for when an activated but unauthenticated user tries to access an endpoint which requires
authentication - These explanations need refining
*/
func (app *application) activationRequiredResponse(w http.ResponseWriter, r *http.Request) {
	message := "your user account must be activated to access this resource"
	app.errorResponse(w, r, http.StatusForbidden, message)
}

/*********************************************************************************************************************/
/*
NOT PERMITTED RESPONSE
This is for when a user without the necessary permission (such as "movie:read" or "movie:write")
tries to perform this action on an endpoint that requires the necessary permission
*/
func (app *application) notPermittedResponse(w http.ResponseWriter, r *http.Request) {
	message := `
		You are not permitted to perform this action.
		Activate your account for full privilege
	`
	app.errorResponse(w, r, http.StatusForbidden, message)
}
