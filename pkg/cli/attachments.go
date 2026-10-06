package cli

import (
	"fmt"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

const defaultTransactionPictureRecentSafetyDays = 7

// AttachmentsCli represents transaction attachment cli
type AttachmentsCli struct {
	CliUsingConfig
	pictures *services.TransactionPictureService
}

// Initialize a transaction attachment cli singleton instance
var (
	Attachments = &AttachmentsCli{
		CliUsingConfig: CliUsingConfig{
			container: settings.Container,
		},
		pictures: services.TransactionPictures,
	}
)

// AnalyzeTransactionPictures analyzes transaction picture files and returns the classification report without deleting anything
func (l *AttachmentsCli) AnalyzeTransactionPictures(c *core.CliContext, uid int64, recentSafetyDays int) (*services.TransactionPictureCleanupReport, error) {
	if !l.CurrentConfig().EnableTransactionPictures {
		log.CliErrorf(c, "[attachments.AnalyzeTransactionPictures] transaction picture is not enabled in current configuration")
		return nil, errs.ErrNotSupported
	}

	if recentSafetyDays < 0 {
		log.CliErrorf(c, "[attachments.AnalyzeTransactionPictures] recent safety days \"%d\" is invalid", recentSafetyDays)
		return nil, errs.ErrParameterInvalid
	}

	if uid > 0 {
		log.CliInfof(c, "[attachments.AnalyzeTransactionPictures] starting analyzing transaction pictures for user \"uid:%d\"", uid)
	} else {
		log.CliInfof(c, "[attachments.AnalyzeTransactionPictures] starting analyzing transaction pictures for all users")
	}

	report, err := l.pictures.AnalyzeTransactionPictures(c, uid, time.Duration(recentSafetyDays)*24*time.Hour)

	if err != nil {
		log.CliErrorf(c, "[attachments.AnalyzeTransactionPictures] failed to analyze transaction pictures, because %s", err.Error())
		return nil, err
	}

	return report, nil
}

// CleanupTransactionPictures analyzes transaction picture files and deletes only the files classified as cleanable
func (l *AttachmentsCli) CleanupTransactionPictures(c *core.CliContext, uid int64, recentSafetyDays int) (*services.TransactionPictureCleanupExecuteResult, *services.TransactionPictureCleanupReport, error) {
	if !l.CurrentConfig().EnableTransactionPictures {
		log.CliErrorf(c, "[attachments.CleanupTransactionPictures] transaction picture is not enabled in current configuration")
		return nil, nil, errs.ErrNotSupported
	}

	if recentSafetyDays < 0 {
		log.CliErrorf(c, "[attachments.CleanupTransactionPictures] recent safety days \"%d\" is invalid", recentSafetyDays)
		return nil, nil, errs.ErrParameterInvalid
	}

	if uid > 0 {
		log.CliInfof(c, "[attachments.CleanupTransactionPictures] starting cleaning transaction pictures for user \"uid:%d\"", uid)
	} else {
		log.CliInfof(c, "[attachments.CleanupTransactionPictures] starting cleaning transaction pictures for all users")
	}

	result, report, err := l.pictures.ExecuteTransactionPicturesCleanup(c, uid, time.Duration(recentSafetyDays)*24*time.Hour)

	if err != nil {
		log.CliErrorf(c, "[attachments.CleanupTransactionPictures] failed to cleanup transaction pictures, because %s", err.Error())
		return nil, nil, err
	}

	return result, report, nil
}

// PrintCleanupReport prints the analysis report to the standard output
func (l *AttachmentsCli) PrintCleanupReport(report *services.TransactionPictureCleanupReport, executeMode bool, uid int64) {
	fmt.Printf("Transaction picture attachment cleanup report\n")
	fmt.Printf("=============================================\n")
	fmt.Printf("[StorageType] %s\n", l.CurrentConfig().StorageType)

	if l.CurrentConfig().StorageType == settings.LocalFileSystemObjectStorageType {
		fmt.Printf("[StoragePath] %s\n", l.CurrentConfig().LocalFileSystemPath)
	}

	if uid > 0 {
		fmt.Printf("[Scope] user uid:%d\n", uid)
	} else {
		fmt.Printf("[Scope] all users\n")
	}

	recentSafetyDays := int64(report.RecentSafetyDuration / (24 * time.Hour))
	recentSafetyBeforeTime := time.Now().Add(-report.RecentSafetyDuration)
	fmt.Printf("[RecentSafetyWindow] %d days (pending or orphan files newer than %s will always be kept)\n", recentSafetyDays, utils.FormatUnixTimeToLongDateTimeInServerTimezone(recentSafetyBeforeTime.Unix()))
	fmt.Printf("[ScannedFiles] %d\n", report.ScannedFileCount)
	fmt.Printf("\n")

	fmt.Printf("%-46s %10s %14s\n", "Category", "Count", "Size")
	fmt.Printf("%-46s %10s %14s\n", "----------------------------------------------", "----------", "--------------")
	printCleanupCategoryStats("Cleanable (confirmed useless)", report.Cleanable)
	printCleanupReasonStats(report, []string{
		services.TransactionPictureCleanupReasonSoftDeleted,
		services.TransactionPictureCleanupReasonOrphanFile,
	})
	printCleanupCategoryStats("Still referenced (attached)", report.Referenced)
	printCleanupReasonStats(report, []string{
		services.TransactionPictureCleanupReasonAttachedToTransaction,
	})
	printCleanupCategoryStats("Recent pending (keep, maybe in use)", report.RecentPending)
	printCleanupReasonStats(report, []string{
		services.TransactionPictureCleanupReasonPendingNewPicture,
		services.TransactionPictureCleanupReasonPossiblyUploadingOrphan,
	})
	printCleanupCategoryStats("Unconfirmed (keep, needs manual review)", report.Unconfirmed)
	printCleanupReasonStats(report, []string{
		services.TransactionPictureCleanupReasonExpiredPendingNewPicture,
		services.TransactionPictureCleanupReasonActiveRecordMissingFile,
		services.TransactionPictureCleanupReasonInvalidStoragePath,
		services.TransactionPictureCleanupReasonExtensionMismatch,
		services.TransactionPictureCleanupReasonUserMismatch,
		services.TransactionPictureCleanupReasonOrphanModifiedTimeUnknown,
	})
	fmt.Printf("\n")

	fmt.Printf("[SoftDeletedRecordsWithoutFile] %d\n", report.SoftDeletedRecordMissingFileCount)
	fmt.Printf("\n")

	if executeMode {
		return
	}

	if report.Cleanable.Count > 0 {
		fmt.Printf("No file has been deleted. After confirming the report above, re-run this command with flag \"--execute\" to delete the %d cleanable file(s) (%s).\n", report.Cleanable.Count, formatFileSize(report.Cleanable.TotalSize))
	} else {
		fmt.Printf("No cleanable file was found, there is nothing to delete. You can re-run this command at any time.\n")
	}
}

// PrintCleanupExecuteResult prints the cleanup execution result to the standard output
func (l *AttachmentsCli) PrintCleanupExecuteResult(result *services.TransactionPictureCleanupExecuteResult) {
	fmt.Printf("\n")
	fmt.Printf("Transaction picture attachment cleanup result\n")
	fmt.Printf("=============================================\n")
	fmt.Printf("[DeletedFiles] %d (%s)\n", result.DeletedFileCount, formatFileSize(result.DeletedFileSize))
	fmt.Printf("[FailedFiles] %d\n", result.FailedFileCount)

	if result.FailedFileCount > 0 {
		fmt.Printf("The following files cannot be deleted, please check the error log and re-run this command, remaining files will not be affected:\n")

		for i := 0; i < len(result.FailedItems); i++ {
			item := result.FailedItems[i]
			fmt.Printf("  - uid:%d pictureId:%d path:%s reason:%s\n", item.Uid, item.PictureId, item.StoragePath, item.Reason)
		}
	}
}

func printCleanupCategoryStats(categoryName string, stats services.TransactionPictureCleanupCategoryStats) {
	fmt.Printf("%-46s %10d %14s\n", categoryName, stats.Count, formatFileSize(stats.TotalSize))
}

func printCleanupReasonStats(report *services.TransactionPictureCleanupReport, reasons []string) {
	for i := 0; i < len(reasons); i++ {
		reasonStats, exists := report.ReasonStats[reasons[i]]

		if !exists || reasonStats.Count < 1 {
			continue
		}

		fmt.Printf("  - %-44s %10d %14s\n", reasons[i], reasonStats.Count, formatFileSize(reasonStats.TotalSize))
	}
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	unitList := []string{"KiB", "MiB", "GiB", "TiB"}
	fileSize := float64(size) / 1024
	unitIndex := 0

	for fileSize >= 1024 && unitIndex < len(unitList)-1 {
		fileSize = fileSize / 1024
		unitIndex++
	}

	return fmt.Sprintf("%.2f %s", fileSize, unitList[unitIndex])
}
