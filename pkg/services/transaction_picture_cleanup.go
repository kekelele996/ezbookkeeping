package services

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/storage"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// TransactionPictureCleanupCategory represents the classification category of a transaction picture file in the cleanup analysis
type TransactionPictureCleanupCategory int8

const (
	// TransactionPictureCleanupCategoryReferenced means the file is still referenced by an active transaction and must be kept
	TransactionPictureCleanupCategoryReferenced TransactionPictureCleanupCategory = iota + 1
	// TransactionPictureCleanupCategoryRecentPending means the file may still be in use (recently uploaded or waiting to be attached to a transaction) and must be kept
	TransactionPictureCleanupCategoryRecentPending
	// TransactionPictureCleanupCategoryCleanable means the file is confirmed to be useless and can be deleted safely
	TransactionPictureCleanupCategoryCleanable
	// TransactionPictureCleanupCategoryUnconfirmed means whether the file can be deleted cannot be confirmed, and it must not be deleted automatically
	TransactionPictureCleanupCategoryUnconfirmed
)

// Classification reason codes. The classification result of a file is monotonic, that means a file classified as cleanable can never become referenced again.
const (
	// The file is referenced by a non-deleted picture record attached to a transaction
	TransactionPictureCleanupReasonAttachedToTransaction = "attached_to_transaction"
	// The file belongs to a non-deleted new picture record which is still within the recent safety window and may be attached to a transaction later
	TransactionPictureCleanupReasonPendingNewPicture = "pending_new_picture"
	// The file has no database record but its modification time is within the recent safety window, it may be created by an upload in progress
	TransactionPictureCleanupReasonPossiblyUploadingOrphan = "possibly_uploading_orphan_file"
	// The file belongs to a soft-deleted picture record and the file can never be accessed again
	TransactionPictureCleanupReasonSoftDeleted = "soft_deleted_picture"
	// The file has no database record and is older than the recent safety window, so it can only be a leftover of an interrupted or failed upload
	TransactionPictureCleanupReasonOrphanFile = "orphan_file"
	// The file belongs to a non-deleted new picture record but the record is older than the recent safety window, whether it is still useful cannot be confirmed
	TransactionPictureCleanupReasonExpiredPendingNewPicture = "expired_pending_new_picture"
	// A non-deleted picture record exists but its file does not exist, it is a data inconsistency and needs manual confirmation
	TransactionPictureCleanupReasonActiveRecordMissingFile = "active_record_missing_file"
	// The storage path of the file does not conform to the uid/picture_id.extension structure and whether it is a transaction picture cannot be confirmed
	TransactionPictureCleanupReasonInvalidStoragePath = "invalid_storage_path"
	// The file extension does not match the extension in the picture record and whether it is the correct file cannot be confirmed
	TransactionPictureCleanupReasonExtensionMismatch = "picture_record_extension_mismatch"
	// The user id in the picture record does not match the user id directory in which the file is stored
	TransactionPictureCleanupReasonUserMismatch = "picture_record_user_mismatch"
	// The file has no database record, but its last modification time is unknown, so whether it is still being uploaded cannot be confirmed
	TransactionPictureCleanupReasonOrphanModifiedTimeUnknown = "orphan_file_modified_time_unknown"
)

// TransactionPictureCleanupItem represents a single file or database record involved in the cleanup analysis
type TransactionPictureCleanupItem struct {
	Uid                  int64
	PictureId            int64
	PictureExtension     string
	StoragePath          string
	Size                 int64
	LastModifiedUnixTime int64
	Category             TransactionPictureCleanupCategory
	Reason               string
}

// TransactionPictureCleanupCategoryStats represents the aggregated count and occupied size of a category or reason
type TransactionPictureCleanupCategoryStats struct {
	Count     int64
	TotalSize int64
}

func (s *TransactionPictureCleanupCategoryStats) addItem(size int64) {
	s.Count++
	s.TotalSize += size
}

// TransactionPictureCleanupReport represents the full result of a transaction picture cleanup analysis
type TransactionPictureCleanupReport struct {
	RecentSafetyDuration time.Duration
	ScannedFileCount     int64
	Referenced           TransactionPictureCleanupCategoryStats
	RecentPending        TransactionPictureCleanupCategoryStats
	Cleanable            TransactionPictureCleanupCategoryStats
	Unconfirmed          TransactionPictureCleanupCategoryStats
	ReasonStats          map[string]*TransactionPictureCleanupCategoryStats
	Items                []*TransactionPictureCleanupItem
	// SoftDeletedRecordMissingFileCount is the count of soft-deleted picture records whose files already do not exist, there is nothing to clean for these records
	SoftDeletedRecordMissingFileCount int64
}

