package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
	"github.com/mayswind/ezbookkeeping/pkg/validators"
)

const (
	testUid int64 = 10001

	expensePrimaryCategoryId  int64 = 2001
	expenseSubCategoryId      int64 = 2002
	transferPrimaryCategoryId int64 = 2003
	transferSubCategoryId     int64 = 2004

	tag1Id int64 = 3001
	tag2Id int64 = 3002

	testTransactionBaseTime int64 = 1700000000
)

// transactionTestEnv contains the initialized transaction test environment
type transactionTestEnv struct {
	cashAccountId int64
	bankAccountId int64
}

func setupTransactionTestEnv(t *testing.T) *transactionTestEnv {
	gin.SetMode(gin.TestMode)

	// Register custom validators (same as web server startup)
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("validTransactionAmount", validators.ValidTransactionAmount)
	}

	dbPath := filepath.Join(t.TempDir(), "ezbookkeeping-test.db")
	err := datastore.InitializeDataStore(&settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{
			DatabaseType: settings.Sqlite3DbType,
			DatabasePath: dbPath,
		},
	})
	require.NoError(t, err)

	err = datastore.Container.UserDataStore.SyncStructs(
		new(models.User),
		new(models.Account),
		new(models.Transaction),
		new(models.TransactionCategory),
		new(models.TransactionTag),
		new(models.TransactionTagIndex),
		new(models.TransactionPictureInfo),
	)
	require.NoError(t, err)

	err = uuid.InitializeUuidGenerator(&settings.Config{
		UuidGeneratorType: settings.InternalUuidGeneratorType,
		UuidServerId:      1,
	})
	require.NoError(t, err)

	nullCtx := core.NewNullContext()

	// Seed user
	fixtureSession := datastore.Container.UserDataStore.Query(nullCtx, testUid)
	defer fixtureSession.Close()

	_, err = fixtureSession.Insert(&models.User{
		Uid:                   testUid,
		Username:              "test_user",
		Email:                 "test@example.com",
		Nickname:              "Test User",
		Password:              "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Salt:                  "0123456789",
		TransactionEditScope:  models.TRANSACTION_EDIT_SCOPE_ALL,
		DefaultCurrency:       "CNY",
	})
	require.NoError(t, err)

	// Create cash account via real account service
	cashAccount := &models.Account{
		Uid:      testUid,
		Category: models.ACCOUNT_CATEGORY_CASH,
		Type:     models.ACCOUNT_TYPE_SINGLE_ACCOUNT,
		Name:     "Cash Account",
		Color:    "ea9553",
		Currency: "CNY",
	}
	require.NoError(t, services.Accounts.CreateAccounts(nullCtx, cashAccount, 0, nil, nil, time.UTC))

	// Create bank account via real account service
	bankAccount := &models.Account{
		Uid:      testUid,
		Category: models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT,
		Type:     models.ACCOUNT_TYPE_SINGLE_ACCOUNT,
		Name:     "Bank Account",
		Color:    "4c91dd",
		Currency: "CNY",
	}
	require.NoError(t, services.Accounts.CreateAccounts(nullCtx, bankAccount, 0, nil, nil, time.UTC))

	now := time.Now().Unix()

	// Seed transaction categories
	_, err = fixtureSession.Insert(
		&models.TransactionCategory{
			CategoryId: expensePrimaryCategoryId, Uid: testUid, Type: models.CATEGORY_TYPE_EXPENSE,
			ParentCategoryId: models.LevelOneTransactionCategoryParentId, Name: "Expense", DisplayOrder: 1,
			Color: "dd5347", CreatedUnixTime: now, UpdatedUnixTime: now,
		},
		&models.TransactionCategory{
			CategoryId: expenseSubCategoryId, Uid: testUid, Type: models.CATEGORY_TYPE_EXPENSE,
			ParentCategoryId: expensePrimaryCategoryId, Name: "Food", DisplayOrder: 1,
			Color: "dd5347", CreatedUnixTime: now, UpdatedUnixTime: now,
		},
		&models.TransactionCategory{
			CategoryId: transferPrimaryCategoryId, Uid: testUid, Type: models.CATEGORY_TYPE_TRANSFER,
			ParentCategoryId: models.LevelOneTransactionCategoryParentId, Name: "Transfer", DisplayOrder: 1,
			Color: "888888", CreatedUnixTime: now, UpdatedUnixTime: now,
		},
		&models.TransactionCategory{
			CategoryId: transferSubCategoryId, Uid: testUid, Type: models.CATEGORY_TYPE_TRANSFER,
			ParentCategoryId: transferPrimaryCategoryId, Name: "Account Transfer", DisplayOrder: 1,
			Color: "888888", CreatedUnixTime: now, UpdatedUnixTime: now,
		},
	)
	require.NoError(t, err)

	// Seed transaction tags
	_, err = fixtureSession.Insert(
		&models.TransactionTag{
			TagId: tag1Id, Uid: testUid, Name: "Tag1", DisplayOrder: 1,
			CreatedUnixTime: now, UpdatedUnixTime: now,
		},
		&models.TransactionTag{
			TagId: tag2Id, Uid: testUid, Name: "Tag2", DisplayOrder: 2,
			CreatedUnixTime: now, UpdatedUnixTime: now,
		},
	)
	require.NoError(t, err)

	return &transactionTestEnv{
		cashAccountId: cashAccount.AccountId,
		bankAccountId: bankAccount.AccountId,
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}

