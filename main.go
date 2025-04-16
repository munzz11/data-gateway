package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Location struct {
	Deployment string    `json:"deployment" bson:"deployment"`
	Platform   string    `json:"platform" bson:"platform"`
	Latitude   float64   `json:"latitude" bson:"latitude"`
	Longitude  float64   `json:"longitude" bson:"longitude"`
	Timestamp  string    `json:"timestamp" bson:"timestamp"`
	Source     string    `json:"source" bson:"source"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
}

var client *mongo.Client
var collection *mongo.Collection

func initDB() error {
	// Get MongoDB URI from environment variable or use default
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Set client options
	clientOptions := options.Client().ApplyURI(mongoURI)

	// Connect to MongoDB
	var err error
	client, err = mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return fmt.Errorf("error connecting to MongoDB: %v", err)
	}

	// Check the connection
	err = client.Ping(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("error pinging MongoDB: %v", err)
	}

	// Get collection
	dbName := os.Getenv("MONGODB_DATABASE")
	if dbName == "" {
		dbName = "robotics"
	}
	collectionName := os.Getenv("MONGODB_COLLECTION")
	if collectionName == "" {
		collectionName = "locations"
	}

	collection = client.Database(dbName).Collection(collectionName)

	// Create indexes
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "deployment", Value: 1},
			{Key: "platform", Value: 1},
			{Key: "timestamp", Value: 1},
		},
	}
	_, err = collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		return fmt.Errorf("error creating indexes: %v", err)
	}

	return nil
}

func handlePostLocation(c *gin.Context) {
	var location Location
	if err := c.ShouldBindJSON(&location); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	location.CreatedAt = time.Now()

	_, err := collection.InsertOne(context.Background(), location)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func handleGetLocations(c *gin.Context) {
	deployment := c.Query("deployment")
	platform := c.Query("platform")

	filter := bson.M{}
	if deployment != "" {
		filter["deployment"] = deployment
	}
	if platform != "" {
		filter["platform"] = platform
	}

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := collection.Find(context.Background(), filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(context.Background())

	var locations []Location
	if err = cursor.All(context.Background(), &locations); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, locations)
}

func handleGetDeployments(c *gin.Context) {
	deployments, err := collection.Distinct(context.Background(), "deployment", bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, deployments)
}

func handleGetPlatforms(c *gin.Context) {
	deployment := c.Param("deployment")
	platforms, err := collection.Distinct(context.Background(), "platform", bson.M{"deployment": deployment})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, platforms)
}

func main() {
	if err := initDB(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	r := gin.Default()
	r.POST("/api/data", handlePostLocation)
	r.GET("/api/locations", handleGetLocations)
	r.GET("/api/deployments", handleGetDeployments)
	r.GET("/api/platforms/:deployment", handleGetPlatforms)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}