// TransactionPictureCleanupExecuteResult represents the result of a cleanup execution
type TransactionPictureCleanupExecuteResult struct {
	DeletedFileCount int64
	DeletedFileSize  int64
	FailedFileCount  int64
	FailedItems      []*TransactionPictureCleanupItem
}

// AnalyzeTransactionPictures scans the transaction picture storage and classifies all files into cleanup categories
func (s *TransactionPictureService) AnalyzeTransactionPictures(c core.Context, uid int64, recentSafetyDuration time.Duration) (*TransactionPictureCleanupReport, error) {
	if uid < 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if recentSafetyDuration < 0 {
		recentSafetyDuration = 0
	}

	listPrefix := ""

	if uid > 0 {
		listPrefix = utils.Int64ToString(uid) + "/"
	}

	objects, err := s.ListTransactionPictures(c, listPrefix)

	if err != nil {
		log.Errorf(c, "[transaction_pictures.AnalyzeTransactionPictures] failed to list transaction picture objects for user \"uid:%d\", because %s", uid, err.Error())
		return nil, err
	}

	pictureInfoProvider := func(c core.Context, pictureUid int64) (map[int64]*models.TransactionPictureInfo, error) {
		pictureInfos, err := s.GetAllPictureInfosByUid(c, pictureUid)

		if err != nil {
			return nil, err
		}

		return s.GetPictureInfoMapByList(pictureInfos), nil
	}

	return buildTransactionPictureCleanupReport(c, objects, uid, time.Now(), recentSafetyDuration, pictureInfoProvider)
}

// ExecuteTransactionPicturesCleanup re-analyzes the storage and deletes only the files classified as cleanable.
// The analysis is always performed immediately before deletion in the same execution, and deleting a non-existent file is treated as success, so the execution can be interrupted and retried safely.
// The returned report reflects the state after deletion (a final re-analysis is performed), so it can be used to verify that the result is consistent with a re-run.
func (s *TransactionPictureService) ExecuteTransactionPicturesCleanup(c core.Context, uid int64, recentSafetyDuration time.Duration) (*TransactionPictureCleanupExecuteResult, *TransactionPictureCleanupReport, error) {
	report, err := s.AnalyzeTransactionPictures(c, uid, recentSafetyDuration)

	if err != nil {
		return nil, nil, err
	}

	result := &TransactionPictureCleanupExecuteResult{
		FailedItems: make([]*TransactionPictureCleanupItem, 0),
	}

	for i := 0; i < len(report.Items); i++ {
		item := report.Items[i]

		if item.Category != TransactionPictureCleanupCategoryCleanable {
			continue
		}

		if item.PictureId <= 0 || item.PictureExtension == "" {
			result.FailedFileCount++
			result.FailedItems = append(result.FailedItems, item)
			continue
		}

		err := s.DeleteTransactionPicture(c, item.Uid, item.PictureId, item.PictureExtension)

		if err != nil {
			log.Errorf(c, "[transaction_pictures.ExecuteTransactionPicturesCleanup] failed to delete cleanable transaction picture \"uid:%d,pictureId:%d,path:%s\", because %s", item.Uid, item.PictureId, item.StoragePath, err.Error())
			result.FailedFileCount++
			result.FailedItems = append(result.FailedItems, item)
			continue
		}

		log.CliInfof(c, "[transaction_pictures.ExecuteTransactionPicturesCleanup] deleted cleanable transaction picture \"uid:%d,pictureId:%d,reason:%s,size:%d\"", item.Uid, item.PictureId, item.Reason, item.Size)
		result.DeletedFileCount++
		result.DeletedFileSize += item.Size
	}

	// Re-analyze after deletion so that the returned report reflects the final state even when the execution was interrupted or partially failed.
	finalReport, finalErr := s.AnalyzeTransactionPictures(c, uid, recentSafetyDuration)

	if finalErr != nil {
		return result, nil, finalErr
	}

	return result, finalReport, nil
}

type transactionPictureInfoProvider func(c core.Context, uid int64) (map[int64]*models.TransactionPictureInfo, error)

