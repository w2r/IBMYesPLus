package main

import (
	"log"
	"net/http"
	"os"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

import "github.com/timjacobi/go-couchdb"

type Visitor struct {
	Name string `json:"name"`
}

type Visitors []Visitor

type alldocsResult struct {
	TotalRows int `json:"total_rows"`
	Offset    int
	Rows      []map[string]interface{}
}

func main() {
	r := gin.Default()

	r.StaticFile("/", "./static/index.html")
	r.Static("/static", "./static")

	var dbName = "mydb"

	//When running locally, get credentials from .env file.
	err := godotenv.Load()
	if err != nil {
		log.Println(".env file does not exist")
	}
	cloudantUrl := os.Getenv("CLOUDANT_URL")

	appEnv, _ := cfenv.Current()
	if appEnv != nil {
		cloudantService, _ := appEnv.Services.WithLabel("cloudantNoSQLDB")
		if len(cloudantService) > 0 {
			cloudantUrl = cloudantService[0].Credentials["url"].(string)
		}
	}

	cloudant, err := couchdb.NewClient(cloudantUrl, nil)
	if err != nil {
		log.Println("Can not connect to Cloudant database")
	}

	//ensure db exists
	//if the db exists the db will be returned anyway
	cloudant.CreateDB(dbName)

	/* Endpoint to greet and add a new visitor to database.
	* Send a POST request to http://localhost:8080/api/visitors with body
	* {
	* 	"name": "Bob"
	* }
	 */
	r.POST("/api/visitors", func(c *gin.Context) {
		var visitor Visitor
		if c.BindJSON(&visitor) == nil {
			cloudant.DB(dbName).Post(visitor)
			c.String(200, "Hello "+visitor.Name)
		}
	})

	/**
	 * Endpoint to get a JSON array of all the visitors in the database
	 * REST API example:
	 * <code>
	 * GET http://localhost:8080/api/visitors
	 * </code>
	 *
	 * Response:
	 * [ "Bob", "Jane" ]
	 * @return An array of all the visitor names
	 */
	r.GET("/api/visitors", func(c *gin.Context) {
		var result alldocsResult
		if cloudantUrl == "" {
			c.JSON(200, gin.H{})
			return
		}
		err := cloudant.DB(dbName).AllDocs(&result, couchdb.Options{"include_docs": true})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to fetch docs"})
		} else {
			c.JSON(200, result.Rows)
		}
	})

	//When running on Cloud Foundry, get the PORT from the environment variable.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" //Local
	}
	r.Run(":" + port)
}
