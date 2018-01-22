package data_exporter

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/naveego/data_exporter/errors"
	"gopkg.in/mgo.v2"

	_ "github.com/denisenkom/go-mssqldb" // register the MS-SQL driver
	_ "github.com/go-sql-driver/mysql"   // register the MySQL driver
	log "github.com/golang/glog"
	_ "github.com/kshvakov/clickhouse" // register the ClickHouse driver
	_ "github.com/lib/pq"              // register the PostgreSQL driver
)

type DB interface {
	Schema() string
	SQL(context string) (*sql.DB, error)
	MongoDB(context string) (*mgo.Database, error)
	Ping(context.Context) error
}

type connectionWrapper struct {
	db     interface{}
	schema string
}

func (c *connectionWrapper) Schema() string {
	return c.schema
}

func (c *connectionWrapper) SQL(context string) (*sql.DB, error) {
	if s, ok := c.db.(*sql.DB); ok {
		return s, nil
	}
	return nil, errors.New(context, "db wasn't a *sql.DB")
}

func (c *connectionWrapper) MongoDB(context string) (*mgo.Database, error) {
	if session, ok := c.db.(*mgo.Session); ok {
		clone := session.Clone()
		db := clone.DB("")
		return db, nil
	}
	return nil, errors.New(context, "db wasn't a *mgo.Session")
}

// OpenConnection extracts the driver name from the DSN (expected as the URI scheme), adjusts it where necessary (e.g.
// some driver supported DSN formats don't include a scheme), opens a DB handle ensuring early termination if the
// context is closed (this is actually prevented by `database/sql` implementation), sets connection limits and returns
// the handle.
//
// Below is the list of supported databases (with built in drivers) and their DSN formats. Unfortunately there is no
// dynamic way of loading a third party driver library (as e.g. with Java classpaths), so any driver additions require
// a binary rebuild.
//
// MySQL
//
// Using the https://github.com/go-sql-driver/mysql driver, DSN format (passed to the driver stripped of the `mysql://`
// prefix):
//   mysql://username:password@protocol(host:port)/dbname?param=value
//
// PostgreSQL
//
// Using the https://godoc.org/github.com/lib/pq driver, DSN format (passed through to the driver unchanged):
//   postgres://username:password@host:port/dbname?param=value
//
// MS SQL Server
//
// Using the https://github.com/denisenkom/go-mssqldb driver, DSN format (passed through to the driver unchanged):
//   sqlserver://username:password@host:port/instance?param=value
//
// Clickhouse
//
// Using the https://github.com/kshvakov/clickhouse driver, DSN format (passed to the driver with the`clickhouse://`
// prefix replaced with `tcp://`):
//   clickhouse://host:port?username=username&password=password&database=dbname&param=value
//
// MongoDB
// Using the gopkg.in/mgo.v2 framework. DSN format:
// 	  mongodb://[username:password@]host1[:port1][,host2[:port2],...[,hostN[:portN]]][/[database][?options]]
func OpenConnection(ctx context.Context, logContext, dsn string, maxConns, maxIdleConns int) (DB, error) {
	// Extract driver name from DSN.
	idx := strings.Index(dsn, "://")
	if idx == -1 {
		return nil, fmt.Errorf("missing driver in data source name. Expected format `<driver>://<dsn>`")
	}
	driver := dsn[:idx]

	log.Infof("Opening connection for DSN %q using driver %q", dsn, driver)

	// Adjust DSN, where necessary.
	switch driver {
	case "mongodb":
		return openMongoDBConnection(ctx, logContext, dsn, maxConns, maxIdleConns)
	case "mysql":
		dsn = strings.TrimPrefix(dsn, "mysql://")
	case "clickhouse":
		dsn = "tcp://" + strings.TrimPrefix(dsn, "clickhouse://")
	}

	// Open the DB handle in a separate goroutine so we can terminate early if the context closes.
	var (
		conn *sql.DB
		err  error
		ch   = make(chan error)
	)
	go func() {
		conn, err = sql.Open(driver, dsn)
		close(ch)
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ch:
		if err != nil {
			return nil, err
		}
	}

	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetMaxOpenConns(maxConns)

	if log.V(1) {
		if len(logContext) > 0 {
			logContext = fmt.Sprintf("[%s] ", logContext)
		}
		log.Infof("%sDatabase handle successfully opened with driver %s.", logContext, driver)
	}
	return &connectionWrapper{db: conn, schema: dsn}, nil
}

func openMongoDBConnection(ctx context.Context, logContext, dsn string, maxConns, maxIdleConns int) (DB, error) {
	fmt.Println("opening mongodb connection", logContext, dsn)
	if !strings.HasPrefix(dsn, "mongodb://") {
		return nil, errors.New(logContext, "dsn must specify mongodb://")
	}

	session, err := mgo.Dial(dsn)
	return &connectionWrapper{
		schema: "mongodb",
		db:     session,
	}, err
}

// Ping is a wrapper around sql.DB.PingContext() that terminates as soon as the context is closed.
//
// sql.DB does not actually pass along the context to the driver when opening a connection (which always happens if the
// database is down) and the driver uses an arbitrary timeout which may well be longer than ours. So we run the ping
// call in a goroutine and terminate immediately if the context is closed.
func (c *connectionWrapper) Ping(ctx context.Context) (err error) {

	switch typedDB := c.db.(type) {
	case *sql.DB:
		return PingSQLDB(ctx, typedDB)
	case *mgo.Session:
		return PingMongoDB(ctx, typedDB)
	default:
		return fmt.Errorf("unknown connection schema: %s", c.Schema())
	}
}

// PingSQLDB is a wrapper around sql.DB.PingContext() that terminates as soon as the context is closed.
//
// sql.DB does not actually pass along the context to the driver when opening a connection (which always happens if the
// database is down) and the driver uses an arbitrary timeout which may well be longer than ours. So we run the ping
// call in a goroutine and terminate immediately if the context is closed.
func PingSQLDB(ctx context.Context, conn *sql.DB) error {
	ch := make(chan error, 1)

	go func() {
		ch <- conn.PingContext(ctx)
		close(ch)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		return err
	}
}

func PingMongoDB(ctx context.Context, session *mgo.Session) error {
	ch := make(chan error, 1)

	go func() {
		ch <- session.Ping()
		close(ch)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		return err
	}
}
