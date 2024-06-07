package types

import "errors"

// Structure to represent a place for recomendations page
type RecPlace struct {
	ID       int
	Name     string
	Address  string
	Phone    string
	Location Location
}

type Location struct {
	Lat, Lon float64
}

var ErrInvalidPage = errors.New("Invalid page value")
