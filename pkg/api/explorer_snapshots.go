package api

import (
	"encoding/json/v2"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

// InsightsExplorerSnapshotsApi represents insights explorer snapshot api
type InsightsExplorerSnapshotsApi struct {
	insightsExplorerSnapshots *services.InsightsExplorerSnapshotService
}

// Initialize a insights explorer snapshot api singleton instance
var (
	InsightsExplorerSnapshots = &InsightsExplorerSnapshotsApi{
		insightsExplorerSnapshots: services.InsightsExplorerSnapshots,
	}
)

// InsightsExplorerSnapshotListHandler returns exploration snapshot list of current user
func (a *InsightsExplorerSnapshotsApi) InsightsExplorerSnapshotListHandler(c *core.WebContext) (any, *errs.Error) {
	var snapshotListReq models.InsightsExplorerSnapshotListRequest
	err := c.ShouldBindQuery(&snapshotListReq)

	if err != nil {
		log.Warnf(c, "[explorer_snapshots.InsightsExplorerSnapshotListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	snapshots, err := a.insightsExplorerSnapshots.GetAllSnapshotsByUid(c, uid, snapshotListReq.ExplorerId)

	if err != nil {
		log.Errorf(c, "[explorer_snapshots.InsightsExplorerSnapshotListHandler] failed to get exploration snapshots for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	snapshotResps := make(models.InsightsExplorerSnapshotInfoResponseSlice, len(snapshots))

	for i := 0; i < len(snapshots); i++ {
		snapshotResps[i], err = snapshots[i].ToInsightsExplorerSnapshotInfoResponse(false)

		if err != nil {
			log.Errorf(c, "[explorer_snapshots.InsightsExplorerSnapshotListHandler] failed to get exploration snapshot response for user \"uid:%d\", because %s", uid, err.Error())
			return nil, errs.ErrInsightsExplorerSnapshotDataInvalid
		}
	}

	return snapshotResps, nil
}

// InsightsExplorerSnapshotGetHandler returns one specific exploration snapshot of current user
func (a *InsightsExplorerSnapshotsApi) InsightsExplorerSnapshotGetHandler(c *core.WebContext) (any, *errs.Error) {
	var snapshotGetReq models.InsightsExplorerSnapshotGetRequest
	err := c.ShouldBindQuery(&snapshotGetReq)

	if err != nil {
		log.Warnf(c, "[explorer_snapshots.InsightsExplorerSnapshotGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	snapshot, err := a.insightsExplorerSnapshots.GetSnapshotBySnapshotId(c, uid, snapshotGetReq.Id)

	if err != nil {
		log.Errorf(c, "[explorer_snapshots.InsightsExplorerSnapshotGetHandler] failed to get exploration snapshot \"id:%d\" for user \"uid:%d\", because %s", snapshotGetReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	snapshotResp, err := snapshot.ToInsightsExplorerSnapshotInfoResponse(true)

	if err != nil {
		log.Errorf(c, "[explorer_snapshots.InsightsExplorerSnapshotGetHandler] failed to get exploration snapshot response for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrInsightsExplorerSnapshotDataInvalid
	}

	return snapshotResp, nil
}

// InsightsExplorerSnapshotCreateHandler saves a new exploration snapshot by request parameters for current user
func (a *InsightsExplorerSnapshotsApi) InsightsExplorerSnapshotCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var snapshotCreateReq models.InsightsExplorerSnapshotCreateRequest
	err := c.ShouldBindJSON(&snapshotCreateReq)

	if err != nil {
		log.Warnf(c, "[explorer_snapshots.InsightsExplorerSnapshotCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	snapshot, err := a.createNewInsightsExplorerSnapshotModel(uid, &snapshotCreateReq)

	if err != nil {
		log.Errorf(c, "[explorer_snapshots.InsightsExplorerSnapshotCreateHandler] failed to parse exploration snapshot data for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrInsightsExplorerSnapshotDataInvalid
	}

	err = a.insightsExplorerSnapshots.CreateSnapshot(c, snapshot)

	if err != nil {
		log.Errorf(c, "[explorer_snapshots.InsightsExplorerSnapshotCreateHandler] failed to create exploration snapshot \"id:%d\" for user \"uid:%d\", because %s", snapshot.SnapshotId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[explorer_snapshots.InsightsExplorerSnapshotCreateHandler] user \"uid:%d\" has created a new exploration snapshot \"id:%d\" successfully", uid, snapshot.SnapshotId)

	snapshotResp, err := snapshot.ToInsightsExplorerSnapshotInfoResponse(false)

	if err != nil {
		log.Errorf(c, "[explorer_snapshots.InsightsExplorerSnapshotCreateHandler] failed to get exploration snapshot response for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrInsightsExplorerSnapshotDataInvalid
	}

	return snapshotResp, nil
}

// InsightsExplorerSnapshotDeleteHandler deletes an existed exploration snapshot by request parameters for current user
func (a *InsightsExplorerSnapshotsApi) InsightsExplorerSnapshotDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var snapshotDeleteReq models.InsightsExplorerSnapshotDeleteRequest
	err := c.ShouldBindJSON(&snapshotDeleteReq)

	if err != nil {
		log.Warnf(c, "[explorer_snapshots.InsightsExplorerSnapshotDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.insightsExplorerSnapshots.DeleteSnapshot(c, uid, snapshotDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[explorer_snapshots.InsightsExplorerSnapshotDeleteHandler] failed to delete exploration snapshot \"id:%d\" for user \"uid:%d\", because %s", snapshotDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[explorer_snapshots.InsightsExplorerSnapshotDeleteHandler] user \"uid:%d\" has deleted exploration snapshot \"id:%d\"", uid, snapshotDeleteReq.Id)
	return true, nil
}

func (a *InsightsExplorerSnapshotsApi) createNewInsightsExplorerSnapshotModel(uid int64, snapshotCreateReq *models.InsightsExplorerSnapshotCreateRequest) (*models.InsightsExplorerSnapshot, error) {
	data, err := json.Marshal(snapshotCreateReq.Data)

	if err != nil {
		return nil, err
	}

	return &models.InsightsExplorerSnapshot{
		Uid:        uid,
		ExplorerId: snapshotCreateReq.ExplorerId,
		Name:       snapshotCreateReq.Name,
		Data:       string(data),
	}, nil
}
