package data_exporter

import (
	"github.com/naveego/data_exporter/config"
	"github.com/naveego/data_exporter/errors"
)

func NewQuery(logContext string, qc *config.QueryConfig, metricFamilies ...*MetricFamily) (Collector, errors.WithContext) {
	switch qc.DSNScheme {
	case "mongodb":
		return NewMongoQuery(logContext, qc, metricFamilies...)
	default:
		return NewSQLQuery(logContext, qc, metricFamilies...)
	}
}
