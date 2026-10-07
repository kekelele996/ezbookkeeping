package errs

import "net/http"

// Error codes related to insights explorer
var (
	ErrInsightsExplorerIdInvalid           = NewNormalError(NormalSubcategoryInsightsExplorer, 0, http.StatusBadRequest, "exploration id is invalid")
	ErrInsightsExplorerNotFound            = NewNormalError(NormalSubcategoryInsightsExplorer, 1, http.StatusBadRequest, "exploration not found")
	ErrInsightsExplorerDataInvalid         = NewNormalError(NormalSubcategoryInsightsExplorer, 2, http.StatusBadRequest, "exploration data is invalid")
	ErrInsightsExplorerSnapshotIdInvalid   = NewNormalError(NormalSubcategoryInsightsExplorer, 3, http.StatusBadRequest, "exploration snapshot id is invalid")
	ErrInsightsExplorerSnapshotNotFound    = NewNormalError(NormalSubcategoryInsightsExplorer, 4, http.StatusBadRequest, "exploration snapshot not found")
	ErrInsightsExplorerSnapshotDataInvalid = NewNormalError(NormalSubcategoryInsightsExplorer, 5, http.StatusBadRequest, "exploration snapshot data is invalid")
)
