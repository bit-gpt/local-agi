package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	models "github.com/mudler/LocalAGI/dbmodels"
)

var DB *gorm.DB

func ConnectDB() {
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASS")
	host := os.Getenv("DB_HOST")
	name := os.Getenv("DB_NAME")

	// Production-ready DSN with proper timeouts and charset
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&timeout=30s&readTimeout=30s&writeTimeout=30s&charset=utf8mb4&collation=utf8mb4_unicode_ci", user, pass, host, name)

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			NoLowerCase:   true, // preserve camelCase column names
		},
	})
	if err != nil {
		log.Fatal("Failed to connect to MySQL:", err)
	}

	// Get underlying sql.DB for connection pool configuration
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Failed to get underlying sql.DB:", err)
	}

	// Configure connection pool for production
	sqlDB.SetMaxIdleConns(25)                 // Keep more idle connections ready
	sqlDB.SetMaxOpenConns(100)                // Allow more concurrent connections
	sqlDB.SetConnMaxLifetime(5 * time.Minute) // Shorter lifetime for better load balancing
	sqlDB.SetConnMaxIdleTime(2 * time.Minute) // Shorter idle time for resource efficiency

	if err := DB.AutoMigrate(&models.User{}, &models.Agent{}, &models.AgentMessage{}, &models.LLMUsage{}, &models.Character{}, &models.AgentState{}, &models.ActionExecution{}, &models.Reminder{}, &models.Observable{}, &models.H402PendingRequests{}); err != nil {
		log.Fatal("Migration failed:", err)
	}

	log.Println("Database connected successfully with connection pooling enabled")
}

// PingDB checks if the database connection is alive
func PingDB() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// EnsureConnection ensures the database connection is alive
func EnsureConnection() error {
	if err := PingDB(); err != nil {
		log.Printf("Database connection lost, attempting to reconnect: %v", err)
		// Try to reconnect
		ConnectDB()
		return PingDB()
	}
	return nil
}

// GetConnectionStats returns connection pool statistics for monitoring
func GetConnectionStats() (int, int, int) {
	sqlDB, err := DB.DB()
	if err != nil {
		return 0, 0, 0
	}
	stats := sqlDB.Stats()
	return stats.OpenConnections, stats.InUse, stats.Idle
}
