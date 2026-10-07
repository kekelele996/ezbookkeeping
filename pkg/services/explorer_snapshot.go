package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// InsightsExplorerSnapshotService represents insights explorer snapshot service
type InsightsExplorerSnapshotService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize a insights explorer snapshot service singleton instance
var (
	InsightsExplorerSnapshots = &InsightsExplorerSnapshotService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetAllSnapshotsByUid returns all exploration snapshots of user, snapshots of specified exploration are returned if explorerId is greater than 0
func (s *InsightsExplorerSnapshotService) GetAllSnapshotsByUid(c core.Context, uid int64, explorerId int64) ([]*models.InsightsExplorerSnapshot, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var snapshots []*models.InsightsExplorerSnapshot
	query := s.UserDataDB(uid).NewSession(c).Select("snapshot_id, uid, explorer_id, name, created_unix_time").Where("uid=? AND deleted=?", uid, false)

	if explorerId > 0 {
		query = query.Where("explorer_id=?", explorerId)
	}

	err := query.OrderBy("created_unix_time desc, snapshot_id desc").Find(&snapshots)

	return snapshots, err
}

// GetSnapshotBySnapshotId returns a exploration snapshot model according to snapshot id
func (s *InsightsExplorerSnapshotService) GetSnapshotBySnapshotId(c core.Context, uid int64, snapshotId int64) (*models.InsightsExplorerSnapshot, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if snapshotId <= 0 {
		return nil, errs.ErrInsightsExplorerSnapshotIdInvalid
	}

	snapshot := &models.InsightsExplorerSnapshot{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(snapshotId).Where("uid=? AND deleted=?", uid, false).Get(snapshot)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrInsightsExplorerSnapshotNotFound
	}

	return snapshot, nil
}

// CreateSnapshot saves a new exploration snapshot model to database
func (s *InsightsExplorerSnapshotService) CreateSnapshot(c core.Context, snapshot *models.InsightsExplorerSnapshot) error {
	if snapshot.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	snapshot.SnapshotId = s.GenerateUuid(uuid.UUID_TYPE_EXPLORER_SNAPSHOT)

	if snapshot.SnapshotId < 1 {
		return errs.ErrSystemIsBusy
	}

	snapshot.Deleted = false
	snapshot.CreatedUnixTime = time.Now().Unix()
	snapshot.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(snapshot.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(snapshot)
		return err
	})
}

// DeleteSnapshot deletes an existed exploration snapshot from database
func (s *InsightsExplorerSnapshotService) DeleteSnapshot(c core.Context, uid int64, snapshotId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if snapshotId <= 0 {
		return errs.ErrInsightsExplorerSnapshotIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.InsightsExplorerSnapshot{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.ID(snapshotId).Cols("deleted", "deleted_unix_time", "updated_unix_time").Where("uid=? AND deleted=?", uid, false).Update(updateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrInsightsExplorerSnapshotNotFound
		}

		return err
	})
}

// DeleteAllSnapshots deletes all existed exploration snapshots from database
func (s *InsightsExplorerSnapshotService) DeleteAllSnapshots(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.InsightsExplorerSnapshot{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Cols("deleted", "deleted_unix_time", "updated_unix_time").Where("uid=? AND deleted=?", uid, false).Update(updateModel)

		if err != nil {
			return err
		}

		return nil
	})
}
