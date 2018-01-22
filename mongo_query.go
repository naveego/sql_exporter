package sql_exporter

import (
	"context"
	"database/sql"
	"fmt"

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

	conn *sql.DB
	stmt *sql.Stmt
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

	q := MongoQuery{
		config:         qc,
		metricFamilies: metricFamilies,
		columnTypes:    columnTypes,
		logContext:     logContext,
	}
	return &q, nil
}

// Collect is the equivalent of prometheus.Collector.Collect() but takes a context to run in and a database to run on.
func (q *MongoQuery) Collect(ctx context.Context, db DB, ch chan<- Metric) {
	if ctx.Err() != nil {
		ch <- NewInvalidMetric(errors.Wrap(q.logContext, ctx.Err()))
		return
	}

	// conn, ok := db.(*sql.DB)
	// if !ok {
	// 	ch <- NewInvalidMetric(errors.New(q.logContext, "db passed to MongoQuery.Collect must be a *sql.DB"))
	// 	return
	// }

	// rows, err := q.run(ctx, conn)
	// if err != nil {
	// 	// TODO: increment an error counter
	// 	ch <- NewInvalidMetric(err)
	// 	return
	// }
	// defer rows.Close()

	// dest, err := q.scanDest(rows)
	// if err != nil {
	// 	// TODO: increment an error counter
	// 	ch <- NewInvalidMetric(err)
	// 	return
	// }
	// for rows.Next() {
	// 	row, err := q.scanRow(rows, dest)
	// 	if err != nil {
	// 		ch <- NewInvalidMetric(err)
	// 		continue
	// 	}
	// 	for _, mf := range q.metricFamilies {
	// 		mf.Collect(row, ch)
	// 	}
	// }
	// if err1 := rows.Err(); err1 != nil {
	// 	ch <- NewInvalidMetric(errors.Wrap(q.logContext, err1))
	// }
}
