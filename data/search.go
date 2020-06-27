package data

import (
	"sort"
	"time"
	"github.com/UHERO/rest-api/models"
	"strings"
)

type SearchRepository struct {
	Categories *FooRepository
	Series     *FooRepository
}

func (r *FooRepository) GetSeriesBySearchText(searchText string) (seriesList []models.DataPortalSeries, err error) {
	seriesList, err = r.GetSeriesBySearchTextAndUniverse(searchText, "UHERO")
	return
}

func (r *FooRepository) GetSeriesBySearchTextAndUniverse(searchText string, universeText string) (seriesList []models.DataPortalSeries, err error) {
	//language=MySQL
	rows, err := r.RunQuery(`/* SELECT
	series.id, series.name, series.universe, series.description, frequency, series.seasonally_adjusted, series.seasonal_adjustment,
	COALESCE(NULLIF(units.long_label, ''), NULLIF(MAX(measurement_units.long_label), '')),
	COALESCE(NULLIF(units.short_label, ''), NULLIF(MAX(measurement_units.short_label), '')),
	COALESCE(NULLIF(series.dataPortalName, ''), MAX(measurements.data_portal_name)),
       series.percent, series.real,
	COALESCE(NULLIF(sources.description, ''), NULLIF(MAX(measurement_sources.description), '')),
	COALESCE(NULLIF(series.source_link, ''), NULLIF(MAX(measurements.source_link), ''), NULLIF(sources.link, ''), NULLIF(MAX(measurement_sources.link), '')),
	COALESCE(NULLIF(source_details.description, ''), NULLIF(MAX(measurement_source_details.description), '')),
	MAX(measurements.table_prefix), MAX(measurements.table_postfix),
	MAX(measurements.id), MAX(measurements.data_portal_name),
	NULL, series.base_year, series.decimals,
	MAX(geo.fips), MAX(geo.handle) AS shandle, MAX(geo.display_name), MAX(geo.display_name_short)
	FROM %s AS series
	LEFT JOIN geographies geo ON geo.id = series.geography_id
	LEFT JOIN measurement_series ON measurement_series.series_id = series.id
	LEFT JOIN measurements ON measurements.id = measurement_series.measurement_id
	LEFT JOIN data_list_measurements ON data_list_measurements.measurement_id = measurements.id
	LEFT JOIN categories ON categories.data_list_id = data_list_measurements.data_list_id
	LEFT JOIN units ON units.id = series.unit_id
	LEFT JOIN units AS measurement_units ON measurement_units.id = measurements.unit_id
	LEFT JOIN sources ON sources.id = series.source_id
	LEFT JOIN sources AS measurement_sources ON measurement_sources.id = measurements.source_id
	LEFT JOIN source_details ON source_details.id = series.source_detail_id
	LEFT JOIN source_details AS measurement_source_details ON measurement_source_details.id = measurements.source_detail_id */
	SELECT series_id, series_name, series_universe, series_description, frequency, seasonally_adjusted, seasonal_adjustment,
	       units_long, units_short, data_portal_name, percent, pv.real, source_description, source_link, source_detail_description,
		   MAX(table_prefix), MAX(table_postfix), MAX(measurement_id), MAX(measurement_portal_name), NULL,
	       base_year, decimals, geo_fips, geo_handle, geo_display_name, geo_display_name_short
	FROM <%PORTAL%> pv
	WHERE series_universe = ?
	AND ((MATCH(series_name, series_description, data_portal_name) AGAINST(? IN NATURAL LANGUAGE MODE))
	  OR (MATCH(category_name) AGAINST(? IN NATURAL LANGUAGE MODE))
	  OR LOWER(CONCAT(series_name,
	  		COALESCE(series_description, ''),
	  		COALESCE(data_portal_name, ''),
	  		COALESCE(category_name, ''))) LIKE CONCAT('%', LOWER(?), '%'))
	GROUP BY series_id
	LIMIT 50;`, universeText, searchText, searchText, searchText)
	if err != nil {
		return
	}
	for rows.Next() {
		dataPortalSeries, scanErr := getNextSeriesFromRows(rows)
		if scanErr != nil {
			return seriesList, scanErr
		}
		geos, freqs, err := getAllFreqsGeos(r, dataPortalSeries.Id, 0)
		if err != nil {
			return seriesList, err
		}
		dataPortalSeries.Geographies = &geos
		dataPortalSeries.Frequencies = &freqs
		seriesList = append(seriesList, dataPortalSeries)
	}
	return
}

