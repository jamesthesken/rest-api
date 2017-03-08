package routers

import (
	"github.com/UHERO/rest-api/controllers"
	"github.com/UHERO/rest-api/data"
	"github.com/gorilla/mux"
)

func SetCategoryRoutes(
	router *mux.Router,
	categoryRepository *data.CategoryRepository,
	seriesRepository *data.SeriesRepository,
	cacheRepository *data.CacheRepository,
) *mux.Router {
	router.HandleFunc("/v1/category", controllers.GetCategory(categoryRepository, cacheRepository)).Methods("GET").Queries(
		"id", "{id:[0-9]+}",
	)
	router.HandleFunc("/v1/category", controllers.GetCategoriesByName(categoryRepository, cacheRepository)).Methods("GET").Queries(
		"search_text", "{searchText:.+}",
	)
	router.HandleFunc("/v1/category", controllers.GetCategoryRoots(categoryRepository, cacheRepository)).Methods("GET").Queries(
		"top_level", "true",
	)
	router.HandleFunc("/v1/category", controllers.GetCategories(categoryRepository, cacheRepository)).Methods("GET").Queries(
		"top_level", "false",
	)
	router.HandleFunc("/v1/category", controllers.GetCategories(categoryRepository, cacheRepository)).Methods("GET")
	router.HandleFunc("/v1/category/freq", controllers.GetFreqByCategoryId(seriesRepository, cacheRepository)).Methods("GET").Queries(
		"id", "{id:[0-9]+}",
	)
	router.HandleFunc("/v1/category/series", controllers.GetInflatedSeriesByCategoryIdGeoAndFreq(seriesRepository, cacheRepository)).Methods("GET").Queries(
		"id", "{id:[0-9]+}",
		"geo", "{geo:[A-Za-z0-9]+}",
		"freq", "{freq:[ASQMWDasqmwd]}",
		"expand", "true",
	)
	router.HandleFunc("/v1/category/series", controllers.GetSeriesByCategoryIdGeoHandleAndFreq(seriesRepository, cacheRepository)).Methods("GET").Queries(
		"id", "{id:[0-9]+}",
		"geo", "{geo:[A-Za-z0-9]+}",
		"freq", "{freq:[ASQMWDasqmwd]}",
	)
	router.HandleFunc("/v1/category/series", controllers.GetSeriesByCategoryIdAndGeoHandle(seriesRepository, cacheRepository)).Methods("GET").Queries(
		"id", "{id:[0-9]+}",
		"geo", "{geo:[A-Za-z0-9]+}",
	)
	router.HandleFunc("/v1/category/series", controllers.GetSeriesByCategoryIdAndFreq(seriesRepository, cacheRepository)).Methods("GET").Queries(
		"id", "{id:[0-9]+}",
		"freq", "{freq:[ASQMWDasqmwd]}",
	)
	router.HandleFunc("/v1/category/series", controllers.GetInflatedSeriesByCategoryId(seriesRepository, cacheRepository)).Methods("GET").Queries(
		"id", "{id:[0-9]+}",
		"expand", "true",
	)
	router.HandleFunc("/v1/category/series", controllers.GetSeriesByCategoryId(seriesRepository, cacheRepository)).Methods("GET").Queries("id", "{id:[0-9]+}")
	return router
}
