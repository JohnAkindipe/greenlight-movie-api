package data

import (
	"database/sql"
	"errors"
	"fmt"
	"greenlight-movie-api/internal/validator"
	"time"

	"github.com/lib/pq"
)

/*********************************************************************************************************************/
//Allowed Genre Values
var allowedGenres = []string{"adventure", "action", "animation", "romance", "comedy", "history", "drama"}

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
	Version int32 			`json:"version,omitempty"`//version number is initially 1 and will be incremented everytime
					//info about the movie is updated
}
/*********************************************************************************************************************/
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
MOVIE MODEL
*/
//Methinks we design this model to wrap a connection pool and will thus represent a pool dedicated to working with
//the movie table in our database. We'll define methods against the MovieModel to perform CRUD operations against
//the movie database. The DB connection pool it wraps will do the heavy lifting, inside these methods, hence
//why we made the engineering decision to include it as a field in the struct, basically, it is a dependency that
//its methods will need, therefore we're doing some sort of dependency injection, here.
type MovieModel struct{
	DBPtr *sql.DB
}

/*
CREATE (INSERT) MOVIE - Create a new movie in the database, return an error
should the operation fail
*/
func (movieModel MovieModel) Insert(moviePtr *Movie) error {
	rowPtr := movieModel.DBPtr.QueryRow(`
		INSERT INTO movies(title, year, runtime, genres)
		VALUES($1, $2, $3, $4) RETURNING id, created_at, version
	`, moviePtr.Title, moviePtr.Year, moviePtr.Runtime, pq.Array(moviePtr.Genres))

	//scan result of sql query into the movie pointed at by moviePtr
	//return an error if unsuccessful
 	return rowPtr.Scan(&moviePtr.ID, &moviePtr.CreatedAt, &moviePtr.Version)
}

/*
READ (GET) MOVIE (Get by Author; movieModel - Movie by author)
Get a movie from the database, given the movie id
*/
func(movieModel MovieModel) GetMovie(id int64) (*Movie, error) {
	// The PostgreSQL bigserial type that we're using for the movie ID starts
    // auto-incrementing at 1 by default, so we know that no movies will have ID values
    // less than that. To avoid making an unnecessary database call, we take a shortcut
    // and return an ErrRecordNotFound error straight away.
    if id < 1 {
        return nil, ErrRecordNotFound
    }
	// Create a movie variable where we will copy the result of
	// the db query into.
	var movie Movie
	query := `
		SELECT * FROM movies WHERE id = $1
	`
	rowPtr := movieModel.DBPtr.QueryRow(
		query, 
		id,
	)
	// scan the response data into the fields of the  Movie struct. 
	// Importantly, notice that we need to convert the scan target for the 
    // genres column using the pq.Array() adapter function again.
	// which was used in the insert function on the genres column
	err := rowPtr.Scan(       
		&movie.ID,
        &movie.CreatedAt,
        &movie.Title,
        &movie.Year,
        &movie.Runtime,
        pq.Array(&movie.Genres),
        &movie.Version,
	)

    // Handle any errors. If there was no matching movie found, Scan() will return 
    // a sql.ErrNoRows error. We check for this and return our custom ErrRecordNotFound 
    // error instead.
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &movie, nil
}

/*
UPDATE MOVIE - Update a movie in the database, return an error
should the operation fail. This accepts a movie argument,
this argument will however be different to that we pass in during
create, this movie argument MUST contain an ID, as well as the fields
we want to update, that is, some fields can be empty. However, in the argument to
Insert, the movie we pass will not contain an ID and must contain all the arguments
in order not to violate the NOT NULL constraints we have in our database.
*/
func (movieModel MovieModel) Update(movie *Movie) error {
    return nil
}

/*
DELETE MOVIE - Delete a movie from the database, given the ID
return an error should the operation fail. Might redesign this to include
the deleted movie as well.
*/
func (movieModel MovieModel) Delete(id int64) error {
    return nil
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