func buildTransactionPictureCleanupReport(c core.Context, objects []storage.ObjectMeta, uidFilter int64, now time.Time, recentSafetyDuration time.Duration, pictureInfoProvider transactionPictureInfoProvider) (*TransactionPictureCleanupReport, error) {
	report := &TransactionPictureCleanupReport{
		RecentSafetyDuration: recentSafetyDuration,
		ReasonStats:          make(map[string]*TransactionPictureCleanupCategoryStats),
		Items:                make([]*TransactionPictureCleanupItem, 0, len(objects)),
	}

	recentSafetyBeforeTime := now.Add(-recentSafetyDuration)

	sortedObjects := make([]storage.ObjectMeta, len(objects))
	copy(sortedObjects, objects)
	sort.Slice(sortedObjects, func(i int, j int) bool {
		return sortedObjects[i].Path < sortedObjects[j].Path
	})

	pictureInfoCache := make(map[int64]map[int64]*models.TransactionPictureInfo)
	presentFileKeys := make(map[int64]map[int64]bool)
	parsedUids := make(map[int64]bool)

	ensurePictureInfoMap := func(pictureUid int64) (map[int64]*models.TransactionPictureInfo, error) {
		if pictureInfoMap, exists := pictureInfoCache[pictureUid]; exists {
			return pictureInfoMap, nil
		}

		pictureInfoMap, err := pictureInfoProvider(c, pictureUid)

		if err != nil {
			return nil, err
		}

		pictureInfoCache[pictureUid] = pictureInfoMap
		return pictureInfoMap, nil
	}

	addItem := func(item *TransactionPictureCleanupItem) {
		report.Items = append(report.Items, item)
		report.addItemToCategoryStats(item)
	}

	for i := 0; i < len(sortedObjects); i++ {
		object := &sortedObjects[i]
		report.ScannedFileCount++

		pictureUid, pictureId, pictureExtension, pathValid := parseTransactionPictureObjectPath(object.Path, uidFilter)

		if !pathValid {
			addItem(&TransactionPictureCleanupItem{
				StoragePath:          object.Path,
				Size:                 object.Size,
				LastModifiedUnixTime: getLastModifiedUnixTime(object),
				Category:             TransactionPictureCleanupCategoryUnconfirmed,
				Reason:               TransactionPictureCleanupReasonInvalidStoragePath,
			})
			continue
		}

		parsedUids[pictureUid] = true

		if _, exists := presentFileKeys[pictureUid]; !exists {
			presentFileKeys[pictureUid] = make(map[int64]bool)
		}

		presentFileKeys[pictureUid][pictureId] = true

		pictureInfoMap, err := ensurePictureInfoMap(pictureUid)

		if err != nil {
			return nil, err
		}

		category, reason := classifyListedTransactionPicture(pictureUid, pictureId, pictureExtension, object.LastModified, pictureInfoMap, recentSafetyBeforeTime)

		addItem(&TransactionPictureCleanupItem{
			Uid:                  pictureUid,
			PictureId:            pictureId,
			PictureExtension:     pictureExtension,
			StoragePath:          object.Path,
			Size:                 object.Size,
			LastModifiedUnixTime: getLastModifiedUnixTime(object),
			Category:             category,
			Reason:               reason,
		})
	}

	for pictureUid := range parsedUids {
		pictureInfoMap := pictureInfoCache[pictureUid]
		presentPictureIds := presentFileKeys[pictureUid]

		for pictureId, pictureInfo := range pictureInfoMap {
			if presentPictureIds[pictureId] {
				continue
			}

			expectedPath := getTransactionPictureStoragePath(pictureUid, pictureId, pictureInfo.PictureExtension)

			if pictureInfo.Deleted {
				report.SoftDeletedRecordMissingFileCount++
				continue
			}

			addItem(&TransactionPictureCleanupItem{
				Uid:              pictureUid,
				PictureId:        pictureId,
				PictureExtension: pictureInfo.PictureExtension,
				StoragePath:      expectedPath,
				Category:         TransactionPictureCleanupCategoryUnconfirmed,
				Reason:           TransactionPictureCleanupReasonActiveRecordMissingFile,
			})
		}
	}

	sort.Slice(report.Items, func(i int, j int) bool {
		if report.Items[i].Uid != report.Items[j].Uid {
			return report.Items[i].Uid < report.Items[j].Uid
		}

		return report.Items[i].StoragePath < report.Items[j].StoragePath
	})

	return report, nil
}

