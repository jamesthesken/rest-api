package data

import (
	"database/sql"
	"errors"
	"github.com/UHERO/rest-api/models"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CategoryRepository struct {
	DB *sql.DB
}

func (r *CategoryRepository) GetNavCategories() (categories []models.Category, err error) {
	categories, err = r.GetNavCategoriesByUniverse("UHERO")
	return
}

func (r *CategoryRepository) GetNavCategoriesByUniverse(universe string) (categories []models.Category, err error) {
	rows, err := r.DB.Query(
		`SELECT categories.id,
			categories.name,
			categories.universe,
			categories.ancestry,
			categories.header,
			categories.default_freq AS catfreq,
			geographies.handle AS catgeo,
			geographies.fips AS catgeofips,
			geographies.display_name AS catgeoname,
			geographies.display_name_short AS catgeonameshort
		FROM categories
		LEFT JOIN geographies ON geographies.id = categories.default_geo_id
		WHERE NOT (categories.hidden OR categories.masked)
		AND categories.ancestry = CONVERT(
			(SELECT id from categories WHERE universe = ? AND ancestry IS NULL), CHAR)
		ORDER BY categories.list_order, COALESCE(geographies.list_order, 999), geographies.handle`, universe)
	if err != nil {
		return
	}
	for rows.Next() {
		category := models.CategoryWithAncestry{}
		err = rows.Scan(
			&category.Id,
			&category.Name,
			&category.Universe,
			&category.Ancestry,
			&category.IsHeader,
			&category.DefaultFrequency,
			&category.DefaultGeoHandle,
			&category.DefaultGeoFIPS,
			&category.DefaultGeoName,
			&category.DefaultGeoShortName,
		)
		if err != nil {
			return
		}
		parentId := getParentId(category.Ancestry)
		dataPortalCategory := models.Category{
			Id:       category.Id,
			Name:     category.Name,
			Universe: category.Universe,
			IsHeader: category.IsHeader,
			ParentId: parentId,
		}
		if category.DefaultFrequency.Valid || category.DefaultGeoHandle.Valid {
			// Only initialize Defaults struct if any defaults values are available
			dataPortalCategory.Defaults = &models.CategoryDefaults{}
		}
		if category.DefaultFrequency.Valid {
			dataPortalCategory.Defaults.Frequency = &models.DataPortalFrequency{
				Freq:  category.DefaultFrequency.String,
				Label: freqLabel[category.DefaultFrequency.String],
			}
		}
		if category.DefaultGeoHandle.Valid {
			dataPortalCategory.Defaults.Geography = &models.DataPortalGeography{
				Handle: category.DefaultGeoHandle.String,
			}
			if category.DefaultGeoFIPS.Valid {
				dataPortalCategory.Defaults.Geography.FIPS = category.DefaultGeoFIPS.String
			}
			if category.DefaultGeoName.Valid {
				dataPortalCategory.Defaults.Geography.Name = category.DefaultGeoName.String
			}
			if category.DefaultGeoShortName.Valid {
				dataPortalCategory.Defaults.Geography.ShortName = category.DefaultGeoShortName.String
			}
		}
		categories = append(categories, dataPortalCategory)
	}
	return
}

func (r *CategoryRepository) GetDefaultNavCategory(universe string) (category models.Category, err error) {
	categories, err := r.GetNavCategoriesByUniverse(universe)
	if err != nil {
		return
	}
	if len(categories) < 1 {
		err = errors.New("No categories found for universe " + universe)
		return
	}
	category = categories[0]
	return
}

func (r *CategoryRepository) GetAllCategories() (categories []models.Category, err error) {
	categories, err = r.GetAllCategoriesByUniverse("UHERO")
	return
}

func (r *CategoryRepository) GetAllCategoriesByUniverse(universe string) (categories []models.Category, err error) {
	rows, err := r.DB.Query(
		`SELECT categories.id,
			categories.name AS catname,
			categories.universe AS universe,
			categories.ancestry AS ancest,
			categories.default_freq AS catfreq,
			geographies.handle AS catgeo,
			geographies.fips AS catgeofips,
			geographies.display_name AS catgeoname,
			geographies.display_name_short AS catgeonameshort,
			MIN(public_data_points.date) AS startdate,
			MAX(public_data_points.date) AS enddate
		FROM categories
		LEFT JOIN geographies ON geographies.id = categories.default_geo_id
		LEFT JOIN data_list_measurements ON data_list_measurements.data_list_id = categories.data_list_id
		LEFT JOIN measurement_series ON measurement_series.measurement_id = data_list_measurements.measurement_id
		LEFT JOIN series_v AS series
		    ON series.id = measurement_series.series_id
		   AND series.geography_id = categories.default_geo_id
		   AND RIGHT(series.name, 1) = categories.default_freq
		   AND NOT series.restricted
		LEFT JOIN public_data_points ON public_data_points.series_id = series.id
		WHERE categories.universe = ?
		AND categories.ancestry IS NOT NULL
		AND NOT (categories.hidden OR categories.masked)
		GROUP BY categories.id, categories.name, categories.universe, categories.ancestry, categories.default_geo_id, categories.default_freq,
		         geographies.handle, geographies.fips, geographies.display_name, geographies.display_name_short
		ORDER BY categories.list_order, COALESCE(geographies.list_order, 999), geographies.handle`, universe)
	if err != nil {
		return
	}
	for rows.Next() {
		category := models.CategoryWithAncestry{}
		err = rows.Scan(
			&category.Id,
			&category.Name,
			&category.Universe,
			&category.Ancestry,
			&category.DefaultFrequency,
			&category.DefaultGeoHandle,
			&category.DefaultGeoFIPS,
			&category.DefaultGeoName,
			&category.DefaultGeoShortName,
			&category.ObservationStart,
			&category.ObservationEnd,
		)
		if err != nil {
			return
		}

		parentId := getParentId(category.Ancestry)
		dataPortalCategory := models.Category{
			Id:       category.Id,
			Name:     category.Name,
			Universe: category.Universe,
			ParentId: parentId,
		}
		if category.DefaultFrequency.Valid || category.DefaultGeoHandle.Valid || category.ObservationStart.Valid || category.ObservationEnd.Valid {
			// Only initialize Defaults struct if any defaults values are available
			dataPortalCategory.Defaults = &models.CategoryDefaults{}
		}
		if category.DefaultFrequency.Valid {
			dataPortalCategory.Defaults.Frequency = &models.DataPortalFrequency{
				Freq:  category.DefaultFrequency.String,
				Label: freqLabel[category.DefaultFrequency.String],
			}
		}
		if category.DefaultGeoHandle.Valid {
			dataPortalCategory.Defaults.Geography = &models.DataPortalGeography{
				Handle: category.DefaultGeoHandle.String,
			}
			if category.DefaultGeoFIPS.Valid {
				dataPortalCategory.Defaults.Geography.FIPS = category.DefaultGeoFIPS.String
			}
			if category.DefaultGeoName.Valid {
				dataPortalCategory.Defaults.Geography.Name = category.DefaultGeoName.String
			}
			if category.DefaultGeoShortName.Valid {
				dataPortalCategory.Defaults.Geography.ShortName = category.DefaultGeoShortName.String
			}
		}
		if category.ObservationStart.Valid {
			dataPortalCategory.Defaults.ObservationStart = &category.ObservationStart.Time
		}
		if category.ObservationEnd.Valid {
			dataPortalCategory.Defaults.ObservationEnd = &category.ObservationEnd.Time
		}
		categories = append(categories, dataPortalCategory)
	}
	return
}

func getParentId(ancestry sql.NullString) (parentId int64) {
	if !ancestry.Valid {
		return
	}
	parents := strings.Split(ancestry.String, "/")
	if len(parents) == 0 {
		return
	}
	parentId, _ = strconv.ParseInt(parents[len(parents)-1], 10, 64)
	return
}

func (r *CategoryRepository) GetCategoryRoots() (categories []models.Category, err error) {
	rows, err := r.DB.Query(`SELECT id, name, universe FROM categories WHERE ancestry IS NULL ORDER BY list_order;`)
	if err != nil {
		return
	}
	for rows.Next() {
		category := models.Category{}
		err = rows.Scan(
			&category.Id,
			&category.Name,
			&category.Universe,
		)
		if err != nil {
			return
		}
		categories = append(categories, category)
	}
	return
}

func (r *CategoryRepository) GetCategoryRootByUniverse(universe string) (category models.Category, err error) {
	rows, err := r.DB.Query(`SELECT categories.id, categories.name, categories.universe, categories.default_freq,
					geographies.handle, geographies.display_name, geographies.display_name_short
				FROM categories
				  LEFT JOIN geographies on geographies.id = categories.default_geo_id
				WHERE categories.universe = ?
				AND categories.ancestry IS NULL;`, universe)
	if err != nil {
		return
	}
	scanCat := models.CategoryWithAncestry{}
	for rows.Next() {
		err = rows.Scan(
			&category.Id,
			&category.Name,
			&category.Universe,
			&scanCat.DefaultFrequency,
			&scanCat.DefaultGeoHandle,
			&scanCat.DefaultGeoName,
			&scanCat.DefaultGeoShortName,
		)
		if err != nil {
			return
		}
		if scanCat.DefaultFrequency.Valid || scanCat.DefaultGeoHandle.Valid {
			category.Defaults = &models.CategoryDefaults{}
		}
		if scanCat.DefaultFrequency.Valid {
			category.Defaults.Frequency = &models.DataPortalFrequency{
				Freq:  scanCat.DefaultFrequency.String,
				Label: freqLabel[scanCat.DefaultFrequency.String],
			}
		}
		if scanCat.DefaultGeoHandle.Valid {
			category.Defaults.Geography = &models.DataPortalGeography{
				Handle:    scanCat.DefaultGeoHandle.String,
				Name:      scanCat.DefaultGeoName.String,
				ShortName: scanCat.DefaultGeoShortName.String,
			}
		}
		return
	}
	return
}

func (r *CategoryRepository) GetCategoryById(id int64) (models.Category, error) {
	return r.GetCategoryByIdGeoFreq(id, "", "")
}

func (r *CategoryRepository) GetCategoryByIdGeoFreq(id int64, originGeo string, originFreq string) (models.Category, error) {
	dataPortalCategory := models.Category{}
	rows, err := r.DB.Query(
		`SELECT categories.id,
			categories.name AS catname,
			categories.universe AS universe,
			parentcat.id AS parent_id,
			categories.header AS header,
			COALESCE(mygeo.handle, parentgeo.handle) AS def_geo,
			COALESCE(categories.default_freq, parentcat.default_freq) AS def_freq,
			geographies.handle AS series_geo,
			RIGHT(series.name, 1) AS series_freq,
			geographies.fips AS geofips,
			geographies.display_name AS geoname,
			geographies.display_name_short AS geonameshort,
			MIN(public_data_points.date) AS startdate,
			MAX(public_data_points.date) AS enddate
		FROM categories
  		LEFT JOIN geographies mygeo ON mygeo.id = categories.default_geo_id
		LEFT JOIN categories parentcat ON parentcat.id = SUBSTRING_INDEX(categories.ancestry, '/', -1)
		LEFT JOIN geographies parentgeo ON parentgeo.id = parentcat.default_geo_id
	        LEFT JOIN data_list_measurements ON data_list_measurements.data_list_id = categories.data_list_id
		LEFT JOIN measurement_series ON measurement_series.measurement_id = data_list_measurements.measurement_id
		LEFT JOIN series_v AS series
		    ON series.id = measurement_series.series_id
		   AND NOT series.restricted
		LEFT JOIN geographies ON geographies.id = series.geography_id
		LEFT JOIN public_data_points ON public_data_points.series_id = series.id
		WHERE categories.id = ?
		AND public_data_points.value IS NOT null /* suppress those with no public data */
		GROUP BY categories.id, categories.name, categories.universe, parentcat.id, categories.header, COALESCE(mygeo.handle, parentgeo.handle),
		         COALESCE(categories.default_freq, parentcat.default_freq), geographies.id, geographies.handle, RIGHT(series.name, 1),
		         geographies.fips, geographies.display_name, geographies.display_name_short
		ORDER BY COALESCE(geographies.list_order, 999), geographies.handle`, id)
	if err != nil {
		return dataPortalCategory, err
	}
	var geosResult []models.DataPortalGeography
	var freqsResult []models.DataPortalFrequency

	for rows.Next() {
		var handle, seriesFreq string
		category := models.CategoryWithAncestry{}
		scangeo := models.Geography{}
		err = rows.Scan(
			&category.Id,
			&category.Name,
			&category.Universe,
			&category.ParentId,
			&category.IsHeader,
			&category.DefaultGeoHandle,
			&category.DefaultFrequency,
			&handle,
			&seriesFreq,
			&scangeo.FIPS,
			&scangeo.Name,
			&scangeo.ShortName,
			&scangeo.ObservationStart,
			&scangeo.ObservationEnd,
		)
		if dataPortalCategory.Id == 0 {
			// Store Category-level information
			dataPortalCategory.Id = category.Id
			dataPortalCategory.Name = category.Name
			dataPortalCategory.Universe = category.Universe
			dataPortalCategory.IsHeader = category.IsHeader
			if category.ParentId.Valid {
				dataPortalCategory.ParentId = category.ParentId.Int64
			}
			if originGeo == "" && category.DefaultGeoHandle.Valid {
				originGeo = category.DefaultGeoHandle.String
			}
			if originFreq == "" && category.DefaultFrequency.Valid {
				originFreq = category.DefaultFrequency.String
			}
			startOfTime := time.Date(1099, 1, 1, 0, 0, 0, 0, &time.Location{})
			endOfTime := time.Date(2999, 1, 1, 0, 0, 0, 0, &time.Location{})
			dataPortalCategory.ObservationStart = &endOfTime // yes, reversed at init time
			dataPortalCategory.ObservationEnd = &startOfTime
		}
		geo := &models.DataPortalGeography{Handle: handle}
		freq := &models.DataPortalFrequency{Freq: seriesFreq, Label: freqLabel[seriesFreq]}
		if scangeo.FIPS.Valid {
			geo.FIPS = scangeo.FIPS.String
		}
		if scangeo.Name.Valid {
			geo.Name = scangeo.Name.String
		}
		if scangeo.ShortName.Valid {
			geo.ShortName = scangeo.ShortName.String
		}
		if scangeo.ObservationStart.Valid {
			geo.ObservationStart = &scangeo.ObservationStart.Time
			freq.ObservationStart = &scangeo.ObservationStart.Time
			if scangeo.ObservationStart.Time.Before(*dataPortalCategory.ObservationStart) {
				dataPortalCategory.ObservationStart = &scangeo.ObservationStart.Time
			}
		}
		if scangeo.ObservationEnd.Valid {
			geo.ObservationEnd = &scangeo.ObservationEnd.Time
			freq.ObservationEnd = &scangeo.ObservationEnd.Time
			if scangeo.ObservationEnd.Time.After(*dataPortalCategory.ObservationEnd) {
				dataPortalCategory.ObservationEnd = &scangeo.ObservationEnd.Time
			}
		}
		if geo.Handle == originGeo {
			freqsResult = append(freqsResult, *freq)
		}
		if seriesFreq == originFreq {
			geosResult = append(geosResult, *geo)

			if geo.Handle == originGeo {
				dataPortalCategory.Current = &models.CurrentGeoFreq{
					Geography:        originGeo,
					Frequency:        originFreq,
					ObservationStart: &scangeo.ObservationStart.Time,
					ObservationEnd:   &scangeo.ObservationEnd.Time,
				}
			}
		}
	}
	sort.Sort(models.ByFrequency(freqsResult))
	dataPortalCategory.Geographies = &geosResult
	dataPortalCategory.Frequencies = &freqsResult
	return dataPortalCategory, err
}

func (r *CategoryRepository) GetCategoriesByName(name string) (categories []models.Category, err error) {
	fuzzyString := "%" + name + "%"
	rows, err := r.DB.Query(`SELECT id, name, universe, ancestry FROM categories
							 WHERE LOWER(name) LIKE ? AND NOT (hidden OR masked)
							 ORDER BY list_order;`, fuzzyString)
	if err != nil {
		return
	}
	for rows.Next() {
		category := models.CategoryWithAncestry{}
		err = rows.Scan(
			&category.Id,
			&category.Name,
			&category.Universe,
			&category.Ancestry,
		)
		if err != nil {
			return
		}
		parentId := getParentId(category.Ancestry)
		categories = append(categories, models.Category{
			Id:       category.Id,
			Name:     category.Name,
			Universe: category.Universe,
			ParentId: parentId,
		})
	}
	return
}

func (r *CategoryRepository) GetChildrenOf(id int64) (children []models.Category, err error) {
	rows, err := r.DB.Query(`SELECT categories.id, categories.universe, categories.name, header, ancestry,
												geographies.handle, geographies.fips, geographies.display_name, geographies.display_name_short, default_freq
										FROM categories LEFT JOIN geographies ON geographies.id = categories.default_geo_id
										WHERE SUBSTRING_INDEX(categories.ancestry, '/', -1) = ?
										AND NOT (categories.hidden OR categories.masked)
										ORDER BY categories.list_order, COALESCE(geographies.list_order, 999), geographies.handle`, id)
	if err != nil {
		return
	}
	for rows.Next() {
		dataPortalCategory := models.Category{}
		category := models.CategoryWithAncestry{}
		err = rows.Scan(
			&dataPortalCategory.Id,
			&dataPortalCategory.Universe,
			&dataPortalCategory.Name,
			&dataPortalCategory.IsHeader,
			&category.Ancestry,
			&category.DefaultGeoHandle,
			&category.DefaultGeoFIPS,
			&category.DefaultGeoName,
			&category.DefaultGeoShortName,
			&category.DefaultFrequency,
		)
		if err != nil {
			return
		}
		if category.Ancestry.Valid {
			dataPortalCategory.ParentId = getParentId(category.Ancestry)
		}
		if category.DefaultFrequency.Valid || category.DefaultGeoHandle.Valid || category.ObservationStart.Valid || category.ObservationEnd.Valid {
			// Only initialize Defaults struct if any defaults values are available
			dataPortalCategory.Defaults = &models.CategoryDefaults{}
		}
		if category.DefaultFrequency.Valid {
			dataPortalCategory.Defaults.Frequency = &models.DataPortalFrequency{
				Freq:  category.DefaultFrequency.String,
				Label: freqLabel[category.DefaultFrequency.String],
			}
		}
		if category.DefaultGeoHandle.Valid {
			dataPortalCategory.Defaults.Geography = &models.DataPortalGeography{
				Handle: category.DefaultGeoHandle.String,
			}
			if category.DefaultGeoFIPS.Valid {
				dataPortalCategory.Defaults.Geography.FIPS = category.DefaultGeoFIPS.String
			}
			if category.DefaultGeoName.Valid {
				dataPortalCategory.Defaults.Geography.Name = category.DefaultGeoName.String
			}
			if category.DefaultGeoShortName.Valid {
				dataPortalCategory.Defaults.Geography.ShortName = category.DefaultGeoShortName.String
			}
		}
		children = append(children, dataPortalCategory)
	}
	return
}

func (r *CategoryRepository) getCategoryTree(
	id int64,
	geo string,
	freq string,
	seriesRepository *SeriesRepository,
) (tree []models.CategoryWithInflatedSeries, err error) {
	kids, err := r.GetChildrenOf(id)
	if err != nil {
		return
	}
	for _, kid := range kids {
		if kid.IsHeader {
			tree = append(tree, models.CategoryWithInflatedSeries{Category: kid})

			subtree, anErr := r.getCategoryTree(kid.Id, geo, freq, seriesRepository) //recursion
			if anErr != nil {
				return
			}
			for _, cat := range subtree {
				tree = append(tree, cat)
			}
			continue
		}
		inflatedCat := models.CategoryWithInflatedSeries{}

		category, anErr := r.GetCategoryByIdGeoFreq(kid.Id, geo, freq)
		if anErr != nil {
			err = anErr
			return
		}
		inflatedCat.Category = category

		if geo != "" && freq != "" {
			seriesList, anErr := seriesRepository.GetInflatedSeriesByGroupGeoAndFreq(kid.Id, geo, freq, Category)
			if anErr != nil {
				err = anErr
				return
			}
			inflatedCat.Series = seriesList
		}
		tree = append(tree, inflatedCat)
	}
	return
}

func (r *CategoryRepository) CreateCategoryPackage(
	id int64,
	geo string,
	freq string,
	seriesRepository *SeriesRepository,
) (pkg models.DataPortalCategoryPackage, err error) {

	tree, err := r.getCategoryTree(id, geo, freq, seriesRepository)
	if err != nil {
		return
	}
	for _, k := range tree {
		pkg.CatSubTree = append(pkg.CatSubTree, k)
	}
	if len(tree) > 0 {
		navCats, anErr := r.GetNavCategoriesByUniverse(tree[0].Category.Universe)
		if anErr != nil {
			err = anErr
			return
		}
		pkg.NavCategories = navCats
	}

	// Add parent nav category to the "categories" array
	for _, navCat := range pkg.NavCategories {
		if navCat.Id == id {
			pkg.CatSubTree = append(pkg.CatSubTree, models.CategoryWithInflatedSeries{Category: navCat})
			break
		}
	}
	return
}
