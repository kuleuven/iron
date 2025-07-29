package transfer

type Progress interface {
	// AddTotalFiles is called to increase the total number of files. Must be multithread-safe.
	AddTotalFiles(n int)

	// AddTransferredFiles is called to increase the number of transferred files. Must be multithread-safe.
	AddTransferredFiles(n int)

	// AddTotalBytes is called to increase the total number of bytes. Must be multithread-safe.
	AddTotalBytes(bytes int64)

	// AddTransferredBytes is called to increase number of transferred bytes. Must be multithread-safe.
	AddTransferredBytes(bytes int64)
}

type NullProgress struct{}

func (NullProgress) AddTotalFiles(n int) {
	// Do nothing
}

func (NullProgress) AddTransferredFiles(n int) {
	// Do nothing
}

func (NullProgress) AddTotalBytes(bytes int64) {
	// Do nothing
}

func (NullProgress) AddTransferredBytes(bytes int64) {
	// Do nothing
}