// invokeTransactionModify invokes the transaction modify handler with the specified request
func invokeTransactionModify(t *testing.T, modifyReq models.TransactionModifyRequest) (any, *errs.Error) {
	reqBody, err := json.Marshal(modifyReq)
	require.NoError(t, err)
	return invokeTransactionModifyRaw(t, reqBody)
}

// invokeTransactionModifyRaw invokes the transaction modify handler with the specified raw json body
func invokeTransactionModifyRaw(t *testing.T, reqBody []byte) (any, *errs.Error) {
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/modify", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(core.ClientTimezoneOffsetHeaderName, "0")

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = httpReq

	c := core.WrapWebContext(ginCtx, nil)
	c.SetTokenClaims(&core.UserTokenClaims{Uid: testUid})

	return Transactions.TransactionModifyHandler(c)
}

// baseModifyRequest returns a modify request based on the original base transaction data
func baseModifyRequest(transactionId int64, accountId int64) models.TransactionModifyRequest {
	return models.TransactionModifyRequest{
		Id:              transactionId,
		Type:            models.TRANSACTION_TYPE_EXPENSE,
		CategoryId:      expenseSubCategoryId,
		Time:            testTransactionBaseTime,
		UtcOffset:       0,
		SourceAccountId: accountId,
		SourceAmount:    2000,
		TagIds:          []string{utils.Int64ToString(tag1Id)},
		Comment:         "",
	}
}

// createExpenseTransaction creates a new expense transaction via real transaction service
func createExpenseTransaction(t *testing.T, accountId int64, amount int64, tagIds []int64, unixTime int64) *models.Transaction {
	transaction := &models.Transaction{
		Uid:               testUid,
		Type:              models.TRANSACTION_DB_TYPE_EXPENSE,
		CategoryId:        expenseSubCategoryId,
		TransactionTime:   utils.GetMinTransactionTimeFromUnixTime(unixTime),
		TimezoneUtcOffset: 0,
		AccountId:         accountId,
		Amount:            amount,
	}

	err := services.Transactions.CreateTransaction(core.NewNullContext(), transaction, tagIds, nil)
	require.NoError(t, err)
	return transaction
}

func getTransaction(t *testing.T, transactionId int64) *models.Transaction {
	transaction, err := services.Transactions.GetTransactionByTransactionId(core.NewNullContext(), testUid, transactionId)
	require.NoError(t, err)
	return transaction
}

