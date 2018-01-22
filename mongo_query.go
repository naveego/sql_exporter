package sql_exporter

import (
	"context"
	"fmt"

	"gopkg.in/mgo.v2/bson"

	"github.com/naveego/sql_exporter/config"
	"github.com/naveego/sql_exporter/errors"
)

// MongoQuery wraps a sql.Stmt and all the metrics populated from it. It helps extract keys and values from result rows.
type MongoQuery struct {
	config         *config.QueryConfig
	metricFamilies []*MetricFamily
	// columnTypes maps column names to the column type expected by metrics: key (string) or value (float64).
	columnTypes columnTypeMap
	logContext  string

	query []bson.M
}

// NewQuery returns a new Query that will populate the given metric families.
func NewMongoQuery(logContext string, qc *config.QueryConfig, metricFamilies ...*MetricFamily) (Collector, errors.WithContext) {
	logContext = fmt.Sprintf("%s, query=%q", logContext, qc.Name)

	columnTypes := make(columnTypeMap)

	for _, mf := range metricFamilies {
		for _, kcol := range mf.config.KeyLabels {
			if err := setColumnType(logContext, kcol, columnTypeKey, columnTypes); err != nil {
				return nil, err
			}
		}
		for _, vcol := range mf.config.Values {
			if err := setColumnType(logContext, vcol, columnTypeValue, columnTypes); err != nil {
				return nil, err
			}
		}
	}

	query, err := ParseMongoAggregationPipeline(qc.Query)
	if err != nil {
		return nil, errors.Wrap(logContext, err)
	}

	q := MongoQuery{
		config:         qc,
		metricFamilies: metricFamilies,
		columnTypes:    columnTypes,
		logContext:     logContext,
		query:          query,
	}
	return &q, nil
}

func ParseMongoAggregationPipeline(json string) ([]bson.M, error) {
	var query []bson.M
	err := bson.UnmarshalJSON([]byte(json), &query)
	return query, err
}

// Collect is the equivalent of prometheus.Collector.Collect() but takes a context to run in and a database to run on.
func (q *MongoQuery) Collect(ctx context.Context, db DB, ch chan<- Metric) {
	if ctx.Err() != nil {
		ch <- NewInvalidMetric(errors.Wrap(q.logContext, ctx.Err()))
		return
	}

	mongodb, err := db.MongoDB(q.logContext)
	if err != nil {
		ch <- NewInvalidMetric(errors.Wrap(q.logContext, ctx.Err()))
	}

	c := mongodb.C(q.config.Collection)

	var results []map[string]interface{}

	if err = c.Pipe(q.query).All(&results); err != nil {
		ch <- NewInvalidMetric(errors.Wrap(q.logContext, ctx.Err()))
	}

	for _, row := range results {
		data, err := q.scanRow(row)
		if err != nil {
			ch <- NewInvalidMetric(err)
			continue
		}
		for _, mf := range q.metricFamilies {
			mf.Collect(data, ch)
		}
	}
}

// scanRow scans the current row into a map of column name to value, with string values for key columns and float64
// values for value columns, using dest as a buffer.
func (q *MongoQuery) scanRow(row map[string]interface{}) (map[string]interface{}, errors.WithContext) {

	// Pick all values we're interested in into a map.
	result := make(map[string]interface{}, len(q.columnTypes))
	for name, _ := range q.columnTypes {
		value, ok := row[name]
		if !ok {
			return nil, errors.Errorf(q.logContext, "query result did not contain expected property %q", name)
		}

		switch v := value.(type) {
		case string:
			result[name] = v
		case int64:
			result[name] = float64(v)
		case int32:
			result[name] = float64(v)
		case int:
			result[name] = float64(v)
		case float64:
			result[name] = v
		default:
			return nil, errors.Errorf(q.logContext, "scanning of query result failed, value at %q was of unacceptable type %T", name, value)

		}
	}
	return result, nil
}
