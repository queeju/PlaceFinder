package db

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"

	"day03es/types"
)

// Limit of recommended places
const recLimit = 3

// Collection of search results
var AllPlaces []Place

// ElasticStore implements the Store interface using Elasticsearch.
type ElasticStore struct {
	client *elasticsearch.Client
}

// NewElasticStore creates a new ElasticStore instance.
func NewElasticStore() *ElasticStore {
	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating the Elasticsearch client: %s", err)
	}
	return &ElasticStore{client: client}
}

// CreateIndex creates the Elasticsearch index.
func (s *ElasticStore) CreateIndex(indName string) {
	// Check if the index exists
	res, err := s.client.Indices.Exists([]string{indName})
	if err != nil {
		log.Fatalf("Error checking index existence: %s", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		fmt.Println("Index already exists.")
		return
	} else if res.StatusCode != http.StatusNotFound {
		log.Fatalf("Unexpected status code: %d", res.StatusCode)
	}

	req := esapi.IndicesCreateRequest{
		Index: indName,
	}

	res, err = req.Do(context.Background(), s.client)
	if err != nil {
		log.Fatalf("Error creating index: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Fatalf("Error creating index: %s", res.String())
	}

	fmt.Printf("Index '%s' created successfully.", indName)
}

// ApplyMapping applies the mapping to the index.
func (s *ElasticStore) ApplyMapping() {
	// Prepare the mapping schema
	mapping := `
	{
	  "properties": {
	    "name": {
	        "type":  "text"
	    },
	    "address": {
	        "type":  "text"
	    },
	    "phone": {
	        "type":  "text"
	    },
	    "location": {
	      "type": "geo_point"
	    }
	  }
	}
	`

	// Prepare the mapping request
	mappingReq := esapi.IndicesPutMappingRequest{
		Index: []string{"places"},
		Body:  strings.NewReader(mapping),
	}

	// Send the mapping request
	res, err := mappingReq.Do(context.Background(), s.client)
	if err != nil {
		log.Fatalf("Error applying mapping: %s", err)
	}
	defer res.Body.Close()

	// Handle the response
	if res.IsError() {
		log.Fatalf("Error applying mapping: %s", res.String())
	}

	fmt.Println("Mapping applied successfully.")
}

// AddData adds data to the index.
func (s *ElasticStore) AddData(path string) uint64 {
	var countSuccessful uint64
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Error opening CSV file: %s", err)
	}
	defer file.Close()

	// Create a CSV reader
	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = '\t'

	// Skip the header row
	_, err = reader.Read()
	if err != nil {
		log.Fatalf("Error reading CSV header: %s", err)
	}

	// Create the BulkIndexer
	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:         "places",         // index name
		Client:        s.client,         // Elasticsearch client
		NumWorkers:    2,                // The number of worker goroutines
		FlushBytes:    1024 * 1024,      // The flush threshold in bytes
		FlushInterval: 30 * time.Second, // The periodic flush interval
	})
	if err != nil {
		log.Fatalf("Error creating the indexer: %s", err)
	}

	// Read and parse the CSV file
	for {
		record, err := reader.Read()
		if err != nil {
			break // End of file
		}

		// Construct a JSON object from the CSV record
		data := map[string]interface{}{
			"name":     record[1],
			"address":  record[2],
			"phone":    record[3],
			"location": map[string]interface{}{"lat": record[5], "lon": record[4]},
		}

		// Encode the JSON object
		jsonData, err := json.Marshal(data)
		if err != nil {
			log.Fatalf("Error encoding JSON: %s", err)
		}

		// Add an item to the BulkIndexer
		err = bi.Add(
			context.Background(),
			esutil.BulkIndexerItem{
				Action:     "index",
				DocumentID: record[0],
				Body:       bytes.NewReader(jsonData),
				// OnSuccess is called for each successful operation
				OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
					atomic.AddUint64(&countSuccessful, 1)
				},
				// OnFailure is called for each failed operation
				OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
					if err != nil {
						log.Printf("ERROR: %s", err)
					} else {
						log.Printf("ERROR: %s: %s", res.Error.Type, res.Error.Reason)
					}
				},
			},
		)
		if err != nil {
			log.Fatalf("Unexpected error: %s", err)
		}
	}

	// Close the indexer
	if err := bi.Close(context.Background()); err != nil {
		log.Fatalf("Unexpected error: %s", err)
	}
	biStats := bi.Stats()

	if biStats.NumFailed > 0 {
		log.Fatalf("Indexed [%d] documents with [%d] errors", biStats.NumFlushed, biStats.NumFailed)
	} else {
		log.Printf("Sucessfuly indexed [%d] documents", biStats.NumFlushed)
	}
	return countSuccessful
}