func getTransactionTagIds(t *testing.T, transactionId int64) []int64 {
	allTagIds, err := services.TransactionTags.GetAllTagIdsOfTransactions(core.NewNullContext(), testUid, []int64{transactionId})
	require.NoError(t, err)
	return allTagIds[transactionId]
}

func getAccount(t *testing.T, accountId int64) *models.Account {
	account, err := services.Accounts.GetAccountByAccountId(core.NewNullContext(), testUid, accountId)
	require.NoError(t, err)
	return account
}

// TestTransactionModify_SuccessWithExpectedVersion verifies normal modification with matching version succeeds
func TestTransactionModify_SuccessWithExpectedVersion(t *testing.T) {
	env := setupTransactionTestEnv(t)

	transaction := createExpenseTransaction(t, env.cashAccountId, 2000, []int64{tag1Id}, testTransactionBaseTime)
	require.Equal(t, int64(1), transaction.Version)

	modifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	modifyReq.Version = int64Pointer(1)
	modifyReq.SourceAmount = 3000
	modifyReq.TagIds = []string{utils.Int64ToString(tag1Id), utils.Int64ToString(tag2Id)}

	result, err := invokeTransactionModify(t, modifyReq)
	require.Nil(t, err)

	transactionResp, ok := result.(*models.TransactionInfoResponse)
	require.True(t, ok, "result should be a transaction response")
	assert.Equal(t, int64(3000), transactionResp.SourceAmount)
	assert.Equal(t, int64(2), transactionResp.Version)
	assert.ElementsMatch(t, []string{"3001", "3002"}, transactionResp.TagIds)
	assert.True(t, transactionResp.Editable)

	// Verify transaction data in database
	updatedTransaction := getTransaction(t, transaction.TransactionId)
	assert.Equal(t, int64(3000), updatedTransaction.Amount)
	assert.Equal(t, int64(2), updatedTransaction.Version)
	assert.ElementsMatch(t, []int64{3001, 3002}, getTransactionTagIds(t, transaction.TransactionId))

	// Verify account balance is updated correctly
	assert.Equal(t, int64(-3000), getAccount(t, env.cashAccountId).Balance)
}

// TestTransactionModify_StaleVersionReturnsConflict verifies modification with stale version returns both current and submitted transactions
func TestTransactionModify_StaleVersionReturnsConflict(t *testing.T) {
	env := setupTransactionTestEnv(t)

	transaction := createExpenseTransaction(t, env.cashAccountId, 2000, []int64{tag1Id}, testTransactionBaseTime)

	// Another device (legacy client) modifies the transaction without version
	legacyModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	legacyModifyReq.SourceAmount = 2500
	legacyResult, legacyErr := invokeTransactionModify(t, legacyModifyReq)
	require.Nil(t, legacyErr)
	require.Equal(t, int64(2), legacyResult.(*models.TransactionInfoResponse).Version)

	// Current client saves with stale version 1
	staleModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	staleModifyReq.Version = int64Pointer(1)
	staleModifyReq.SourceAmount = 3000
	staleModifyReq.TagIds = []string{utils.Int64ToString(tag1Id), utils.Int64ToString(tag2Id)}

	result, err := invokeTransactionModify(t, staleModifyReq)
	assert.Nil(t, result)
	require.NotNil(t, err)
	assert.Equal(t, errs.ErrTransactionVersionConflict.Code(), err.Code())
	assert.Equal(t, http.StatusConflict, err.HttpStatusCode)

	conflictResp, ok := err.Context.(*models.TransactionModifyConflictResponse)
	require.True(t, ok, "error context should be transaction modify conflict response")

	// Verify current transaction data (the other device's modification)
	assert.Equal(t, int64(2500), conflictResp.CurrentTransaction.SourceAmount)
	assert.Equal(t, int64(2), conflictResp.CurrentTransaction.Version)
	assert.ElementsMatch(t, []string{"3001"}, conflictResp.CurrentTransaction.TagIds)

	// Verify submitted transaction data (current client's modification)
	assert.Equal(t, int64(3000), conflictResp.SubmittedTransaction.SourceAmount)
	assert.Equal(t, int64(1), conflictResp.SubmittedTransaction.Version)
	assert.ElementsMatch(t, []string{"3001", "3002"}, conflictResp.SubmittedTransaction.TagIds)

	// Verify database is not overwritten by the stale modification
	storedTransaction := getTransaction(t, transaction.TransactionId)
	assert.Equal(t, int64(2500), storedTransaction.Amount)
	assert.Equal(t, int64(2), storedTransaction.Version)
	assert.Equal(t, int64(-2500), getAccount(t, env.cashAccountId).Balance)
}