func (r *SearchRepository) GetSearchSummary(searchText string) (searchSummary models.SearchSummary, err error) {
	searchSummary, err = r.GetSearchSummaryByUniverse(searchText, "UHERO")
	return
}

func (r *SearchRepository) GetSearchSummaryByUniverse(searchText string, universeText string) (searchSummary models.SearchSummary, err error) {
	searchSummary.SearchText = searchText

	var observationStart, observationEnd models.NullTime
	//language=MySQL
	err = r.Series.RunQueryRow(`
	    SELECT MIN(public_data_points.date) AS start_date, MAX(public_data_points.date) AS end_date
	    FROM <%DATAPOINTS%> as public_data_points
	    JOIN <%SERIES%> AS series ON series.id = public_data_points.series_id
	    JOIN (
			SELECT series_id FROM measurement_series WHERE measurement_id IN (
				SELECT measurement_id FROM data_list_measurements WHERE data_list_id IN (
					SELECT data_list_id FROM categories
					WHERE universe = ?
					AND ((MATCH(name) AGAINST(? IN NATURAL LANGUAGE MODE))
						OR (LOWER(COALESCE(name, '')) LIKE CONCAT('%', LOWER(?), '%')))
				)
			)
			UNION
			SELECT id AS series_id FROM <%SERIES%>
			WHERE universe = ?
			AND ((MATCH(name, description, dataPortalName) AGAINST(? IN NATURAL LANGUAGE MODE))
				OR LOWER(CONCAT(name, COALESCE(description, ''), COALESCE(dataPortalName, ''))) LIKE CONCAT('%', LOWER(?), '%'))
	    ) AS s ON s.series_id = series.id `,
		universeText, searchText, searchText,
		universeText, searchText, searchText).Scan(
		&observationStart,
		&observationEnd,
	)
	if err != nil {
		return
	}
	if observationStart.Valid && observationStart.Time.After(time.Time{}) {
		searchSummary.ObservationStart = &observationStart.Time
	}
	if observationEnd.Valid && observationEnd.Time.After(time.Time{}) {
		searchSummary.ObservationEnd = &observationEnd.Time
	}

	rootCat, err := r.Categories.GetCategoryRootByUniverse(universeText)
	if err != nil {
		return
	}
	if rootCat.Defaults != nil && rootCat.Defaults.Geography != nil {
		searchSummary.DefaultGeo = rootCat.Defaults.Geography
	}
	if rootCat.Defaults != nil && rootCat.Defaults.Frequency != nil {
		searchSummary.DefaultFreq = rootCat.Defaults.Frequency
	}

	//language=MySQL
	rows, err := r.Series.RunQuery(`
	SELECT DISTINCT geo_fips, geo_display_name, geo_display_name_short, geo_handle AS geo, RIGHT(series_name, 1) as freq
	FROM <%PORTAL%> pv
	WHERE series_universe = ?
	AND ((MATCH(series_name, series_description, data_portal_name) AGAINST(? IN NATURAL LANGUAGE MODE))
	  OR (MATCH(category_name) AGAINST(? IN NATURAL LANGUAGE MODE))
	  OR LOWER(CONCAT(series_name,
	  		COALESCE(series_description, ''),
	  		COALESCE(data_portal_name, ''),
	  		COALESCE(category_name, ''))) LIKE CONCAT('%', LOWER(?), '%'))
	ORDER BY 1,2,3,4;`, universeText, searchText, searchText, searchText)
												/**** REPLACE LIKE+CONCAT with REGEXP! no need LOWER ****/
	if err != nil {
		return
	}
	seenGeos := map[string]models.DataPortalGeography{}
	seenFreqs := map[string]models.DataPortalFrequency{}

	for rows.Next() {
		scangeo := models.Geography{}
		frequency := models.DataPortalFrequency{}
		err = rows.Scan(
			&scangeo.FIPS,
			&scangeo.Name,
			&scangeo.ShortName,
			&scangeo.Handle,
			&frequency.Freq,
		)
		handle := scangeo.Handle
		if _, ok := seenGeos[handle]; !ok {
			geo := &models.DataPortalGeography{Handle: handle}
			if scangeo.FIPS.Valid {
				geo.FIPS = scangeo.FIPS.String
			}
			if scangeo.Name.Valid {
				geo.Name = scangeo.Name.String
			}
			if scangeo.ShortName.Valid {
				geo.ShortName = scangeo.ShortName.String
			}
			if searchSummary.DefaultGeo == nil {
				searchSummary.DefaultGeo = geo
			}
			seenGeos[handle] = *geo
		}
		handle = frequency.Freq
		if _, ok := seenFreqs[handle]; !ok {
			freq := &models.DataPortalFrequency{
				Freq:  handle,
				Label: freqLabel[handle],
			}
			if searchSummary.DefaultFreq == nil {
				searchSummary.DefaultFreq = freq
			}
			seenFreqs[handle] = *freq
		}
	}
	geosResult := make([]models.DataPortalGeography, 0, len(seenGeos))
	for _, value := range seenGeos {
		geosResult = append(geosResult, value)
	}
	sort.Sort(models.ByGeography(geosResult))

	freqsResult := make([]models.DataPortalFrequency, 0, len(seenFreqs))
	for _, value := range seenFreqs {
		freqsResult = append(freqsResult, value)
	}
	sort.Sort(models.ByFrequency(freqsResult))

	searchSummary.Geographies = &geosResult
	searchSummary.Frequencies = &freqsResult
	return
}

