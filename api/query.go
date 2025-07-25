package api

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/kuleuven/iron/msg"
)

// Query defines a query
type PreparedQuery struct {
	api         *API
	resultLimit int
	maxRows     int
	columns     []msg.ColumnNumber
	conditions  map[msg.ColumnNumber]string
}

// Condition defines a condition
type Condition struct {
	Column msg.ColumnNumber
	Op     string
	Value  string
}

// Equal creates a Condition that checks if the specified column is equal to the given value.
func Equal[V string | int | int64](column msg.ColumnNumber, value V) Condition {
	return Condition{
		Column: column,
		Op:     "=",
		Value:  fmt.Sprintf("'%v'", value),
	}
}

// NotEqual creates a Condition that checks if the specified column is equal to the given value.
func NotEqual[V string | int | int64](column msg.ColumnNumber, value V) Condition {
	return Condition{
		Column: column,
		Op:     "<>",
		Value:  fmt.Sprintf("'%v'", value),
	}
}

// Like creates a Condition that checks if the specified column matches the given SQL LIKE expression.
func Like(column msg.ColumnNumber, value string) Condition {
	return Condition{
		Column: column,
		Op:     "LIKE",
		Value:  fmt.Sprintf("'%s'", value),
	}
}

// In creates a Condition that checks if the specified column is in the given list of values.
// Note that it is not safe to use this method if one of the values contains a ' character,
// and at least two values are provided.
func In[V string | int | int64](column msg.ColumnNumber, values []V) Condition {
	if len(values) == 1 {
		return Equal(column, values[0])
	}

	strValues := make([]string, len(values))

	for i, v := range values {
		strValues[i] = fmt.Sprintf("'%v'", v)
	}

	// Use small caps IN condition to avoid issues if a name contains "in"
	// See https://github.com/irods/irods/blob/main/plugins/database/src/general_query.cpp#L1482-L1485
	return Condition{
		Column: column,
		Op:     "in",
		Value:  fmt.Sprintf("(%s)", strings.Join(strValues, ",")),
	}
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

// With adds a list of conditions to the query.
func (q PreparedQuery) With(condition ...Condition) PreparedQuery {
	for _, c := range condition {
		q.conditions[c.Column] = fmt.Sprintf("%s %s", c.Op, c.Value)
	}

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
// This method blocks an irods connection until the result has been closed.
// If the context is closed, no more results will be returned.
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

var ErrAttributeIndexMismatch = fmt.Errorf("attribute index mismatch")

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

		if col.AttributeIndex != r.Query.columns[attr] {
			return fmt.Errorf("%w: expected %d, got %d", ErrAttributeIndexMismatch, r.Query.columns[attr], col.AttributeIndex)
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
	if r.result == nil {
		return r.closeErr
	}

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

	r.err = r.Conn.Request(r.Context, msg.GEN_QUERY_AN, r.query, r.result)
	r.row = -1

	if Is(r.err, msg.CAT_NO_ROWS_FOUND) {
		r.err = nil
	}
}

func (r *Result) cleanup() {
	r.Context = context.Background() // Don't run with a canceled context

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
	if value == "" {
		return nil
	}

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

// With adds a list of conditions to the query.
func (r PreparedSingleRowQuery) With(condition ...Condition) PreparedSingleRowQuery {
	for _, c := range condition {
		r.conditions[c.Column] = fmt.Sprintf("%s %s", c.Op, c.Value)
	}

	return r
}

// Execute executes the query.
func (r PreparedSingleRowQuery) Execute(ctx context.Context) *SingleRowResult {
	result := PreparedQuery(r).Execute(ctx)

	defer result.Close()

	if result.Next() {
		return &SingleRowResult{
			result: result.result,
			Query:  r,
		}
	}

	if result.Err() != nil {
		return &SingleRowResult{
			err:   result.Err(),
			Query: r,
		}
	}

	return &SingleRowResult{
		err:   ErrNoRowFound,
		Query: r,
	}
}

type SingleRowResult struct {
	result *msg.QueryResponse
	Query  PreparedSingleRowQuery
	err    error
}

// ErrNoRowFound is returned when no rows are found in a QueryRow result.
var ErrNoRowFound = &msg.IRODSError{
	Code:    msg.CAT_NO_ROWS_FOUND,
	Message: "query returned zero rows",
}

// Scan reads the values in the current row into the values pointed
// to by dest, in order.  If an error occurs during scanning, the
// error is returned. The values pointed to by dest before the error
// occurred might be modified.
// If no rows are found, ErrNoRowFound is returned.
func (r *SingleRowResult) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}

	for attr := range dest {
		col := r.result.SQLResult[attr]
		if len(col.Values) == 0 {
			return fmt.Errorf("%w: row 1 is missing from column %d", ErrNoSQLResults, attr)
		}

		if col.AttributeIndex != r.Query.columns[attr] {
			return fmt.Errorf("%w: expected %d, got %d", ErrAttributeIndexMismatch, r.Query.columns[attr], col.AttributeIndex)
		}

		value := col.Values[0]

		if err := parseValue(value, dest[attr]); err != nil {
			return err
		}
	}

	return nil
}