// TestTransactionModify_ConflictResolution verifies user can keep server side data or merge both sides after conflict
func TestTransactionModify_ConflictResolution(t *testing.T) {
	env := setupTransactionTestEnv(t)

	transaction := createExpenseTransaction(t, env.cashAccountId, 2000, []int64{tag1Id}, testTransactionBaseTime)

	// Transaction becomes version 2 after another operation
	otherModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	otherModifyReq.Version = int64Pointer(1)
	otherModifyReq.SourceAmount = 2500
	_, err := invokeTransactionModify(t, otherModifyReq)
	require.Nil(t, err)

	// Keep server side: resubmit identical current data with current version, nothing will be updated
	keepServerModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	keepServerModifyReq.Version = int64Pointer(2)
	keepServerModifyReq.SourceAmount = 2500
	keepResult, keepErr := invokeTransactionModify(t, keepServerModifyReq)
	assert.Nil(t, keepResult)
	require.NotNil(t, keepErr)
	assert.Equal(t, errs.ErrNothingWillBeUpdated.Code(), keepErr.Code())

	// Merge both sides: use current version with merged amount and tags
	mergeModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	mergeModifyReq.Version = int64Pointer(2)
	mergeModifyReq.SourceAmount = 3000
	mergeModifyReq.TagIds = []string{utils.Int64ToString(tag1Id), utils.Int64ToString(tag2Id)}
	mergeResult, mergeErr := invokeTransactionModify(t, mergeModifyReq)
	require.Nil(t, mergeErr)

	mergedTransactionResp, ok := mergeResult.(*models.TransactionInfoResponse)
	require.True(t, ok, "merge result should be a transaction response")
	assert.Equal(t, int64(3000), mergedTransactionResp.SourceAmount)
	assert.Equal(t, int64(3), mergedTransactionResp.Version)
	assert.ElementsMatch(t, []string{"3001", "3002"}, mergedTransactionResp.TagIds)

	// Verify merged data in database
	storedTransaction := getTransaction(t, transaction.TransactionId)
	assert.Equal(t, int64(3000), storedTransaction.Amount)
	assert.Equal(t, int64(3), storedTransaction.Version)
	assert.ElementsMatch(t, []int64{3001, 3002}, getTransactionTagIds(t, transaction.TransactionId))
}