func (r *FooRepository) GetSearchResultsByGeoAndFreq(searchText string, geo string, freq string) (seriesList []models.DataPortalSeries, err error) {
	seriesList, err = r.GetSearchResultsByGeoAndFreqAndUniverse(searchText, geo, freq, "UHERO")
	return
}

func (r *FooRepository) GetSearchResultsByGeoAndFreqAndUniverse(
	searchText string,
	geo string,
	freq string,
	universeText string,
) (seriesList []models.DataPortalSeries, err error) {
	//language=MySQL
	rows, err := r.RunQuery(`/* SELECT
	series.id, series.name, series.universe, series.description, frequency, series.seasonally_adjusted, series.seasonal_adjustment,
	COALESCE(NULLIF(units.long_label, ''), NULLIF(MAX(measurement_units.long_label), '')),
	COALESCE(NULLIF(units.short_label, ''), NULLIF(MAX(measurement_units.short_label), '')),
	COALESCE(NULLIF(series.dataPortalName, ''), MAX(measurements.data_portal_name)),
   		series.percent, series.real,
	COALESCE(NULLIF(sources.description, ''), NULLIF(MAX(measurement_sources.description), '')),
	COALESCE(NULLIF(series.source_link, ''), NULLIF(MAX(measurements.source_link), ''), NULLIF(sources.link, ''), NULLIF(MAX(measurement_sources.link), '')),
	COALESCE(NULLIF(source_details.description, ''), NULLIF(MAX(measurement_source_details.description), '')),
	MAX(measurements.table_prefix), MAX(measurements.table_postfix),
	MAX(measurements.id), MAX(measurements.data_portal_name),
	NULL, series.base_year, series.decimals,
	MAX(geo.fips), MAX(geo.handle), MAX(geo.display_name), MAX(geo.display_name_short)
	FROM %s AS series
	LEFT JOIN geographies geo ON geo.id = series.geography_id
	LEFT JOIN measurement_series ON measurement_series.series_id = series.id
	LEFT JOIN measurements ON measurements.id = measurement_series.measurement_id
	LEFT JOIN data_list_measurements ON data_list_measurements.measurement_id = measurements.id
	LEFT JOIN categories ON categories.data_list_id = data_list_measurements.data_list_id
	LEFT JOIN units ON units.id = series.unit_id
	LEFT JOIN units AS measurement_units ON measurement_units.id = measurements.unit_id
	LEFT JOIN sources ON sources.id = series.source_id
	LEFT JOIN sources AS measurement_sources ON measurement_sources.id = measurements.source_id
	LEFT JOIN source_details ON source_details.id = series.source_detail_id
	LEFT JOIN source_details AS measurement_source_details ON measurement_source_details.id = measurements.source_detail_id */
	SELECT series_id, series_name, series_universe, series_description, frequency, seasonally_adjusted, seasonal_adjustment,
	       units_long, units_short, data_portal_name, percent, pv.real, source_description, source_link, source_detail_description,
		   MAX(table_prefix), MAX(table_postfix), MAX(measurement_id), MAX(measurement_portal_name), NULL,
	       base_year, decimals, geo_fips, geo_handle, geo_display_name, geo_display_name_short
	FROM <%PORTAL%> pv
	WHERE series_universe = ?
	AND geo_handle = ?
	AND frequency = ?
	AND ((MATCH(series_name, series_description, data_portal_name) AGAINST(? IN NATURAL LANGUAGE MODE))
	  OR (MATCH(category_name) AGAINST(? IN NATURAL LANGUAGE MODE))
	  OR LOWER(CONCAT(series_name,
	  		COALESCE(series_description, ''),
	  		COALESCE(data_portal_name, ''),
	  		COALESCE(category_name, ''))) LIKE CONCAT('%', LOWER(?), '%'))
	GROUP BY series_id
	LIMIT 50;`,
		universeText,
		geo,
		freqDbNames[strings.ToUpper(freq)],
		searchText,
		searchText,
		searchText,
	)

	if err != nil {
		return
	}
	for rows.Next() {
		dataPortalSeries, scanErr := getNextSeriesFromRows(rows)
		if scanErr != nil {
			return seriesList, scanErr
		}
		geos, freqs, err := getAllFreqsGeos(r, dataPortalSeries.Id, 0)
		if err != nil {
			return seriesList, err
		}
		dataPortalSeries.Geographies = &geos
		dataPortalSeries.Frequencies = &freqs
		seriesList = append(seriesList, dataPortalSeries)
	}
	return
}

