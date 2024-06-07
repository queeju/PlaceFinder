# PlaceFinder

PlaceFinder is a Golang-based web application designed to index, search, and display information about various places using Elasticsearch. The application provides a simple HTML interface to browse places and a JSON API for programmatic access, along with JWT authentication for secure API access. 
This project was developed as a part of School 21 curriculum.

## Description

PlaceFinder is designed to index a large dataset of places (such as restaurants) into Elasticsearch and provide an easy-to-use interface to browse and search through them. The project demonstrates the integration of Golang with Elasticsearch for full-text search capabilities and provides a simple HTML interface for viewing the data, along with a JSON API for accessing the data programmatically.

## Prerequisites

Before you begin, ensure you have met the following requirements:

-   **Golang** v 1.21
-   **Elasticsearch** v 8.13
-   **Taskfile** v3.37

## Usage

1.  Add Elasticsearch directory to the Taskfile (vars:es_dir)
2.  Start Elasticsearch
	`task start-es`
3.  In a separate terminal, build the application
	`task build`
4. If you are running the app for the first time, you need to setup the database:
	`./PlaceFinder -s`
	`curl -XPUT -H "Content-Type: application/json" "http://localhost:9200/places/_settings" -d '{ "index" : { "max_result_window" : 20000 }}'`
5. Run the app and see search results via browser or curl
	- HTML interface
		http://localhost:8888
	- API interface
		http://localhost:8888/api/places?page=1 
	- Recommendations
		http://localhost:8888/api/recommend?lat=55.797129&lon=37.579789
	
6. To enable authentication, run the app with flag -a:
	`./PlaceFinder -a`
	Obtain a JWT token
	http://localhost:8888/api/get_token
	Get recommendations via curl
	`curl -X GET -H "Authorization: Bearer your.token.here" http://localhost:8888/api/recommend?lat=55.797129&lon=37.579789`
