package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

type Location struct {
	Deployment string  `json:"deployment"`
	Platform   string  `json:"platform"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Timestamp  string  `json:"timestamp"`
	Source     string  `json:"source"`
}

var db *sql.DB

func initDB() error {
	var err error
	dbPath := filepath.Join("data", "locations.db")
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("error creating data directory: %v", err)
	}

	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS locations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		deployment TEXT NOT NULL,
		platform TEXT NOT NULL,
		latitude REAL NOT NULL,
		longitude REAL NOT NULL,
		timestamp TEXT NOT NULL,
		source TEXT NOT NULL
	);
	`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("error creating table: %v", err)
	}

	return nil
}

func main() {
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// API routes
	r.POST("/locations", handlePostLocation)
	r.GET("/locations", handleGetLocations)
	r.GET("/deployments", handleGetDeployments)
	r.GET("/platforms/:deployment", handleGetPlatforms)
	r.GET("/download_kml/:deployment", handleDownloadKML)

	// Get host and port from environment variables
	host := os.Getenv("API_HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	addr := fmt.Sprintf("%s:%s", host, port)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handlePostLocation(c *gin.Context) {
	var loc Location
	if err := c.ShouldBindJSON(&loc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stmt, err := db.Prepare("INSERT INTO locations (deployment, platform, latitude, longitude, timestamp, source) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(loc.Deployment, loc.Platform, loc.Latitude, loc.Longitude, loc.Timestamp, loc.Source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func handleGetLocations(c *gin.Context) {
	deployment := c.Query("deployment")
	platform := c.Query("platform")

	query := "SELECT deployment, platform, latitude, longitude, timestamp, source FROM locations"
	var args []interface{}
	if deployment != "" {
		query += " WHERE deployment = ?"
		args = append(args, deployment)
		if platform != "" {
			query += " AND platform = ?"
			args = append(args, platform)
		}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var locations []Location
	for rows.Next() {
		var loc Location
		if err := rows.Scan(&loc.Deployment, &loc.Platform, &loc.Latitude, &loc.Longitude, &loc.Timestamp, &loc.Source); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		locations = append(locations, loc)
	}

	c.JSON(http.StatusOK, locations)
}

func handleGetDeployments(c *gin.Context) {
	rows, err := db.Query("SELECT DISTINCT deployment FROM locations")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var deployments []string
	for rows.Next() {
		var deployment string
		if err := rows.Scan(&deployment); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		deployments = append(deployments, deployment)
	}

	c.JSON(http.StatusOK, deployments)
}

func handleGetPlatforms(c *gin.Context) {
	deployment := c.Param("deployment")
	rows, err := db.Query("SELECT DISTINCT platform FROM locations WHERE deployment = ?", deployment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var platforms []string
	for rows.Next() {
		var platform string
		if err := rows.Scan(&platform); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		platforms = append(platforms, platform)
	}

	c.JSON(http.StatusOK, platforms)
}

func handleDownloadKML(c *gin.Context) {
	deployment := c.Param("deployment")
	rows, err := db.Query("SELECT platform, latitude, longitude, timestamp FROM locations WHERE deployment = ? ORDER BY timestamp", deployment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Create KML structure
	kml := `<?xml version="1.0" encoding="UTF-8"?>
<kml xmlns="http://www.opengis.net/kml/2.2">
  <Document>
    <name>%s Track</name>
    <Style id="yellowLineGreenPoly">
      <LineStyle>
        <color>7f00ffff</color>
        <width>4</width>
      </LineStyle>
      <PolyStyle>
        <color>7f00ff00</color>
      </PolyStyle>
    </Style>`

	// Group locations by platform
	platformData := make(map[string][]struct {
		Latitude  float64
		Longitude float64
		Timestamp string
	})

	for rows.Next() {
		var platform string
		var lat, lon float64
		var timestamp string
		if err := rows.Scan(&platform, &lat, &lon, &timestamp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		platformData[platform] = append(platformData[platform], struct {
			Latitude  float64
			Longitude float64
			Timestamp string
		}{lat, lon, timestamp})
	}

	// Add placemarks for each platform
	for platform, points := range platformData {
		kml += fmt.Sprintf(`
    <Placemark>
      <name>%s Track</name>
      <styleUrl>#yellowLineGreenPoly</styleUrl>
      <LineString>
        <extrude>1</extrude>
        <tessellate>1</tessellate>
        <altitudeMode>relativeToGround</altitudeMode>
        <coordinates>`, platform)

		// Add coordinates for the track
		for _, point := range points {
			kml += fmt.Sprintf("%f,%f,0 ", point.Longitude, point.Latitude)
		}

		kml += `</coordinates>
      </LineString>
    </Placemark>`

		// Add placemarks for individual points
		for _, point := range points {
			kml += fmt.Sprintf(`
    <Placemark>
      <name>%s</name>
      <TimeStamp>
        <when>%s</when>
      </TimeStamp>
      <Point>
        <coordinates>%f,%f,0</coordinates>
      </Point>
    </Placemark>`, point.Timestamp, point.Timestamp, point.Longitude, point.Latitude)
		}
	}

	kml += `
  </Document>
</kml>`

	c.Header("Content-Type", "application/vnd.google-earth.kml+xml")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_track.kml", deployment))
	c.String(http.StatusOK, kml)
}
