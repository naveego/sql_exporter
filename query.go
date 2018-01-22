package sql_exporter

import (
	"fmt"

	"github.com/naveego/sql_exporter/config"
	"github.com/naveego/sql_exporter/errors"
)

func NewQuery(logContext string, qc *config.QueryConfig, metricFamilies ...*MetricFamily) (Collector, errors.WithContext) {
	fmt.Println(logContext, qc)
	switch qc.DSNScheme {
	case "mongodb":
		return NewMongoQuery(logContext, qc, metricFamilies...)
	default:
		return NewSQLQuery(logContext, qc, metricFamilies...)
	}
}
