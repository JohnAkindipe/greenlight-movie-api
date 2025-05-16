package data

import (
	"fmt"
	"greenlight-movie-api/internal/validator"
	"time"
)

/*********************************************************************************************************************/
//Allowed Genre Values
var allowedGenres = []string{"adventure", "action", "animation", "romance", "comedy", "history"}

/*********************************************************************************************************************/
//MOVIE STRUCT
//This defines the data format for a movie in our API
/*
The - (hyphen) directive can be used when you never want a particular struct field to appear in the JSON output.
This is useful for fields that contain internal system information that isn’t relevant to your users, or sensitive
information that you don’t want to expose (like the hash of a password).

In contrast the omitempty directive hides a field in the JSON output if and only if the struct field value is empty,
where empty is defined as being:
- Equal to false, 0, or ""
- An empty array, slice or map
- A nil pointer or a nil interface value
*/
type Movie struct {
	ID        int64 		`json:"id"`
	CreatedAt time.Time		`json:"-"`
	Title string			`json:"title"`
	Year int32				`json:"year,omitempty"`
	Runtime Runtime 		`json:"runtime,omitempty"`//Movie runtime (in minutes) 
	Genres []string			`json:"genres,omitempty"`
	Version int32 			`json:"version"`//version number is initially 1 and will be incremented everytime
					//info about the movie is updated
}


/*
MOVIE INPUT
Data we expect from client when they want to create a movie
*/
type MovieInput struct {
		Title   string   `json:"title"`
        Year    int32    `json:"year"`
        Runtime Runtime    `json:"runtime"`
        Genres  []string `json:"genres"`
}
/*********************************************************************************************************************/
/*
VALIDATE USER'S INPUT
Call all the individual validate functions
*/
func ValidateMovie(movieValidatorPtr *validator.Validator, movieDataPtr *Movie) {

	// Ensure Genres Slice contains between 1 and 5 unique genres, as contained in our allowed genres list
	movieValidatorPtr.Check(
		validator.Unique(movieDataPtr.Genres), 
		"genres", 
		"duplicate genres not allowed",
	)
	movieValidatorPtr.Check(
		len(movieDataPtr.Genres) > 0 && len(movieDataPtr.Genres) <= 5,  
		"genres", 
		"genre should contain between 1 and 5 unique genres",
	)
	permittedGenres(movieDataPtr.Genres, movieValidatorPtr)

	// Ensure Title is not empty and not greater than 500 bytes in length
	movieValidatorPtr.Check(
		movieDataPtr.Title != "", 
		"title", 
		"movie title cannot be empty",
	)
	movieValidatorPtr.Check(
		len([]byte(movieDataPtr.Title)) <= 500, 
		"title", 
		"movie title must not be > 500 bytes long",
	)

	// Ensure runtime is an integer greater than 0
	movieValidatorPtr.Check(movieDataPtr.Runtime > 0, "runtime", "runtime should be an integer greater than 0")

	// Ensure movie year is not empty and must be between 1888 and current year
	movieValidatorPtr.Check(
		movieDataPtr.Year != 0, 
		"year", 
		fmt.Errorf("invalid movie year: %d. year must be from 1888 to date", movieDataPtr.Year).Error(),
	)
	movieValidatorPtr.Check(
		movieDataPtr.Year >=  1888 && int(movieDataPtr.Year) <= time.Now().Year(), 
		"year",  
		fmt.Errorf("invalid movie year: %d. year must be from 1888 to date", movieDataPtr.Year).Error(),
	)
}

/*********************************************************************************************************************/
/*
VALIDATE GENRES
Validate that the genres slice only contains genres in the permitted genres slice
*/
func permittedGenres(genres []string, movieValidatorPtr *validator.Validator) {
	for _, genre := range genres {
		if !validator.PermittedValue(genre, allowedGenres...) {
			movieValidatorPtr.AddError(
				"genres", 
				fmt.Sprintf("must not contain values aside the following: %+v", allowedGenres),
			)
			return
		}
	}
}