package api

import (
	"context"
	"encoding/json"

	"github.com/kuleuven/iron/msg"
)

// GenericQuery prepares a genquery2 query
func (api *API) GenericQuery(query string) GenericQuery {
	return GenericQuery{
		api:   api,
		query: query,
	}
}

// GenericQuery prepares a genquery2 query
func (api *API) GenericQueryRow(query string) GenericSingleRowQuery {
	return GenericSingleRowQuery{
		api:   api,
		query: query,
	}
}

// GenericQueryColumns returns the possible columns
func (api *API) GenericQueryColumns(ctx context.Context) ([]string, error) {
	req := msg.GenQuery2Request{
		Zone:           api.Zone,
		ColumnMappings: 1,
	}

	var resp msg.String

	if err := api.Request(ctx, msg.GENQUERY2_AN, req, &resp); err != nil {
		return nil, err
	}

	results := map[string]any{}

	if err := json.Unmarshal([]byte(resp.String), &results); err != nil {
		return nil, err
	}

	var fields []string

	for k := range results {
		fields = append(fields, k)
	}

	return fields, nil
}

type GenericQuery struct {
	api   *API
	query string
}

func (gq GenericQuery) SQL(ctx context.Context) (string, error) {
	req := msg.GenQuery2Request{
		Query:   gq.query,
		Zone:    gq.api.Zone,
		SQLOnly: 1,
	}

	var resp msg.String

	err := gq.api.Request(ctx, msg.GENQUERY2_AN, req, &resp)

	return resp.String, err
}

func (gq GenericQuery) Execute(ctx context.Context) *GenericResult {
	req := msg.GenQuery2Request{
		Query: gq.query,
		Zone:  gq.api.Zone,
	}

	var resp msg.String

	if err := gq.api.Request(ctx, msg.GENQUERY2_AN, req, &resp); err != nil {
		return &GenericResult{err: err}
	}

	rows := [][]string{}

	if err := json.Unmarshal([]byte(resp.String), &rows); err != nil {
		return &GenericResult{err: err}
	}

	return &GenericResult{
		rows: rows,
	}
}

var _ QueryResult = (*GenericResult)(nil)

type GenericResult struct {
	rows [][]string
	row  []string
	err  error
}

func (gr *GenericResult) Err() error {
	return gr.err
}

func (gr *GenericResult) Next() bool {
	if len(gr.rows) == 0 {
		gr.row = nil

		return false
	}

	gr.row = gr.rows[0]
	gr.rows = gr.rows[1:]

	return true
}

func (gr *GenericResult) Scan(dest ...any) error {
	if gr.err != nil {
		return gr.err
	}

	if gr.row == nil {
		return ErrRowOutOfBound
	}

	if len(dest) > len(gr.row) {
		return ErrAttributeOutOfBound
	}

	for i := range dest {
		if err := parseValue(gr.row[i], dest[i]); err != nil {
			return err
		}
	}

	return nil
}

func (gr *GenericResult) Close() error {
	gr.rows = nil

	return nil
}

type GenericSingleRowQuery GenericQuery

func (gqr GenericSingleRowQuery) SQL(ctx context.Context) (string, error) {
	return GenericQuery(gqr).SQL(ctx)
}

func (gqr GenericSingleRowQuery) Execute(ctx context.Context) *GenericSingleRowResult {
	result := GenericQuery(gqr).Execute(ctx)

	defer result.Close()

	if result.Next() {
		return &GenericSingleRowResult{
			row: result.row,
		}
	}

	if result.Err() != nil {
		return &GenericSingleRowResult{
			err: result.Err(),
		}
	}

	return &GenericSingleRowResult{
		err: ErrNoRowFound,
	}
}

type GenericSingleRowResult struct {
	err error
	row []string
}

func (grr *GenericSingleRowResult) Scan(dest ...any) error {
	if grr.err != nil {
		return grr.err
	}

	if len(dest) > len(grr.row) {
		return ErrAttributeOutOfBound
	}

	for i := range dest {
		if err := parseValue(grr.row[i], dest[i]); err != nil {
			return err
		}
	}

	return nil
}
