package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"day03es/db"
	"day03es/types"
)

// struct to represent a place in the HTML template
type PlaceHTML struct {
	Name    string
	Address string
	Phone   string
}

// HTML template
const htmlTemplate = `
<!doctype html>
<html>
<head>
	<meta charset="utf-8">
	<title>Places</title>
	<meta name="description" content="">
	<meta name="viewport" content="width=device-width, initial-scale=1">
</head>

<body>
<h5>Total: {{.Total}}</h5>
<ul>
	{{range .Places}}
	<li>
			<div>{{.Name}}</div>
			<div>{{.Address}}</div>
			<div>{{.Phone}}</div>
	</li>
	{{end}}
</ul>
<div>
    <a href="/?page=1">First</a>
    {{if .PrevPage}}
    <a href="/?page={{.PrevPage}}">Previous</a>
    {{end}}
    {{if ne .NextPage 0}}
    <a href="/?page={{.NextPage}}">Next</a>
    {{end}}
    <a href="/?page={{.TotalPages}}">Last</a>
</div>
</body>
</html>
`

// Number of places per page
const limit = 10

func CreateServer(store db.Store, fAuth bool) error {

	// Define a handler function to handle incoming HTTP requests
	mainHandler := func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			JSONHandler(store)(w, r)
		} else {
			HTMLHandler(store)(w, r)
		}
	}

	// Different recHandler func with or without authentification
	var recommendHandler http.HandlerFunc
	if fAuth {
		recommendHandler = validateToken(recHandler(store))
		http.HandleFunc("/api/get_token", getTokenHandler)
	} else {
		recommendHandler = recHandler(store)
	}

	// Register the handler function with the default ServeMux
	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/api/recommend", recommendHandler)

	// Start the HTTP server and listen for incoming requests on port 8888
	fmt.Println("Server is running on port 8888...")

	return http.ListenAndServe(":8888", nil)
}

func recHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse latitude and longitude parameters from the request URL
		latParam := r.URL.Query().Get("lat")
		lonParam := r.URL.Query().Get("lon")
		var (
			lat, lon float64
			err      error
		)

		// Convert latitude and longitude parameters to float64
		if latParam == "" {
			lat = 55.797129
		} else {
			lat, err = strconv.ParseFloat(latParam, 64)
			if err != nil {
				http.Error(w, "Invalid 'lat' parameter", http.StatusBadRequest)
				return
			}
		}

		if lonParam == "" {
			lon = 37.579789
		} else {
			lon, err = strconv.ParseFloat(lonParam, 64)
			if err != nil {
				http.Error(w, "Invalid 'lon' parameter", http.StatusBadRequest)
				return
			}
		}

		// Get recommended places via ES query
		places, err := store.GetRecommended(lat, lon)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Construct the response JSON
		response := map[string]interface{}{
			"name":   "Recommendation",
			"places": places,
		}

		// Set the Content-Type header to application/json
		w.Header().Set("Content-Type", "application/json")

		// Encode the response JSON and write it to the response writer
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

func JSONHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		page, _, places, totalPlaces, err := handlerHelper(r, store)

		if err == types.ErrInvalidPage {
			errMsg := fmt.Sprintf("Invalid page value: '%d'", page)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		} else if err != nil {
			errMsg := fmt.Sprintf("Invalid page value")
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		// Prepare the response data in JSON format
		response := map[string]interface{}{
			"name":   "Places",
			"total":  totalPlaces,
			"places": placesToJSON(places),
		}

		// Encode the response data as JSON with indentation
		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// Convert places to JSON format
func placesToJSON(places []db.Place) []map[string]interface{} {
	result := make([]map[string]interface{}, len(places))
	for i, place := range places {
		lat, err := strconv.ParseFloat(place.Source.Location.Lat, 64)
		if err != nil {
			continue
		}
		lon, err := strconv.ParseFloat(place.Source.Location.Lon, 64)
		if err != nil {
			continue
		}
		result[i] = map[string]interface{}{
			"id":      place.ID,
			"name":    place.Source.Name,
			"address": place.Source.Address,
			"phone":   place.Source.Phone,
			"location": map[string]float64{
				"lat": lat,
				"lon": lon,
			},
		}
	}
	return result
}

func HTMLHandler(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, totalPages, places, totalPlaces, err := handlerHelper(r, store)

		// Check if the page value is within the valid range
		if err == types.ErrInvalidPage {
			errMsg := fmt.Sprintf("Invalid page value: '%d'", page)
			http.Error(w, errMsg, http.StatusBadRequest)
			log.Println("HTMLHandler 1:", err)
			return
		} else if err != nil {
			errMsg := fmt.Sprintf("Invalid page value")
			http.Error(w, errMsg, http.StatusBadRequest)
			log.Println("HTMLHandler 2:", err)
			return
		}

		// Render the HTML response with the list of places and pagination links
		renderHTMLResponse(w, places, totalPlaces, page, totalPages)
	}
}

func handlerHelper(r *http.Request, store db.Store) (int, int, []db.Place, int, error) {
	page, err := getPageFromRequest(r)

	if err != nil {
		log.Println("handleHelper 1:", err)
		return page, 0, nil, 0, err
	}

	var (
		places      []db.Place
		totalPlaces int
	)

	places, totalPlaces, err = store.GetPlaces(limit, (page-1)*limit)
	if err != nil && err != types.ErrInvalidPage {
		log.Println("handleHelper 2:", err)
		return page, 0, nil, 0, err
	}

	// Calculate pagination information
	totalPages := int(math.Ceil(float64(totalPlaces) / float64(limit)))
	return page, totalPages, places, totalPlaces, nil
}

// Extract page number from request URL
func getPageFromRequest(r *http.Request) (int, error) {
	pageParam := r.URL.Query().Get("page")
	if pageParam == "" {
		return 1, nil
	}

	// Convert the page parameter to an integer
	page, err := strconv.Atoi(pageParam)

	if page < 0 {
		log.Printf("getPageFromRequest: page = %d\n", page)
		return page, types.ErrInvalidPage
	}
	if err != nil {
		log.Println("getPageFromRequest:", err)
		return 0, err
	}

	return page, nil
}

// RenderHTMLResponse generates HTML content with the list of places and pagination links
func renderHTMLResponse(w http.ResponseWriter, places []db.Place, totalPlaces, page, totalPages int) {
	// Create a slice to hold the place data for rendering in the HTML template
	placeHTMLs := make([]PlaceHTML, len(places))
	for i, place := range places {
		placeHTMLs[i] = PlaceHTML{
			Name:    place.Source.Name,
			Address: place.Source.Address,
			Phone:   place.Source.Phone,
		}
	}

	prevPage := page - 1
	nextPage := page + 1

	// Disable "Next" link on the last page
	if nextPage > totalPages {
		nextPage = 0
	}

	tmpl := template.Must(template.New("htmlTemplate").Parse(htmlTemplate))
	data := struct {
		Places     []PlaceHTML
		Total      int
		PageSize   int
		Current    int
		TotalPages int
		PrevPage   int
		NextPage   int
	}{
		Places:     placeHTMLs,
		Total:      totalPlaces,
		PageSize:   limit,
		Current:    page,
		TotalPages: totalPages,
		PrevPage:   prevPage,
		NextPage:   nextPage,
	}

	// Render the HTML content to the response writer
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
