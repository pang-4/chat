package main

import(
	"log"
	"net/http"

	"github.com/pang-4/chat/internal/router"
	"github.com/pang-4/chat/internal/database"
)

func main(){

	db, err := database.Connect()
	if err != nil{
		log.Fatal(err)
	}

	defer db.Close()

	err = database.Migrate(db)
	if err != nil{
		log.Fatal(err)
	}

	router.SetupRoutes()

	log.Println("Server Start: 8080")

	err = http.ListenAndServe(":8080",nil)
	if err != nil{
		log.Fatal(err)
	}
}