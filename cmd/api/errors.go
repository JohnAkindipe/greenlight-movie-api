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
		uri = r.URL.RequestURI()
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
func (appPtr *application) serverErrorResponse (w http.ResponseWriter, r *http.Request, err error) {

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
