# Data Gateway Service

The Data Gateway service is a central component of REEF that handles data ingestion from post op log files.

## Data Flow

1. Platforms can send data to the Data Gateway via HTTP POST requests or the archive-proccessor can scan and sublit archived data
2. The gateway validates incoming data format
3. Valid submissions are send to the db for access by the report generation tools

## API Endpoints

### POST /api/data
Accepts data from robotic platforms in the following format:

```json
{
    "deployment": "string",      // Deployment identifier
    "platform": "string",        // Platform identifier
    "timestamp": "string",       // ISO 8601 timestamp
    "latitude": float64,         // GPS latitude
    "longitude": float64,        // GPS longitude
    "data": {                    // Platform-specific data
        // Additional fields as needed
    }
}
```

### GET /api/locations
Returns location history for visualization:
```json
[
    {
        "platform": "string",
        "timestamp": "string",
        "latitude": float64,
        "longitude": float64
    }
]
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| API_HOST | HTTP server host | 0.0.0.0 |
| API_PORT | HTTP server port | 8080 |
| MONGODB_URI | MongoDB connection string | mongodb://mongodb:27017 |
| MONGODB_DATABASE | Database name | robotics |
| MONGODB_COLLECTION | Collection name | robot_data |

## Development

### Prerequisites
- Go 1.21 or later
- Docker and Docker Compose

### Building
```bash
docker build -t data-gateway .
```

### Running
```bash
docker-compose up
```