// TestTransactionModify_NoVersionKeepsLegacyBehavior verifies requests without version keep the original last-write-wins behavior
func TestTransactionModify_NoVersionKeepsLegacyBehavior(t *testing.T) {
	env := setupTransactionTestEnv(t)

	transaction := createExpenseTransaction(t, env.cashAccountId, 2000, []int64{tag1Id}, testTransactionBaseTime)

	// First modification without version (legacy client behavior)
	firstModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	firstModifyReq.SourceAmount = 2500
	result, err := invokeTransactionModify(t, firstModifyReq)
	require.Nil(t, err)

	firstTransactionResp := result.(*models.TransactionInfoResponse)
	assert.Equal(t, int64(2500), firstTransactionResp.SourceAmount)
	assert.Equal(t, int64(2), firstTransactionResp.Version)

	// Another intervening modification via version-aware client
	newClientModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	newClientModifyReq.Version = int64Pointer(2)
	newClientModifyReq.SourceAmount = 2600
	_, err = invokeTransactionModify(t, newClientModifyReq)
	require.Nil(t, err)

	// Legacy client saves again without version and overwrites successfully (no conflict)
	legacyModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	legacyModifyReq.SourceAmount = 3000
	legacyResult, legacyErr := invokeTransactionModify(t, legacyModifyReq)
	require.Nil(t, legacyErr)
	require.Equal(t, int64(3000), legacyResult.(*models.TransactionInfoResponse).SourceAmount)
	require.Equal(t, int64(4), legacyResult.(*models.TransactionInfoResponse).Version)

	// Raw json payload without version field also works as before
	rawPayload := core.O{
		"id":              utils.Int64ToString(transaction.TransactionId),
		"type":            models.TRANSACTION_TYPE_EXPENSE,
		"categoryId":      utils.Int64ToString(expenseSubCategoryId),
		"time":            testTransactionBaseTime,
		"utcOffset":       0,
		"sourceAccountId": utils.Int64ToString(env.cashAccountId),
		"sourceAmount":    3100,
		"tagIds":          []string{utils.Int64ToString(tag1Id)},
		"comment":         "",
	}
	rawReqBody, marshalErr := json.Marshal(rawPayload)
	require.NoError(t, marshalErr)
	rawResult, rawErr := invokeTransactionModifyRaw(t, rawReqBody)
	require.Nil(t, rawErr)
	assert.Equal(t, int64(3100), rawResult.(*models.TransactionInfoResponse).SourceAmount)
	assert.Equal(t, int64(5), rawResult.(*models.TransactionInfoResponse).Version)
}

// TestTransactionModify_FailureLeavesNoPartialData verifies a failed modification rolls back all data
func TestTransactionModify_FailureLeavesNoPartialData(t *testing.T) {
	env := setupTransactionTestEnv(t)

	transaction := createExpenseTransaction(t, env.cashAccountId, 2000, []int64{tag1Id}, testTransactionBaseTime)

	failModifyReq := baseModifyRequest(transaction.TransactionId, env.cashAccountId)
	failModifyReq.Version = int64Pointer(1)
	failModifyReq.SourceAmount = 9999
	failModifyReq.TagIds = []string{utils.Int64ToString(tag1Id), utils.Int64ToString(tag2Id)}
	failModifyReq.PictureIds = []string{"999999"}

	result, err := invokeTransactionModify(t, failModifyReq)
	assert.Nil(t, result)
	require.NotNil(t, err)
	assert.Equal(t, errs.ErrTransactionPictureNotFound.Code(), err.Code())

	// Verify transaction row is unchanged and version is not bumped
	storedTransaction := getTransaction(t, transaction.TransactionId)
	assert.Equal(t, int64(2000), storedTransaction.Amount)
	assert.Equal(t, int64(1), storedTransaction.Version)

	// Verify tag indexes are unchanged (new tag is not inserted)
	assert.Equal(t, []int64{3001}, getTransactionTagIds(t, transaction.TransactionId))

	// Verify account balance is unchanged
	assert.Equal(t, int64(-2000), getAccount(t, env.cashAccountId).Balance)
}