// Fetch all places from the index
func (s *ElasticStore) GetAllPlaces() ([]Place, error) {
	// Prepare the query
	query := map[string]interface{}{
		"query": map[string]interface{}{"match_all": struct{}{}},
	}

	// Encode the query as JSON
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		log.Println("GetAllPlaces:", err)
		return nil, err
	}

	// Execute the search request
	res, err := s.client.Search(
		s.client.Search.WithContext(context.Background()),
		s.client.Search.WithIndex("places"),
		s.client.Search.WithBody(&buf),
		s.client.Search.WithSize(20000),
	)
	if err != nil {
		log.Println("GetAllPlaces:", err)
		return nil, err
	}
	defer res.Body.Close()

	// Parse the response
	var result struct {
		Hits struct {
			Total struct {
				Value    int    `json:"value"`
				Relation string `json:"relation"`
			} `json:"total"`
			MaxScore float64 `json:"max_score"`
			Hits     []Place `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Println("GetAllPlaces:", err)
		return nil, err
	}

	return result.Hits.Hits, nil
}

// Get a page of results
func (s *ElasticStore) GetPlaces(limit int, offset int) ([]Place, int, error) {
	if len(AllPlaces) == 0 {
		var err error
		AllPlaces, err = s.GetAllPlaces()
		if err != nil {
			log.Println("GetPlaces:", err)
			return nil, 0, err
		}
	}

	if offset < 0 {
		return nil, 0, types.ErrInvalidPage
	}

	ln := len(AllPlaces)
	if offset >= ln {
		return nil, 0, types.ErrInvalidPage
	}
	if offset+limit > ln {
		limit = ln - offset
	}
	if limit < 0 {
		return nil, 0, types.ErrInvalidPage
	}
	res := AllPlaces[offset : offset+limit]
	return res, ln, nil
}

func (es *ElasticStore) GetRecommended(lat, lon float64) ([]types.RecPlace, error) {
	// Define the Elasticsearch query for searching three closest restaurants
	query := map[string]interface{}{
		"size": recLimit,
		"sort": []map[string]interface{}{
			{
				"_geo_distance": map[string]interface{}{
					"location": map[string]interface{}{
						"lat": lat,
						"lon": lon,
					},
					"order":           "asc",
					"unit":            "km",
					"mode":            "min",
					"distance_type":   "arc",
					"ignore_unmapped": true,
				},
			},
		},
	}

	// Encode the query as JSON
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, err
	}

	// Execute the Elasticsearch query
	res, err := es.client.Search(
		es.client.Search.WithContext(context.Background()),
		es.client.Search.WithIndex("places"),
		es.client.Search.WithBody(&buf),
	)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Parse the response JSON
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Extract the recommended places from the response
	places := make([]types.RecPlace, 0)

	hits, _ := result["hits"].(map[string]interface{})["hits"].([]interface{})
	for _, hit := range hits {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})
		lat, err := strconv.ParseFloat(source["location"].(map[string]interface{})["lat"].(string), 64)
		if err != nil {
			continue
		}

		lon, err := strconv.ParseFloat(source["location"].(map[string]interface{})["lon"].(string), 64)
		if err != nil {
			continue
		}
		location := types.Location{
			Lat: lat,
			Lon: lon,
		}

		id, err := strconv.Atoi(hit.(map[string]interface{})["_id"].(string))
		if err != nil {
			continue
		}

		place := types.RecPlace{
			ID:       id,
			Name:     source["name"].(string),
			Address:  source["address"].(string),
			Phone:    source["phone"].(string),
			Location: location,
		}

		places = append(places, place)
	}

	return places, nil
}
