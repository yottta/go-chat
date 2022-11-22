package main

import (
	"github.com/yottta/chat/directory/app"
	httpx "github.com/yottta/chat/directory/infra/http"
	"log"
	"net/http"
)

func main() {
	clientsSvc := app.NewClientsSvc()
	app := app.App{
		Clients: clientsSvc,
	}
	handler := httpx.NewHandler(&app)
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
