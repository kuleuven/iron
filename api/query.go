package api

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

// Query defines a query
type PreparedQuery struct {
	api         *API
	resultLimit int
	maxRows     int
	columns     []msg.ColumnNumber
	conditions  map[msg.ColumnNumber]string
}

// Query prepares a query to read from the irods catalog.
func (api *API) Query(columns ...msg.ColumnNumber) PreparedQuery {
	return PreparedQuery{
		api:        api,
		columns:    columns,
		maxRows:    500,
		conditions: make(map[msg.ColumnNumber]string),
	}
}

// Where adds a condition to the query for the specified column.
// The condition is a string that will be used to filter results
// based on the specified column.
func (q PreparedQuery) Where(column msg.ColumnNumber, condition string) PreparedQuery {
	q.conditions[column] = condition

	return q
}

// Limit limits the number of results.
func (q PreparedQuery) Limit(limit int) PreparedQuery {
	q.resultLimit = limit

	if q.maxRows > limit && limit > 0 {
		q.maxRows = limit
	}

	return q
}

// Execute executes the query.
// When called using iron.Client, this method blocks an irods connection
// until the result has been closed.
// When called using iron.Conn directly, the caller is responsible for not
// running a second query on the same connection.
func (q PreparedQuery) Execute(ctx context.Context) *Result {
	conn, err := q.api.Connect(ctx)
	if err != nil {
		return &Result{err: err}
	}

	result := &Result{
		Conn:    conn,
		Context: ctx,
		Query:   q,
	}

	result.buildQuery()
	result.executeQuery()

	return result
}

type Result struct {
	Conn     Conn
	Context  context.Context //nolint:containedctx
	Query    PreparedQuery
	query    *msg.QueryRequest
	result   *msg.QueryResponse
	err      error
	closeErr error
	row      int
}

// Err returns an error if the result has one.
func (r *Result) Err() error {
	return r.err
}

// Next returns true if there are more results.
func (r *Result) Next() bool {
	if r.err != nil {
		return false
	}

	if r.result.RowCount == 0 {
		r.cleanup()

		return false
	}

	r.row++

	if r.row >= r.Query.resultLimit && r.Query.resultLimit > 0 {
		r.cleanup()

		return false
	}

	if r.row < r.result.RowCount {
		return true
	}

	if r.result.ContinueIndex == 0 {
		r.cleanup()

		return false
	}

	r.query.ContinueIndex = r.result.ContinueIndex
	r.Query.resultLimit -= r.result.RowCount

	r.executeQuery()

	return r.Next()
}

var ErrRowOutOfBound = fmt.Errorf("row out of bound")

var ErrAttributeOutOfBound = fmt.Errorf("attribute count out of bound")

var ErrNoSQLResults = fmt.Errorf("no sql results")

// Scan reads the values in the current row into the values pointed
// to by dest, in order.  If an error occurs during scanning, the
// error is returned. The values pointed to by dest before the error
// occurred might be modified.
func (r *Result) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}

	if r.row < 0 || r.row >= r.result.RowCount {
		return ErrRowOutOfBound
	}

	if r.result.AttributeCount < len(dest) {
		return ErrAttributeOutOfBound
	}

	for attr := range dest {
		col := r.result.SQLResult[attr]
		if len(col.Values) <= r.row {
			return fmt.Errorf("%w: row %d is missing from column %d", ErrNoSQLResults, r.row, attr)
		}

		value := col.Values[r.row]

		if err := parseValue(value, dest[attr]); err != nil {
			return err
		}
	}

	return nil
}

// Close releases all resources associated with the result.
// It's safe to call Close multiple times.
func (r *Result) Close() error {
	r.cleanup()

	return r.closeErr
}

func (r *Result) buildQuery() {
	r.query = &msg.QueryRequest{
		MaxRows: r.Query.maxRows,
		Options: 0x20,
	}

	for _, col := range r.Query.columns {
		r.query.Selects.Add(int(col), 1)
	}

	for col, condition := range r.Query.conditions {
		r.query.Conditions.Add(int(col), condition)
	}

	r.Query.api.setFlags(&r.query.KeyVals)
}