func (r *FooRepository) GetInflatedSearchResultsByGeoAndFreq(searchText string, geo string, freq string) (seriesList []models.InflatedSeries, err error) {
	seriesList, err = r.GetInflatedSearchResultsByGeoAndFreqAndUniverse(searchText, geo, freq, "UHERO")
	return
}

func (r *FooRepository) GetInflatedSearchResultsByGeoAndFreqAndUniverse(
	searchText string,
	geo string,
	freq string,
	universeText string,
) (seriesList []models.InflatedSeries, err error) {
	//language=MySQL
	rows, err := r.RunQuery(`/* SELECT DISTINCT
	series.id, series.name, series.universe, series.description, frequency, series.seasonally_adjusted, series.seasonal_adjustment,
	COALESCE(NULLIF(units.long_label, ''), NULLIF(MAX(measurement_units.long_label), '')),
	COALESCE(NULLIF(units.short_label, ''), NULLIF(MAX(measurement_units.short_label), '')),
	COALESCE(NULLIF(series.dataPortalName, ''), MAX(measurements.data_portal_name)),
                series.percent, series.real,
	COALESCE(NULLIF(sources.description, ''), NULLIF(MAX(measurement_sources.description), '')),
	COALESCE(NULLIF(series.source_link, ''), NULLIF(MAX(measurements.source_link), ''), NULLIF(sources.link, ''), NULLIF(MAX(measurement_sources.link), '')),
	COALESCE(NULLIF(source_details.description, ''), NULLIF(MAX(measurement_source_details.description), '')),
	MAX(measurements.table_prefix), MAX(measurements.table_postfix),
	MAX(measurements.id), MAX(measurements.data_portal_name),
	NULL, series.base_year, series.decimals,
	MAX(geo.fips), MAX(geo.handle), MAX(geo.display_name), MAX(geo.display_name_short)
	FROM %s AS series
	LEFT JOIN geographies geo ON geo.id = series.geography_id
	LEFT JOIN measurement_series ON measurement_series.series_id = series.id
	LEFT JOIN measurements ON measurements.id = measurement_series.measurement_id
	LEFT JOIN data_list_measurements ON data_list_measurements.measurement_id = measurements.id
	LEFT JOIN categories ON categories.data_list_id = data_list_measurements.data_list_id
	LEFT JOIN units ON units.id = series.unit_id
	LEFT JOIN units AS measurement_units ON measurement_units.id = measurements.unit_id
	LEFT JOIN sources ON sources.id = series.source_id
	LEFT JOIN sources AS measurement_sources ON measurement_sources.id = measurements.source_id
	LEFT JOIN source_details ON source_details.id = series.source_detail_id
	LEFT JOIN source_details AS measurement_source_details ON measurement_source_details.id = measurements.source_detail_id */
	SELECT series_id, series_name, series_universe, series_description, frequency, seasonally_adjusted, seasonal_adjustment,
	       units_long, units_short, data_portal_name, percent, pv.real, source_description, source_link, source_detail_description,
		   MAX(table_prefix), MAX(table_postfix), MAX(measurement_id), MAX(measurement_portal_name), NULL,
	       base_year, decimals, geo_fips, geo_handle, geo_display_name, geo_display_name_short
	FROM <%PORTAL%> pv
	WHERE series_universe = ?
	AND geo_handle = ?
	AND frequency = ?
	AND ((MATCH(series_name, series_description, data_portal_name) AGAINST(? IN NATURAL LANGUAGE MODE))
	  OR (MATCH(category_name) AGAINST(? IN NATURAL LANGUAGE MODE))
	  OR LOWER(CONCAT(series_name,
	  		COALESCE(series_description, ''),
	  		COALESCE(data_portal_name, ''),
	  		COALESCE(category_name, ''))) LIKE CONCAT('%', LOWER(?), '%'))
	GROUP BY series_id
	LIMIT 50;`,
		universeText,
		geo,
		freqDbNames[strings.ToUpper(freq)],
		searchText,
		searchText,
		searchText,
	)
	if err != nil {
		return
	}
	for rows.Next() {
		dataPortalSeries, scanErr := getNextSeriesFromRows(rows)
		if scanErr != nil {
			return seriesList, scanErr
		}
		geos, freqs, err := getAllFreqsGeos(r, dataPortalSeries.Id, 0)
		if err != nil {
			return seriesList, err
		}
		dataPortalSeries.Geographies = &geos
		dataPortalSeries.Frequencies = &freqs
		seriesObservations, scanErr := r.GetSeriesObservations(dataPortalSeries.Id)
		if scanErr != nil {
			return seriesList, scanErr
		}
		inflatedSeries := models.InflatedSeries{dataPortalSeries, seriesObservations}
		seriesList = append(seriesList, inflatedSeries)
	}
	return
}

func (r *SearchRepository) CreateSearchPackage(
	searchText string,
	geo string,
	freq string,
	universe string,
) (pkg models.DataPortalSearchPackage, err error) {
	searchSummary, err := r.GetSearchSummaryByUniverse(searchText, universe)
	if err != nil {
		return
	}
	pkg.SearchSummary = searchSummary

	if geo == "" {
		geo = searchSummary.DefaultGeo.Handle
	}
	if freq == "" {
		freq = searchSummary.DefaultFreq.Freq
	}
	seriesList, err := r.Series.GetInflatedSearchResultsByGeoAndFreqAndUniverse(searchText, geo, freq, universe)
	if err != nil {
		return
	}
	pkg.Series = seriesList
	return
}
