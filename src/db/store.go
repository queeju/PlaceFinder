package db

import "day03es/types"

// Place represents a location entry.
type Place struct {
	Index  string  `json:"_index"`
	ID     string  `json:"_id"`
	Score  float64 `json:"_score"`
	Source Source  `json:"_source"`
}

type Source struct {
	Address  string `json:"address"`
	Location struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	} `json:"location"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// Store defines methods for interacting with the database.
type Store interface {
	// returns a list of items, a total number of hits and (or) an error in case of one
	GetPlaces(limit int, offset int) ([]Place, int, error)

	// returns a list of closest places based on specified location
	GetRecommended(lat, lon float64) ([]types.RecPlace, error)
}
