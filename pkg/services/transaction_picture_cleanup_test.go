package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/storage"
)

func newCleanupPictureInfo(uid int64, pictureId int64, extension string, deleted bool, transactionId int64, createdUnixTime int64) *models.TransactionPictureInfo {
	return &models.TransactionPictureInfo{
		Uid:              uid,
		Deleted:          deleted,
		TransactionId:    transactionId,
		PictureId:        pictureId,
		PictureExtension: extension,
		CreatedUnixTime:  createdUnixTime,
	}
}

func TestClassifyListedTransactionPicture(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	recentSafetyDuration := 7 * 24 * time.Hour
	recentSafetyBeforeTime := now.Add(-recentSafetyDuration)
	uid := int64(1)

	pictureInfoMap := map[int64]*models.TransactionPictureInfo{
		101: newCleanupPictureInfo(uid, 101, "jpg", false, 1001, now.Add(-24*time.Hour).Unix()),
		102: newCleanupPictureInfo(uid, 102, "jpg", true, 1002, now.Add(-30*24*time.Hour).Unix()),
		105: newCleanupPictureInfo(uid, 105, "jpg", false, models.TransactionPictureNewPictureTransactionId, now.Add(-24*time.Hour).Unix()),
		106: newCleanupPictureInfo(uid, 106, "jpg", false, models.TransactionPictureNewPictureTransactionId, now.Add(-30*24*time.Hour).Unix()),
		107: newCleanupPictureInfo(uid, 107, "jpg", false, models.TransactionPictureNewPictureTransactionId, now.Add(-24*time.Hour).Unix()),
		108: newCleanupPictureInfo(2, 108, "jpg", false, 2001, now.Add(-24*time.Hour).Unix()),
	}

	testCases := []struct {
		name             string
		pictureId        int64
		extension        string
		lastModifiedTime time.Time
		expectCategory   TransactionPictureCleanupCategory
		expectReason     string
	}{
		{"referenced picture attached to transaction", 101, "jpg", now.Add(-24 * time.Hour), TransactionPictureCleanupCategoryReferenced, TransactionPictureCleanupReasonAttachedToTransaction},
		{"soft deleted picture", 102, "jpg", now.Add(-30 * 24 * time.Hour), TransactionPictureCleanupCategoryCleanable, TransactionPictureCleanupReasonSoftDeleted},
		{"old orphan file without database record", 103, "jpg", now.Add(-30 * 24 * time.Hour), TransactionPictureCleanupCategoryCleanable, TransactionPictureCleanupReasonOrphanFile},
		{"recent orphan file may still be uploading", 104, "jpg", now.Add(-1 * time.Hour), TransactionPictureCleanupCategoryRecentPending, TransactionPictureCleanupReasonPossiblyUploadingOrphan},
		{"recent pending new picture record", 105, "jpg", now.Add(-24 * time.Hour), TransactionPictureCleanupCategoryRecentPending, TransactionPictureCleanupReasonPendingNewPicture},
		{"expired pending new picture record", 106, "jpg", now.Add(-30 * 24 * time.Hour), TransactionPictureCleanupCategoryUnconfirmed, TransactionPictureCleanupReasonExpiredPendingNewPicture},
		{"extension mismatch between file and record", 107, "png", now.Add(-24 * time.Hour), TransactionPictureCleanupCategoryUnconfirmed, TransactionPictureCleanupReasonExtensionMismatch},
		{"user mismatch between file path and record", 108, "jpg", now.Add(-24 * time.Hour), TransactionPictureCleanupCategoryUnconfirmed, TransactionPictureCleanupReasonUserMismatch},
		{"orphan file with unknown modification time", 109, "jpg", time.Time{}, TransactionPictureCleanupCategoryUnconfirmed, TransactionPictureCleanupReasonOrphanModifiedTimeUnknown},
		{"orphan file exactly on the recent safety boundary is kept", 110, "jpg", recentSafetyBeforeTime, TransactionPictureCleanupCategoryRecentPending, TransactionPictureCleanupReasonPossiblyUploadingOrphan},
		{"orphan file one second before the recent safety boundary is cleanable", 112, "jpg", recentSafetyBeforeTime.Add(-1 * time.Second), TransactionPictureCleanupCategoryCleanable, TransactionPictureCleanupReasonOrphanFile},
		{"pending record exactly on the recent safety boundary is kept", 111, "jpg", now, TransactionPictureCleanupCategoryRecentPending, TransactionPictureCleanupReasonPendingNewPicture},
	}

	// the boundary pending record shares the same reason check logic, add it into the map for this case
	pictureInfoMap[111] = newCleanupPictureInfo(uid, 111, "jpg", false, models.TransactionPictureNewPictureTransactionId, recentSafetyBeforeTime.Unix())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			category, reason := classifyListedTransactionPicture(uid, testCase.pictureId, testCase.extension, testCase.lastModifiedTime, pictureInfoMap, recentSafetyBeforeTime)
			assert.Equal(t, testCase.expectCategory, category)
			assert.Equal(t, testCase.expectReason, reason)
		})
	}
}

