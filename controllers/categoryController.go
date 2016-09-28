package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/uhero/rest-api/common"
	"github.com/uhero/rest-api/data"
	"log"
)

// ReadApplications returns a handler that returns all of the user's applications
func GetCategories(categoryRepository *data.CategoryRepository) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("Inside of GetCategories")
		categories, err := categoryRepository.GetAllCategories()
		if err != nil {
			common.DisplayAppError(
				w,
				err,
				"An unexpected error has occurred",
				500,
			)
			return
		}
		j, err := json.Marshal(CategoriesResource{Data: categories})
		if err != nil {
			common.DisplayAppError(
				w,
				err,
				"An unexpected error processing JSON has occurred",
				500,
			)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(j)
	}
}
