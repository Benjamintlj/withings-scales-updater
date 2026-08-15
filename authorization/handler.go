package authorization

import (
	"fmt"
	"log"
	"net/http"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	log.Println("endpoint hit")

	fmt.Fprintln(w, "<h1>hello rhiannon</h1>")

	w.WriteHeader(http.StatusOK)
}