func TestBuildTransactionPictureCleanupReport(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	recentSafetyDuration := 7 * 24 * time.Hour

	objects := []storage.ObjectMeta{
		{Path: "1/101.jpg", Size: 100, LastModified: now.Add(-24 * time.Hour)},
		{Path: "1/102.jpg", Size: 200, LastModified: now.Add(-30 * 24 * time.Hour)},
		{Path: "1/103.jpg", Size: 300, LastModified: now.Add(-30 * 24 * time.Hour)},
		{Path: "1/104.jpg", Size: 400, LastModified: now.Add(-1 * time.Hour)},
		{Path: "1/105.jpg", Size: 500, LastModified: now.Add(-24 * time.Hour)},
		{Path: "1/106.jpg", Size: 600, LastModified: now.Add(-30 * 24 * time.Hour)},
		{Path: "1/107.png", Size: 700, LastModified: now.Add(-24 * time.Hour)},
		{Path: "1/badid.jpg", Size: 10, LastModified: now.Add(-30 * 24 * time.Hour)},
		{Path: "1/2/3.jpg", Size: 20, LastModified: now.Add(-30 * 24 * time.Hour)},
		{Path: "stray.jpg", Size: 30, LastModified: now.Add(-30 * 24 * time.Hour)},
		{Path: "2/201.jpg", Size: 50, LastModified: now.Add(-48 * time.Hour)},
	}

	pictureInfoMaps := map[int64]map[int64]*models.TransactionPictureInfo{
		1: {
			101: newCleanupPictureInfo(1, 101, "jpg", false, 1001, now.Add(-24*time.Hour).Unix()),
			102: newCleanupPictureInfo(1, 102, "jpg", true, 1002, now.Add(-30*24*time.Hour).Unix()),
			105: newCleanupPictureInfo(1, 105, "jpg", false, models.TransactionPictureNewPictureTransactionId, now.Add(-24*time.Hour).Unix()),
			106: newCleanupPictureInfo(1, 106, "jpg", false, models.TransactionPictureNewPictureTransactionId, now.Add(-30*24*time.Hour).Unix()),
			107: newCleanupPictureInfo(1, 107, "jpg", false, models.TransactionPictureNewPictureTransactionId, now.Add(-24*time.Hour).Unix()),
			108: newCleanupPictureInfo(1, 108, "jpg", false, 1008, now.Add(-10*24*time.Hour).Unix()),
			109: newCleanupPictureInfo(1, 109, "jpg", true, 0, now.Add(-40*24*time.Hour).Unix()),
		},
		2: {
			201: newCleanupPictureInfo(2, 201, "jpg", false, 2001, now.Add(-48*time.Hour).Unix()),
		},
	}

	pictureInfoProvider := func(c core.Context, uid int64) (map[int64]*models.TransactionPictureInfo, error) {
		return pictureInfoMaps[uid], nil
	}

	report, err := buildTransactionPictureCleanupReport(core.NewNullContext(), objects, 0, now, recentSafetyDuration, pictureInfoProvider)
	assert.NoError(t, err)
	assert.Equal(t, int64(11), report.ScannedFileCount)

	assert.Equal(t, int64(2), report.Referenced.Count)
	assert.Equal(t, int64(150), report.Referenced.TotalSize)

	assert.Equal(t, int64(2), report.RecentPending.Count)
	assert.Equal(t, int64(900), report.RecentPending.TotalSize)

	assert.Equal(t, int64(2), report.Cleanable.Count)
	assert.Equal(t, int64(500), report.Cleanable.TotalSize)
	assert.Equal(t, int64(1), report.ReasonStats[TransactionPictureCleanupReasonSoftDeleted].Count)
	assert.Equal(t, int64(200), report.ReasonStats[TransactionPictureCleanupReasonSoftDeleted].TotalSize)
	assert.Equal(t, int64(1), report.ReasonStats[TransactionPictureCleanupReasonOrphanFile].Count)
	assert.Equal(t, int64(300), report.ReasonStats[TransactionPictureCleanupReasonOrphanFile].TotalSize)

	// expired pending (106, 600), extension mismatch (107.png, 700), invalid paths (badid.jpg 10, nested 20, stray 30), active record missing file (108, 0 size)
	assert.Equal(t, int64(6), report.Unconfirmed.Count)
	assert.Equal(t, int64(1360), report.Unconfirmed.TotalSize)
	assert.Equal(t, int64(1), report.ReasonStats[TransactionPictureCleanupReasonExpiredPendingNewPicture].Count)
	assert.Equal(t, int64(1), report.ReasonStats[TransactionPictureCleanupReasonExtensionMismatch].Count)
	assert.Equal(t, int64(3), report.ReasonStats[TransactionPictureCleanupReasonInvalidStoragePath].Count)
	assert.Equal(t, int64(60), report.ReasonStats[TransactionPictureCleanupReasonInvalidStoragePath].TotalSize)
	assert.Equal(t, int64(1), report.ReasonStats[TransactionPictureCleanupReasonActiveRecordMissingFile].Count)
	assert.Equal(t, int64(0), report.ReasonStats[TransactionPictureCleanupReasonActiveRecordMissingFile].TotalSize)

	// soft deleted record 109 has no file and does not produce an item
	assert.Equal(t, int64(1), report.SoftDeletedRecordMissingFileCount)

	// items are sorted deterministically by uid and storage path, invalid paths are recorded with uid 0 and therefore come first
	itemPathList := make([]string, 0, len(report.Items))

	for i := 0; i < len(report.Items); i++ {
		itemPathList = append(itemPathList, report.Items[i].StoragePath)
	}

	assert.Contains(t, itemPathList, "1/101.jpg")
	assert.Contains(t, itemPathList, "1/102.jpg")
	assert.Less(t, indexOfItemPath(t, itemPathList, "1/101.jpg"), indexOfItemPath(t, itemPathList, "1/103.jpg"))
	assert.Less(t, indexOfItemPath(t, itemPathList, "1/103.jpg"), indexOfItemPath(t, itemPathList, "2/201.jpg"))

	// the virtual item for the missing file of an active record points to the expected storage path
	var missingFileItem *TransactionPictureCleanupItem

	for i := 0; i < len(report.Items); i++ {
		if report.Items[i].Reason == TransactionPictureCleanupReasonActiveRecordMissingFile {
			missingFileItem = report.Items[i]
			break
		}
	}

	assert.NotNil(t, missingFileItem)
	assert.Equal(t, int64(1), missingFileItem.Uid)
	assert.Equal(t, int64(108), missingFileItem.PictureId)
	assert.Equal(t, "1/108.jpg", missingFileItem.StoragePath)
	assert.Equal(t, int64(0), missingFileItem.Size)
}