// TestTransactionModify_TransferTransactionVersion verifies both transfer legs keep the same version and stale version is rejected
func TestTransactionModify_TransferTransactionVersion(t *testing.T) {
	env := setupTransactionTestEnv(t)

	transferTransaction := &models.Transaction{
		Uid:                   testUid,
		Type:                  models.TRANSACTION_DB_TYPE_TRANSFER_OUT,
		CategoryId:            transferSubCategoryId,
		TransactionTime:       utils.GetMinTransactionTimeFromUnixTime(testTransactionBaseTime + 3600),
		TimezoneUtcOffset:     0,
		AccountId:             env.cashAccountId,
		Amount:                1000,
		RelatedAccountId:      env.bankAccountId,
		RelatedAccountAmount:  1000,
	}
	err := services.Transactions.CreateTransaction(core.NewNullContext(), transferTransaction, nil, nil)
	require.NoError(t, err)

	// Normal modification with matching version
	modifyReq := models.TransactionModifyRequest{
		Id:                   transferTransaction.TransactionId,
		Version:              int64Pointer(1),
		Type:                 models.TRANSACTION_TYPE_TRANSFER,
		CategoryId:           transferSubCategoryId,
		Time:                 testTransactionBaseTime + 3600,
		UtcOffset:            0,
		SourceAccountId:      env.cashAccountId,
		DestinationAccountId: env.bankAccountId,
		SourceAmount:         1500,
		DestinationAmount:    1500,
	}
	result, err := invokeTransactionModify(t, modifyReq)
	require.Nil(t, err)

	transferResp, ok := result.(*models.TransactionInfoResponse)
	require.True(t, ok, "result should be a transaction response")
	assert.Equal(t, int64(1500), transferResp.SourceAmount)
	assert.Equal(t, int64(2), transferResp.Version)

	// Verify both legs are updated with the same amount and version
	outLeg := getTransaction(t, transferTransaction.TransactionId)
	inLeg := getTransaction(t, transferTransaction.RelatedId)
	assert.Equal(t, int64(1500), outLeg.Amount)
	assert.Equal(t, int64(1500), inLeg.Amount)
	assert.Equal(t, int64(2), outLeg.Version)
	assert.Equal(t, int64(2), inLeg.Version)

	// Stale version modification is rejected
	staleModifyReq := modifyReq
	staleModifyReq.Version = int64Pointer(1)
	staleModifyReq.SourceAmount = 2000
	staleModifyReq.DestinationAmount = 2000
	staleResult, staleErr := invokeTransactionModify(t, staleModifyReq)
	assert.Nil(t, staleResult)
	require.NotNil(t, staleErr)
	assert.Equal(t, errs.ErrTransactionVersionConflict.Code(), staleErr.Code())

	// Verify both legs remain unchanged
	assert.Equal(t, int64(1500), getTransaction(t, transferTransaction.TransactionId).Amount)
	assert.Equal(t, int64(1500), getTransaction(t, transferTransaction.RelatedId).Amount)
	assert.Equal(t, int64(2), getTransaction(t, transferTransaction.TransactionId).Version)
	assert.Equal(t, int64(2), getTransaction(t, transferTransaction.RelatedId).Version)
}

// TestTransactionModify_ServiceRejectsStaleExpectedVersion verifies service-level expected version check rejects stale modification (race-proof path)
func TestTransactionModify_ServiceRejectsStaleExpectedVersion(t *testing.T) {
	env := setupTransactionTestEnv(t)

	transaction := createExpenseTransaction(t, env.cashAccountId, 2000, []int64{tag1Id}, testTransactionBaseTime)

	staleTransactionModel := &models.Transaction{
		TransactionId:     transaction.TransactionId,
		Uid:               testUid,
		Type:              models.TRANSACTION_DB_TYPE_EXPENSE,
		CategoryId:        expenseSubCategoryId,
		TransactionTime:   utils.GetMinTransactionTimeFromUnixTime(testTransactionBaseTime),
		TimezoneUtcOffset: 0,
		AccountId:         env.cashAccountId,
		Amount:            5000,
	}

	err := services.Transactions.ModifyTransaction(core.NewNullContext(), staleTransactionModel, int64Pointer(999), false, 0, nil, nil, nil, nil)
	require.NotNil(t, err)

	customErr, ok := err.(*errs.Error)
	require.True(t, ok, "error should be a custom error")
	assert.Equal(t, errs.ErrTransactionVersionConflict.Code(), customErr.Code())

	// Verify database is unchanged
	storedTransaction := getTransaction(t, transaction.TransactionId)
	assert.Equal(t, int64(2000), storedTransaction.Amount)
	assert.Equal(t, int64(1), storedTransaction.Version)
	assert.Equal(t, int64(-2000), getAccount(t, env.cashAccountId).Balance)
}
