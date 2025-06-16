package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"greenlight-movie-api/internal/validator"
	"time"

	"github.com/lib/pq"
)

/*********************************************************************************************************************/
//Allowed Genre Values
var AllowedGenres = []string{"adventure", "action", "animation", "romance", "comedy", "history", "drama", "sci-fi"}

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
Use pointers for the Title, Year and Runtime fields. This will be 
nil if there is no corresponding key in the JSON. We don't need 
pointers for the Genres field, because slices already have the 
zero-value nil.
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
func (movieModel MovieModel) InsertMovie(moviePtr *Movie) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3 * time.Second)
	defer cancelFunc()

	rowPtr := movieModel.DBPtr.QueryRowContext(
		ctx,
		`
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

	ctx,cancelFunc := context.WithTimeout(context.Background(), (3 * time.Second))
	defer cancelFunc()

	rowPtr := movieModel.DBPtr.QueryRowContext(
		ctx,
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
UPDATE MOVIE - UpdateMovie a movie in the database, return an error should the operation 
fail. This accepts a movie argument, this argument will however be different to that we 
pass in during create, this movie argument MUST contain an ID, (we know that it will 
infact surely contain an ID, cos the movie we're passing in will be the movie we got from 
the db, from calling GETMOVIE prior, using the id passed from the request). All errors 
with gettinig the movie successfully from the db are handled upstream from this stage, 
so that whatever movie we do pass to the UpdateMovie function is coming fresh from the DB 
and will indeed contain an ID, which we use to update the specific movie in the DB. 

However, in the argument to Insert, the movie we pass will not contain an ID and must 
contain all the arguments in order not to violate the NOT NULL constraints we have in 
our database.
*/
func (movieModel MovieModel) UpdateMovie(moviePtr *Movie) error {
	//query to update required fields, we return * from this query
	//because we'll be using the method QueryRow, which requires
	//that we return one row of results at least
    query := `
		UPDATE movies
		SET title = $1, year = $2, 
		runtime = $3, genres = $4,
		version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING *
	`

	ctx, cancelFunc := context.WithTimeout(context.Background(), 3 * time.Second)
	defer cancelFunc()
	//execute the query with the appropriate arguments, notice that
	//we're also updating the version by 1 from the previuos value
	//this will happen everytime we update a resource
	//Refer notes(1) for notes on optimistic concurrency
	rowPtr := movieModel.DBPtr.QueryRowContext(
		ctx,
		query, 
		moviePtr.Title, 
		moviePtr.Year, 
		moviePtr.Runtime, 
		pq.Array(moviePtr.Genres),
		moviePtr.ID,
		moviePtr.Version,
	)

	//Scan the row into the moviePtr and handle any potential errors
	err := rowPtr.Scan(       
		&moviePtr.ID,
        &moviePtr.CreatedAt,
        &moviePtr.Title,
        &moviePtr.Year,
        &moviePtr.Runtime,
        pq.Array(&moviePtr.Genres),
        &moviePtr.Version,
	)
	
	//I must say it seems absurt to think that an errRecordNotFound
	//error will ever be returned, given the fact that the id that
	//was used for the update operation was provided by the DB itself
	if err != nil {
		switch {
			case errors.Is(err, sql.ErrNoRows):
				return ErrEditConflict
			default:
				return err
		}
	}

	return nil
}

/*
DELETE MOVIE - Delete a movie from the database, given the ID
return an error should the operation fail. Might redesign this to include
the deleted movie as well.
*/
func (movieModel MovieModel) Delete(id int64) (*Movie, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}
	query := `
		DELETE FROM movies WHERE id = $1
		RETURNING id, title, year, runtime, genres
	`

	var deletedMovie Movie

	ctx, cancelFunc := context.WithTimeout(context.Background(), 3 * time.Second)
	defer cancelFunc()

	err := movieModel.DBPtr.QueryRowContext(ctx, query, id).Scan(
		&deletedMovie.ID, 
		&deletedMovie.Title, 
		&deletedMovie.Year,
		&deletedMovie.Runtime,
		pq.Array(&deletedMovie.Genres),
	)

	fmt.Println(err)
	if err != nil {
		switch {
			case errors.Is(err, sql.ErrNoRows):
				return nil, ErrRecordNotFound
			default:
				return nil, err
		}
	}
    return &deletedMovie, nil
}

//filters Filters - pass this in later.
func (movieModel MovieModel) GetAllMovies(title string, genres []string) ([]*Movie, error) {
	query := 
	`SELECT * FROM movies 
	WHERE (LOWER(title) = LOWER($1) OR $1 = '')
	AND (genres @> $2 or $2 = '{}')
	ORDER BY id`

	ctx, cancelFunc := context.WithTimeout(context.Background(), 3 * time.Second)
	defer cancelFunc()

	movieRows, err := movieModel.DBPtr.QueryContext(ctx, query, title, pq.Array(genres))
	if err != nil {
		return nil, err
	}
	// Importantly, defer a call to rows.Close() to ensure that the resultset is closed
    // before GetAll() returns.
	defer movieRows.Close()

	moviePtrs := []*Movie{}

	//check and prepare a next row for reading; must be called even before
	//the first scan
	for movieRows.Next() {
		var movie Movie
		//scan the current row into a movie struct
		err := movieRows.Scan( 
			&movie.ID, &movie.CreatedAt, &movie.Title, &movie.Year,
        	&movie.Runtime, pq.Array(&movie.Genres), &movie.Version,
		)
		//return if an error is encountered
		if err != nil {
			return nil, err
		}
		//if no error, append the movie to the movies slice and continue
		moviePtrs = append(moviePtrs, &movie)
	}

    // When the rows.Next() loop has finished, call rows.Err() to retrieve any error 
    // that was encountered during the iteration.
	if err := movieRows.Err(); err != nil {
		return nil, err
	}
	
	// If everything went OK, then return the slice of movies.
	return moviePtrs, nil
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
		if !validator.PermittedValue(genre, AllowedGenres...) {
			movieValidatorPtr.AddError(
				"genres", 
				fmt.Sprintf("must not contain values aside the following: %+v", AllowedGenres),
			)
			return
		}
	}
}

/*********************************************************************************************************************/
/*
NOTES:
1 - OPTIMISTIC CONCURRENCY IN UPDATES
Due to the behaviour of go in processing requests, if the server receives two separate requests to update the same
resource at the same time, we will end up with a race condition because both requests will try and update the 
resource without any synchronization refer to ch8.2 Let's Go further for further explanation. What we can do is,
since we increment the version number with every update, this tells us if the data has been updated since the time
we read it last from the DB, in that case, we only allow an update operation to pass if the version number in the
DB is the same as the version number of the movie in the update operation (i.e we have not updated the movie since the
last time we read it from the DB). Otherwise, if the version number is not the same, we know that another update
operation has occurred from the last time we read from the DB and we are working with a stale version of the DB data,
hence, we want this update operation to fail.
*/