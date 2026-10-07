package models

import "encoding/json/v2"

// InsightsExplorerSnapshot represents a frozen immutable snapshot of one insights exploration view
type InsightsExplorerSnapshot struct {
	SnapshotId      int64  `xorm:"PK"`
	Uid             int64  `xorm:"INDEX(IDX_insights_explorer_snapshot_uid_deleted_created) NOT NULL"`
	ExplorerId      int64  `xorm:"INDEX NOT NULL"`
	Deleted         bool   `xorm:"INDEX(IDX_insights_explorer_snapshot_uid_deleted_created) NOT NULL"`
	Name            string `xorm:"VARCHAR(128) NOT NULL"`
	Data            string `xorm:"MEDIUMBLOB"`
	CreatedUnixTime int64  `xorm:"INDEX(IDX_insights_explorer_snapshot_uid_deleted_created)"`
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// InsightsExplorerSnapshotCreateRequest represents all parameters of exploration snapshot creation request
type InsightsExplorerSnapshotCreateRequest struct {
	Name       string         `json:"name" binding:"required,notBlank,max=128"`
	ExplorerId int64          `json:"explorerId,string" binding:"min=0"`
	Data       map[string]any `json:"data" binding:"required"`
}

// InsightsExplorerSnapshotGetRequest represents all parameters of exploration snapshot getting request
type InsightsExplorerSnapshotGetRequest struct {
	Id int64 `form:"id,string" binding:"required,min=1"`
}

// InsightsExplorerSnapshotListRequest represents all parameters of exploration snapshot listing request
type InsightsExplorerSnapshotListRequest struct {
	ExplorerId int64 `form:"explorerId,string" binding:"min=0"`
}

// InsightsExplorerSnapshotDeleteRequest represents all parameters of exploration snapshot deleting request
type InsightsExplorerSnapshotDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// InsightsExplorerSnapshotInfoResponse represents a view-object of exploration snapshot
type InsightsExplorerSnapshotInfoResponse struct {
	Id              int64          `json:"id,string"`
	ExplorerId      int64          `json:"explorerId,string"`
	Name            string         `json:"name"`
	CreatedUnixTime int64          `json:"createdUnixTime"`
	Data            map[string]any `json:"data,omitempty"`
}

// ToInsightsExplorerSnapshotInfoResponse returns a view-object according to database model
func (a *InsightsExplorerSnapshot) ToInsightsExplorerSnapshotInfoResponse(withData bool) (*InsightsExplorerSnapshotInfoResponse, error) {
	var data map[string]any = nil

	if withData && a.Data != "" {
		err := json.Unmarshal([]byte(a.Data), &data)

		if err != nil {
			return nil, err
		}
	}

	return &InsightsExplorerSnapshotInfoResponse{
		Id:              a.SnapshotId,
		ExplorerId:      a.ExplorerId,
		Name:            a.Name,
		CreatedUnixTime: a.CreatedUnixTime,
		Data:            data,
	}, nil
}

// InsightsExplorerSnapshotInfoResponseSlice represents the slice data structure of InsightsExplorerSnapshotInfoResponse
type InsightsExplorerSnapshotInfoResponseSlice []*InsightsExplorerSnapshotInfoResponse

// Len returns the count of items
func (s InsightsExplorerSnapshotInfoResponseSlice) Len() int {
	return len(s)
}

// Swap swaps two items
func (s InsightsExplorerSnapshotInfoResponseSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less reports whether the first item is less than the second item
func (s InsightsExplorerSnapshotInfoResponseSlice) Less(i, j int) bool {
	if s[i].CreatedUnixTime != s[j].CreatedUnixTime {
		return s[i].CreatedUnixTime > s[j].CreatedUnixTime
	}

	return s[i].Id > s[j].Id
}
