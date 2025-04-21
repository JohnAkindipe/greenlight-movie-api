package main

import (
	"net/http"
)

/*********************************************************************************************************************/
/*
The important thing to point out here is that healthcheckHandler is implemented as a method on our application struct.
This is an effective and idiomatic way to make dependencies available to our handlers without resorting to
global variables or closures — any dependency that the healthcheckHandler needs can simply be included as a field
in the application struct when we initialize it in main().

HANDLES GET /v1/healthcheck
*/
func (appPtr *application) healthcheckHandler (w http.ResponseWriter, r *http.Request) {
/*********************************************************************************************************************/
	//USING JSON MARSHALLING
	//wrap the data with the envelope
	wrappedData := envelope{ 
		"status": "available",
		"system_info": map[string]string{
			"version": version,
			"environment": appPtr.config.env,
		},
	}

	// headers := map[string][]string {
	// 	"Content-Type": {"application/json"},
	// }

	// Pass the map to the app.writeJSON method. If there was an error, we log it and send the client
    // a generic error message.
	err := appPtr.writeJSON(w, http.StatusOK, wrappedData, nil)
	if err != nil {
		appPtr.logger.Error(err.Error())
		http.Error(w, "The server encountered a problem and could not process your request", http.StatusInternalServerError)
	}
}