func (r *Result) executeQuery() {
	r.result = &msg.QueryResponse{}

	r.err = r.Conn.Request(r.Context, 702, r.query, r.result)
	r.row = -1

	if rodsErr, ok := r.err.(*msg.IRODSError); ok && rodsErr.Code == -808000 { // CAT_NO_ROWS_FOUND
		r.err = nil
	}

	if r.err != nil {
		return
	}
}

func (r *Result) cleanup() {
	for r.result.ContinueIndex != 0 {
		r.query.ContinueIndex = r.result.ContinueIndex
		r.query.MaxRows = 0

		r.executeQuery()
	}

	if r.Conn != nil {
		r.closeErr = r.Conn.Close()
		r.Conn = nil
	}
}

func parseValue(value string, dest interface{}) error {
	switch reflect.ValueOf(dest).Elem().Kind() { //nolint:exhaustive
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("%w: %s (int)", err, value)
		}

		reflect.ValueOf(dest).Elem().SetInt(i)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("%w: %s (uint)", err, value)
		}

		reflect.ValueOf(dest).Elem().SetUint(u)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("%w: %s (float)", err, value)
		}

		reflect.ValueOf(dest).Elem().SetFloat(f)

	case reflect.String:
		reflect.ValueOf(dest).Elem().SetString(value)

	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%w: %s (bool)", err, value)
		}

		reflect.ValueOf(dest).Elem().SetBool(b)

	case reflect.Struct:
		if reflect.ValueOf(dest).Elem().Type() == reflect.TypeOf(time.Time{}) {
			t, err := parseTime(value)
			if err != nil {
				return fmt.Errorf("%w: %s (time)", err, value)
			}

			reflect.ValueOf(dest).Elem().Set(reflect.ValueOf(t))

			return nil
		}

		fallthrough
	default:
		return fmt.Errorf("unsupported type %T", dest)
	}

	return nil
}

func parseTime(timestring string) (time.Time, error) {
	i64, err := strconv.ParseInt(timestring, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse IRODS time string '%s'", timestring)
	}

	if i64 <= 0 {
		return time.Time{}, nil
	}

	return time.Unix(i64, 0), nil
}

type PreparedSingleRowQuery PreparedQuery

// QueryRow prepares a query to read a single row from the irods catalog.
func (api *API) QueryRow(columns ...msg.ColumnNumber) PreparedSingleRowQuery {
	return PreparedSingleRowQuery{
		api:         api,
		columns:     columns,
		resultLimit: 1,
		maxRows:     1,
		conditions:  make(map[msg.ColumnNumber]string),
	}
}

// Where adds a condition to the query for the specified column.
// The condition is a string that will be used to filter results
// based on the specified column.
func (r PreparedSingleRowQuery) Where(column msg.ColumnNumber, condition string) PreparedSingleRowQuery {
	r.conditions[column] = condition

	return r
}

// Execute executes the query.
func (r PreparedSingleRowQuery) Execute(ctx context.Context) *SingleRowResult {
	result := PreparedQuery(r).Execute(ctx)

	defer result.Close()

	if result.Next() {
		return &SingleRowResult{result: result.result}
	}

	if result.Err() != nil {
		return &SingleRowResult{err: result.Err()}
	}

	return &SingleRowResult{err: ErrNoRowFound}
}

type SingleRowResult struct {
	result *msg.QueryResponse
	err    error
}

var ErrNoRowFound = fmt.Errorf("no row found")

// Scan reads the values in the current row into the values pointed
// to by dest, in order.  If an error occurs during scanning, the
// error is returned. The values pointed to by dest before the error
// occurred might be modified.
func (r *SingleRowResult) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}

	for attr := range dest {
		col := r.result.SQLResult[attr]
		if len(col.Values) == 0 {
			return fmt.Errorf("%w: row 1 is missing from column %d", ErrNoSQLResults, attr)
		}

		value := col.Values[0]

		if err := parseValue(value, dest[attr]); err != nil {
			return err
		}
	}

	return nil
}
