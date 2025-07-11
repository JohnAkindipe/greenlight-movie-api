package main

import (
	"errors"
	"greenlight-movie-api/internal/data"
	"greenlight-movie-api/internal/validator"
	"net/http"
)

/*
DUMMY USER
{
    "name": "John",
    "email": "john@john.com",
    "password": "akindipe123"
}
*/

//POST /v1/users
//To create a new user
func (appPtr *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	//Define a struct that describes the data
	//we expect for a new user
	type newUserInput struct {
		Name     string	`json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	//initialize a new user as the destination to marshal
	//the incoming json data into
	userInput := newUserInput{}

	//decode the json data from the body into the user struct
	err := appPtr.readJSON(w, r, &userInput)
	if err != nil {
		appPtr.badRequestResponse(w, r, err)
		return
	}

	//copy the newuserinput into a data.User struct which we will pass
	//into the data.ValidateUser function. We have to do this because
	//the data.User struct does not allow us to encode or decode the 
	//password field to and from json. This is necessary for security reasons
	//exactly why is not clear to me yet.
	user := data.User{
		Name: userInput.Name,
		Email: userInput.Email,
		Activated: false,
	}
	if err := user.Password.Set(userInput.Password); err != nil {
		appPtr.serverErrorResponse(w, r, err)
		return
	}

	//perform validation checks on the user using the validation
	//we created already
	userValidatorPtr := validator.New()

	data.ValidateUser(userValidatorPtr, &user)
	if !userValidatorPtr.Valid() {
		appPtr.failedValidationResponse(w, r, userValidatorPtr.Errors)
		return
	}

	//At this point, the user has passed all our validation checks
	//we can pass this user into the database to be inserted into the
	//database
	err = appPtr.dbModel.UserModel.InsertUser(&user)
	if err != nil {
		switch {
			case errors.Is(err, data.ErrDuplicateEmail):
				userValidatorPtr.AddError("email", "an account exists already with that email")
				appPtr.failedValidationResponse(w, r, userValidatorPtr.Errors)
			default:
				appPtr.serverErrorResponse(w, r, err)
		}
		return
	}

	//We have successfully inserted the user into our db.
	//send a message to the user that we have successfully created the user
	//with the data of the newly created user in json. Send an error response
	//if (for whatever reason), we are unable to send the json response
	err = appPtr.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
	if err != nil {
		appPtr.serverErrorResponse(w, r, err)
		return
	}
}

//GET /v1/user/:email
//To get a user by their email
func (appPtr *application) getUser(w http.ResponseWriter, r *http.Request) {

}