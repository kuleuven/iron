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
	api         *api
	ResultLimit int
	MaxRows     int
	Columns     []msg.ColumnNumber
	Conditions  map[msg.ColumnNumber]string
	AsAdmin     bool
}

func (api *api) Query(columns ...msg.ColumnNumber) PreparedQuery {
	return PreparedQuery{
		api:        api,
		Columns:    columns,
		MaxRows:    500,
		Conditions: make(map[msg.ColumnNumber]string),
	}
}

func (q PreparedQuery) Where(column msg.ColumnNumber, condition string) PreparedQuery {
	q.Conditions[column] = condition

	return q
}

func (q PreparedQuery) Limit(limit int) PreparedQuery {
	q.ResultLimit = limit

	if q.MaxRows > limit && limit > 0 {
		q.MaxRows = limit
	}

	return q
}

func (q PreparedQuery) Admin() PreparedQuery {
	q.AsAdmin = true

	return q
}

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

func (r *Result) Err() error {
	return r.err
}

func (r *Result) Next() bool {
	if r.err != nil {
		return false
	}

	if r.result.RowCount == 0 {
		r.cleanup()

		return false
	}

	r.row++

	if r.row >= r.Query.ResultLimit && r.Query.ResultLimit > 0 {
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
	r.Query.ResultLimit -= r.result.RowCount

	r.executeQuery()

	return r.Next()
}

var ErrRowOutOfBound = fmt.Errorf("row out of bound")

var ErrAttributeOutOfBound = fmt.Errorf("attribute count out of bound")

var ErrNoSQLResults = fmt.Errorf("no sql results")

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

		if err := r.parseValue(value, dest[attr]); err != nil {
			return err
		}
	}

	return nil
}

func (r *Result) Close() error {
	r.cleanup()

	return r.closeErr
}

func (r *Result) buildQuery() {
	r.query = &msg.QueryRequest{
		MaxRows: r.Query.MaxRows,
		Options: 0x20,
	}

	for _, col := range r.Query.Columns {
		r.query.Selects.Add(int(col), 1)
	}

	for col, condition := range r.Query.Conditions {
		r.query.Conditions.Add(int(col), condition)
	}

	if r.Query.AsAdmin {
		r.query.KeyVals.Add(msg.ADMIN_KW, "true")
	}
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
	if r.result.ContinueIndex != 0 {
		r.query.ContinueIndex = r.result.ContinueIndex
		r.query.MaxRows = 0

		r.executeQuery()
	}

	if r.Conn != nil {
		r.closeErr = r.Conn.Close()
		r.Conn = nil
	}
}

func (r *Result) parseValue(value string, dest interface{}) error {
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
