package models

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsightsExplorerSnapshotToInfoResponseWithoutData(t *testing.T) {
	snapshot := &InsightsExplorerSnapshot{
		SnapshotId:      1,
		ExplorerId:      2,
		Name:            "monthly expense",
		Data:            `{"version":1}`,
		CreatedUnixTime: 1728000000,
	}

	resp, err := snapshot.ToInsightsExplorerSnapshotInfoResponse(false)

	assert.Nil(t, err)
	assert.Equal(t, int64(1), resp.Id)
	assert.Equal(t, int64(2), resp.ExplorerId)
	assert.Equal(t, "monthly expense", resp.Name)
	assert.Equal(t, int64(1728000000), resp.CreatedUnixTime)
	assert.Nil(t, resp.Data)
}

func TestInsightsExplorerSnapshotToInfoResponseWithData(t *testing.T) {
	snapshot := &InsightsExplorerSnapshot{
		SnapshotId: 1,
		Data:       `{"version":1,"frozenTime":1728000000}`,
	}

	resp, err := snapshot.ToInsightsExplorerSnapshotInfoResponse(true)

	assert.Nil(t, err)
	assert.NotNil(t, resp.Data)
	assert.Equal(t, float64(1), resp.Data["version"])
	assert.Equal(t, float64(1728000000), resp.Data["frozenTime"])
}

func TestInsightsExplorerSnapshotToInfoResponseWithInvalidData(t *testing.T) {
	snapshot := &InsightsExplorerSnapshot{
		SnapshotId: 1,
		Data:       `invalid json`,
	}

	_, err := snapshot.ToInsightsExplorerSnapshotInfoResponse(true)
	assert.NotNil(t, err)
}

func TestInsightsExplorerSnapshotInfoResponseSliceLess(t *testing.T) {
	var snapshotRespSlice InsightsExplorerSnapshotInfoResponseSlice
	snapshotRespSlice = append(snapshotRespSlice, &InsightsExplorerSnapshotInfoResponse{
		Id:              1,
		CreatedUnixTime: 100,
	})
	snapshotRespSlice = append(snapshotRespSlice, &InsightsExplorerSnapshotInfoResponse{
		Id:              2,
		CreatedUnixTime: 300,
	})
	snapshotRespSlice = append(snapshotRespSlice, &InsightsExplorerSnapshotInfoResponse{
		Id:              3,
		CreatedUnixTime: 200,
	})

	sort.Sort(snapshotRespSlice)

	assert.Equal(t, int64(2), snapshotRespSlice[0].Id)
	assert.Equal(t, int64(3), snapshotRespSlice[1].Id)
	assert.Equal(t, int64(1), snapshotRespSlice[2].Id)
}
