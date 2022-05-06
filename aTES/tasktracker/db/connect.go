package db

import (
	"fmt"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Connection struct {
	*gorm.DB
}

var (
	host     = "psql"
	port     = 5432
	user     = "postgres"
	password = "password"
	dbname   = "postgres"
)

func Connect() Connection {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err := gorm.Open(postgres.Open(psqlInfo), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	fmt.Println(db.AutoMigrate(&Task{}))
	fmt.Println(db.AutoMigrate(&JiraAccount{}))
	fmt.Println("Successfully connected!")
	return Connection{db}
}
