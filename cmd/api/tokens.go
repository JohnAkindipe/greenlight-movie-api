package main

import (
	"errors"
	"fmt"
	"greenlight-movie-api/internal/data"
	"greenlight-movie-api/internal/validator"
	"net/http"
	"time"
)

func (appPtr *application) createActivationTokenHandler(w http.ResponseWriter, r *http.Request) {
	//Define a struct that describes the data
	//we expect for a registered user looking
	//to get the activation token
	var reqInput struct {
		Email    string `json:"email"`
	}

	//decode the json data from the body into the user struct
	err := appPtr.readJSON(w, r, &reqInput)
	if err != nil {
		appPtr.badRequestResponse(w, r, err)
		return
	}

	//Validate the email 
	emailValidatorPtr := validator.New()
	data.ValidateEmail(emailValidatorPtr, reqInput.Email)

	//Send Error if email is not valid
	if !emailValidatorPtr.Valid() {
		appPtr.failedValidationResponse(w, r, emailValidatorPtr.Errors)
		return
	}

	//Check if email belongs to a user in our db
	//Send error if no such email in db
	userPtr, err := appPtr.dbModel.UserModel.GetUserByEmail(reqInput.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			emailValidatorPtr.AddError("email", "no matching email address found")
			appPtr.failedValidationResponse(w, r, emailValidatorPtr.Errors)
		default:
			appPtr.serverErrorResponse(w, r, err)
		}
		return
	}

	//EMAIL IS VALID and belongs to a user in our db atp
	//If user is already activated, let themm know they've been activated
	//return an error
	if userPtr.Activated {
		emailValidatorPtr.AddError("email", "user has already been activated")
		appPtr.failedValidationResponse(w, r, emailValidatorPtr.Errors)
		return
	}

	//Create activation token
	tokenPtr, err := appPtr.dbModel.TokenModel.New(data.ScopeActivation, userPtr.ID, 3*24*time.Hour)
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
		return
	}

	//Launch a background goroutine to send an email to the user containing the activation token
	appPtr.background(func() {
		fmt.Println(userPtr.Email)
		data := map[string]any{
			"userID":          userPtr.ID,
			"activationToken": tokenPtr.Plaintext,
		}
        // Since email addresses MAY be case sensitive, notice that we are sending this 
        // email using the address stored in our database for the user --- not to the 
        // reqInput.Email address provided by the client in this request.
		err := appPtr.mailer.Send(
			userPtr.Email,
			"activation_token.tmpl",
			data,
		)
		if err != nil {
			appPtr.logger.Error(err.Error())
			//See notes(3) below for why we log instead
		}
	})
	//then an html reponse to the user     
	//Send a 202 Accepted response and confirmation message to the client.
	env := envelope{
		"message": "an email will be sent to you with activation instructions",
	}
	err = appPtr.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
	}
}

/*********************************************************************************************************************/
/*
NOTES
1 WHY DELETE_ALL_FOR_USER method on TokenModel
If you implement an endpoint like this, it’s important to note that this would allow users to potentially have multiple 
valid activation tokens ‘on the go’ at any one time. That’s fine — but you just need to make sure that you delete all 
the activation tokens for a user once they’ve successfully activated (not just the token that they used).
*/