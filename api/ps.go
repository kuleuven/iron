package api

import (
	"context"

	"github.com/kuleuven/iron/msg"
)

// Procs returns a list of processes with columns
// ICAT_COLUMN_PROCESS_ID, ICAT_COLUMN_STARTTIME, ICAT_COLUMN_PROXY_NAME,
// ICAT_COLUMN_PROXY_ZONE, ICAT_COLUMN_CLIENT_NAME, ICAT_COLUMN_CLIENT_ZONE,
// ICAT_COLUMN_REMOTE_ADDR, ICAT_COLUMN_SERVER_ADDR, ICAT_COLUMN_PROG_NAME.
func (api *API) Procs(ctx context.Context) *Result {
	req := msg.ProcStatRequest{}

	var resp msg.QueryResponse

	if err := api.Request(ctx, msg.PROC_STAT_AN, req, &resp); err != nil {
		return &Result{err: err}
	}

	result := &Result{
		Context: ctx,
		result:  &resp,
	}

	// Initialize columns after the fact,
	// because this is checked in Scan().
	result.Query.columns = result.Columns()

	return result
}