func TestBuildTransactionPictureCleanupReportWithUidFilter(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	recentSafetyDuration := 7 * 24 * time.Hour

	objects := []storage.ObjectMeta{
		{Path: "1/101.jpg", Size: 100, LastModified: now.Add(-24 * time.Hour)},
		{Path: "2/201.jpg", Size: 50, LastModified: now.Add(-48 * time.Hour)},
	}

	requestedUids := make(map[int64]bool)
	pictureInfoProvider := func(c core.Context, uid int64) (map[int64]*models.TransactionPictureInfo, error) {
		requestedUids[uid] = true
		return map[int64]*models.TransactionPictureInfo{}, nil
	}

	report, err := buildTransactionPictureCleanupReport(core.NewNullContext(), objects, 1, now, recentSafetyDuration, pictureInfoProvider)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), report.ScannedFileCount)
	// the file of user 1 is a recent orphan and is kept, the file of user 2 cannot be parsed under the filter and is unconfirmed
	assert.Equal(t, int64(1), report.RecentPending.Count)
	assert.Equal(t, int64(1), report.Unconfirmed.Count)
	assert.Equal(t, int64(0), report.Cleanable.Count)
	assert.True(t, requestedUids[1])
	assert.False(t, requestedUids[2])
}

func TestBuildTransactionPictureCleanupReportProviderError(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	objects := []storage.ObjectMeta{
		{Path: "1/101.jpg", Size: 100, LastModified: now.Add(-24 * time.Hour)},
	}

	pictureInfoProvider := func(c core.Context, uid int64) (map[int64]*models.TransactionPictureInfo, error) {
		return nil, assert.AnError
	}

	report, err := buildTransactionPictureCleanupReport(core.NewNullContext(), objects, 0, now, 24*time.Hour, pictureInfoProvider)
	assert.Nil(t, report)
	assert.Error(t, err)
}

func indexOfItemPath(t *testing.T, itemPathList []string, targetPath string) int {
	for i := 0; i < len(itemPathList); i++ {
		if itemPathList[i] == targetPath {
			return i
		}
	}

	t.Fatalf("item path %s not found in %v", targetPath, itemPathList)
	return -1
}

func TestParseTransactionPictureObjectPath(t *testing.T) {
	testCases := []struct {
		name        string
		objectPath  string
		uidFilter   int64
		expectValid bool
		expectUid   int64
		expectId    int64
		expectExt   string
	}{
		{"normal jpg file", "1/100.jpg", 0, true, 1, 100, "jpg"},
		{"nested path is invalid", "1/2/100.jpg", 0, false, 0, 0, ""},
		{"file without uid directory is invalid", "100.jpg", 0, false, 0, 0, ""},
		{"non numeric uid is invalid", "abc/100.jpg", 0, false, 0, 0, ""},
		{"non numeric picture id is invalid", "1/abc.jpg", 0, false, 0, 0, ""},
		{"unsupported extension is invalid", "1/100.bin", 0, false, 0, 0, ""},
		{"file without extension is invalid", "1/100", 0, false, 0, 0, ""},
		{"other user path is filtered out", "2/100.jpg", 1, false, 0, 0, ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			uid, pictureId, extension, valid := parseTransactionPictureObjectPath(testCase.objectPath, testCase.uidFilter)
			assert.Equal(t, testCase.expectValid, valid)
			assert.Equal(t, testCase.expectUid, uid)
			assert.Equal(t, testCase.expectId, pictureId)
			assert.Equal(t, testCase.expectExt, extension)
		})
	}
}
