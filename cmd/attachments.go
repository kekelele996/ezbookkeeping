package cmd

import (
	"fmt"

	"github.com/urfave/cli/v3"

	clis "github.com/mayswind/ezbookkeeping/pkg/cli"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
)

// Attachments represents the attachment maintenance command
var Attachments = &cli.Command{
	Name:  "attachments",
	Usage: "ezBookkeeping transaction attachment maintenance",
	Commands: []*cli.Command{
		{
			Name:   "transaction-pictures",
			Usage:  "Analyze and clean up unused transaction picture files. Only analysis is performed by default, files classified as cleanable will not be deleted until flag \"--execute\" is specified",
			Action: bindAction(cleanupTransactionPictures),
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:     "execute",
					Required: false,
					Usage:    "Actually delete the files classified as cleanable. The classification is re-performed immediately before deletion, and this command can be safely interrupted and re-run",
				},
				&cli.IntFlag{
					Name:     "recent-days",
					Aliases:  []string{"d"},
					Required: false,
					Value:    7,
					Usage:    "The recent safety window in days. Newly uploaded but unsaved pictures and unlisted orphan files within this window are always kept",
				},
				&cli.Int64Flag{
					Name:     "uid",
					Required: false,
					Usage:    "Only analyze or clean up the transaction pictures of the specified user, all users are processed by default",
				},
			},
		},
	},
}

func cleanupTransactionPictures(c *core.CliContext) error {
	_, err := initializeSystem(c)

	if err != nil {
		return err
	}

	executeMode := c.Bool("execute")
	recentSafetyDays := c.Int("recent-days")
	uid := c.Int64("uid")

	if executeMode {
		result, finalReport, err := clis.Attachments.CleanupTransactionPictures(c, uid, recentSafetyDays)

		if err != nil {
			log.CliErrorf(c, "[attachments.cleanupTransactionPictures] error occurs when cleaning up transaction pictures")
			return err
		}

		clis.Attachments.PrintCleanupExecuteResult(result)

		if finalReport != nil {
			fmt.Printf("\n")
			clis.Attachments.PrintCleanupReport(finalReport, true, uid)
		}

		if result.FailedFileCount > 0 {
			return errs.ErrOperationFailed
		}

		log.CliInfof(c, "[attachments.cleanupTransactionPictures] transaction pictures cleanup has been completed, %d file(s) were deleted", result.DeletedFileCount)
	} else {
		report, err := clis.Attachments.AnalyzeTransactionPictures(c, uid, recentSafetyDays)

		if err != nil {
			log.CliErrorf(c, "[attachments.cleanupTransactionPictures] error occurs when analyzing transaction pictures")
			return err
		}

		clis.Attachments.PrintCleanupReport(report, false, uid)
		log.CliInfof(c, "[attachments.cleanupTransactionPictures] transaction pictures analysis has been completed, no file was changed")
	}

	return nil
}