func (r *TransactionPictureCleanupReport) addItemToCategoryStats(item *TransactionPictureCleanupItem) {
	var categoryStats *TransactionPictureCleanupCategoryStats

	switch item.Category {
	case TransactionPictureCleanupCategoryReferenced:
		categoryStats = &r.Referenced
	case TransactionPictureCleanupCategoryRecentPending:
		categoryStats = &r.RecentPending
	case TransactionPictureCleanupCategoryCleanable:
		categoryStats = &r.Cleanable
	case TransactionPictureCleanupCategoryUnconfirmed:
		categoryStats = &r.Unconfirmed
	default:
		return
	}

	categoryStats.addItem(item.Size)

	reasonStats, exists := r.ReasonStats[item.Reason]

	if !exists {
		reasonStats = &TransactionPictureCleanupCategoryStats{}
		r.ReasonStats[item.Reason] = reasonStats
	}

	reasonStats.addItem(item.Size)
}

func classifyListedTransactionPicture(uid int64, pictureId int64, pictureExtension string, fileLastModifiedTime time.Time, pictureInfoMap map[int64]*models.TransactionPictureInfo, recentSafetyBeforeTime time.Time) (TransactionPictureCleanupCategory, string) {
	pictureInfo, exists := pictureInfoMap[pictureId]

	if !exists {
		if fileLastModifiedTime.IsZero() {
			return TransactionPictureCleanupCategoryUnconfirmed, TransactionPictureCleanupReasonOrphanModifiedTimeUnknown
		}

		if !fileLastModifiedTime.Before(recentSafetyBeforeTime) {
			return TransactionPictureCleanupCategoryRecentPending, TransactionPictureCleanupReasonPossiblyUploadingOrphan
		}

		return TransactionPictureCleanupCategoryCleanable, TransactionPictureCleanupReasonOrphanFile
	}

	if pictureInfo.Uid != uid {
		return TransactionPictureCleanupCategoryUnconfirmed, TransactionPictureCleanupReasonUserMismatch
	}

	if pictureInfo.PictureExtension != pictureExtension {
		return TransactionPictureCleanupCategoryUnconfirmed, TransactionPictureCleanupReasonExtensionMismatch
	}

	if pictureInfo.Deleted {
		return TransactionPictureCleanupCategoryCleanable, TransactionPictureCleanupReasonSoftDeleted
	}

	if pictureInfo.TransactionId == models.TransactionPictureNewPictureTransactionId {
		if pictureInfo.CreatedUnixTime >= recentSafetyBeforeTime.Unix() {
			return TransactionPictureCleanupCategoryRecentPending, TransactionPictureCleanupReasonPendingNewPicture
		}

		return TransactionPictureCleanupCategoryUnconfirmed, TransactionPictureCleanupReasonExpiredPendingNewPicture
	}

	return TransactionPictureCleanupCategoryReferenced, TransactionPictureCleanupReasonAttachedToTransaction
}

func parseTransactionPictureObjectPath(objectPath string, uidFilter int64) (uid int64, pictureId int64, pictureExtension string, valid bool) {
	pathParts := strings.Split(objectPath, "/")

	if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] == "" {
		return 0, 0, "", false
	}

	uid, err := strconv.ParseInt(pathParts[0], 10, 64)

	if err != nil || uid <= 0 {
		return 0, 0, "", false
	}

	if uidFilter > 0 && uid != uidFilter {
		return 0, 0, "", false
	}

	fileName := pathParts[1]
	pictureExtension = utils.GetFileNameExtension(fileName)

	if utils.GetImageContentType(pictureExtension) == "" {
		return 0, 0, "", false
	}

	fileBaseName := utils.GetFileNameWithoutExtension(fileName)
	pictureId, err = strconv.ParseInt(fileBaseName, 10, 64)

	if err != nil || pictureId <= 0 {
		return 0, 0, "", false
	}

	return uid, pictureId, pictureExtension, true
}

func getTransactionPictureStoragePath(uid int64, pictureId int64, pictureExtension string) string {
	return utils.Int64ToString(uid) + "/" + utils.Int64ToString(pictureId) + "." + pictureExtension
}

func getLastModifiedUnixTime(object *storage.ObjectMeta) int64 {
	if object.LastModified.IsZero() {
		return 0
	}

	return object.LastModified.Unix()
}
