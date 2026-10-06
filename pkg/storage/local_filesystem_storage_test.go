package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

func newLocalFileSystemStorageForTest(t *testing.T, rootPath string, pathPrefix string) *LocalFileSystemObjectStorage {
	storage, err := NewLocalFileSystemObjectStorage(&settings.Config{LocalFileSystemPath: rootPath}, pathPrefix)
	assert.NoError(t, err)
	return storage
}

func writeFileForTest(t *testing.T, fullPath string, content string, modifiedTime time.Time) {
	err := os.MkdirAll(filepath.Dir(fullPath), os.ModePerm)
	assert.NoError(t, err)
	err = os.WriteFile(fullPath, []byte(content), 0644)
	assert.NoError(t, err)
	err = os.Chtimes(fullPath, modifiedTime, modifiedTime)
	assert.NoError(t, err)
}

func TestLocalFileSystemObjectStorageList(t *testing.T) {
	rootPath := t.TempDir()
	storage := newLocalFileSystemStorageForTest(t, rootPath, "transaction")
	ctx := core.NewNullContext()
	modifiedTime := time.Date(2026, 10, 1, 8, 30, 0, 0, time.UTC)

	writeFileForTest(t, filepath.Join(rootPath, "transaction", "1", "1001.jpg"), "referenced", modifiedTime)
	writeFileForTest(t, filepath.Join(rootPath, "transaction", "1", "1002.png"), "soft-deleted", modifiedTime.Add(-24*time.Hour))
	writeFileForTest(t, filepath.Join(rootPath, "transaction", "2", "2001.jpg"), "orphan", modifiedTime.Add(-48*time.Hour))
	writeFileForTest(t, filepath.Join(rootPath, "transaction", "stray.jpg"), "invalid", modifiedTime)

	objects, err := storage.List(ctx, "")
	assert.NoError(t, err)
	assert.Len(t, objects, 4)

	objectMap := make(map[string]ObjectMeta)

	for i := 0; i < len(objects); i++ {
		objectMap[objects[i].Path] = objects[i]
	}

	referencedObject, exists := objectMap["1/1001.jpg"]
	assert.True(t, exists)
	assert.Equal(t, int64(10), referencedObject.Size)
	assert.True(t, modifiedTime.Equal(referencedObject.LastModified))

	_, exists = objectMap["2/2001.jpg"]
	assert.True(t, exists)

	_, exists = objectMap["stray.jpg"]
	assert.True(t, exists)

	// listing with a user directory prefix only returns files of that user
	objects, err = storage.List(ctx, "1/")
	assert.NoError(t, err)
	assert.Len(t, objects, 2)

	for i := 0; i < len(objects); i++ {
		assert.Equal(t, "1/", filepath.ToSlash(filepath.Dir(objects[i].Path))+"/")
	}
}

func TestLocalFileSystemObjectStorageListNonExistentPrefix(t *testing.T) {
	rootPath := t.TempDir()
	storage := newLocalFileSystemStorageForTest(t, rootPath, "transaction")
	ctx := core.NewNullContext()

	objects, err := storage.List(ctx, "999/")
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Len(t, objects, 0)
}

func TestLocalFileSystemObjectStorageDeleteIsIdempotent(t *testing.T) {
	rootPath := t.TempDir()
	storage := newLocalFileSystemStorageForTest(t, rootPath, "transaction")
	ctx := core.NewNullContext()

	objectPath := filepath.Join(rootPath, "transaction", "1", "1001.jpg")
	writeFileForTest(t, objectPath, "data", time.Now())

	err := storage.Delete(ctx, "1/1001.jpg")
	assert.NoError(t, err)

	exists, err := storage.Exists(ctx, "1/1001.jpg")
	assert.NoError(t, err)
	assert.False(t, exists)

	// deleting a non-existent file must still succeed so that interrupted cleanup can be retried safely
	err = storage.Delete(ctx, "1/1001.jpg")
	assert.NoError(t, err)
}
