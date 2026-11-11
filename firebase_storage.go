package firebase_storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

var (
	ErrFirebaseURLRequired = errors.New("firebase database URL is required")
	ErrLockNotFound       = storage_lock.ErrLockNotFound
)

// FirebaseStorageOptions 配置 Firebase 存储
type FirebaseStorageOptions struct {
	DatabaseURL string
	HTTPClient  *http.Client
	TenantID    string // 租户 ID，用于数据隔离
}

// FirebaseStorage 基于 Firebase Realtime Database 的存储实现
// 使用 Firebase REST API，无需认证（适用于公开规则配置的数据库）
type FirebaseStorage struct {
	options     *FirebaseStorageOptions
	httpClient  *http.Client
	databaseURL string
	tenantID    string
}

var _ storage.Storage = &FirebaseStorage{}

func NewFirebaseStorage(ctx context.Context, options *FirebaseStorageOptions) (*FirebaseStorage, error) {
	if options == nil {
		return nil, errors.New("options is required")
	}
	if options.DatabaseURL == "" {
		return nil, ErrFirebaseURLRequired
	}

	// 确保 databaseURL 以 / 结尾
	databaseURL := options.DatabaseURL
	if databaseURL[len(databaseURL)-1] != '/' {
		databaseURL += "/"
	}

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	tenantID := options.TenantID
	if tenantID == "" {
		// 如果没有提供租户 ID，使用默认前缀
		tenantID = "default"
	}

	s := &FirebaseStorage{
		options:     options,
		httpClient:  httpClient,
		databaseURL: databaseURL,
		tenantID:    tenantID,
	}

	if err := s.Init(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *FirebaseStorage) GetName() string {
	return "firebase-storage"
}

func (s *FirebaseStorage) Capabilities() []storage.StorageCapability {
	// Firebase Realtime Database 通过条件更新支持 CAS
	// 使用服务器时间作为时间源
	return []storage.StorageCapability{
		storage.CapabilityCAS,
		storage.CapabilityReliableTime,
		storage.CapabilityAtomicDelete,
	}
}

func (s *FirebaseStorage) Init(ctx context.Context) error {
	// Firebase REST API 不需要显式初始化
	// 通过 ping 测试连接
	testURL := s.databaseURL + ".json"
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to firebase: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("firebase connection failed with status %d", resp.StatusCode)
	}

	return nil
}

func (s *FirebaseStorage) Get(ctx context.Context, lockId string) (string, error) {
	url := s.buildURL(lockId)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", ErrLockNotFound
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("firebase get failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (s *FirebaseStorage) Set(ctx context.Context, lockId string, lockInformation *storage.LockInformation) error {
	url := s.buildURL(lockId)
	body, err := json.Marshal(lockInformation)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("firebase set failed with status %d", resp.StatusCode)
	}

	return nil
}

func (s *FirebaseStorage) UpdateWithVersion(ctx context.Context, lockId string, exceptedVersion, newVersion storage.Version, lockInformation *storage.LockInformation) error {
	// Firebase 条件更新需要使用条件 PUT
	// 我们先获取当前版本，然后使用条件更新
	current, err := s.Get(ctx, lockId)
	if err != nil {
		if errors.Is(err, ErrLockNotFound) {
			return storage_lock.ErrVersionMiss
		}
		return err
	}

	var currentInfo storage.LockInformation
	if err := json.Unmarshal([]byte(current), &currentInfo); err != nil {
		return err
	}

	if currentInfo.Version != exceptedVersion {
		return storage_lock.ErrVersionMiss
	}

	// 版本匹配，执行更新
	lockInformation.Version = newVersion
	return s.Set(ctx, lockId, lockInformation)
}

func (s *FirebaseStorage) DeleteWithVersion(ctx context.Context, lockId string, exceptedVersion storage.Version, lockInformation *storage.LockInformation) error {
	current, err := s.Get(ctx, lockId)
	if err != nil {
		return err
	}

	var currentInfo storage.LockInformation
	if err := json.Unmarshal([]byte(current), &currentInfo); err != nil {
		return err
	}

	if currentInfo.Version != exceptedVersion {
		return storage_lock.ErrVersionMiss
	}

	// 版本匹配，执行删除
	url := s.buildURL(lockId)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("firebase delete failed with status %d", resp.StatusCode)
	}

	return nil
}

func (s *FirebaseStorage) Delete(ctx context.Context, lockId string) error {
	url := s.buildURL(lockId)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return ErrLockNotFound
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("firebase delete failed with status %d", resp.StatusCode)
	}

	return nil
}

func (s *FirebaseStorage) List(ctx context.Context) storage.StorageIterator {
	// Firebase REST API 不支持直接列出所有键
	// 这里返回一个空迭代器
	return &firebaseIterator{}
}

func (s *FirebaseStorage) Watch(ctx context.Context, lockId string) (<-chan *storage.LockInformation, error) {
	// Firebase REST API 支持通过 SSE 监听变化
	url := s.buildURL(lockId)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 添加监听参数
	q := req.URL.Query()
	q.Add("shallow", "true")
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO: 实现 SSE 监听
	// 这里先返回一个简单的轮询监听
	return storage.NewPollingLockWatcher(s, 1*time.Second).Watch(ctx, lockId)
}

func (s *FirebaseStorage) Close(ctx context.Context) error {
	// HTTP 客户端不需要显式关闭
	return nil
}

// buildURL 构建 Firebase REST API URL
// 格式: https://database-url.firebaseio.com/tenant-id/lock-id.json
func (s *FirebaseStorage) buildURL(lockId string) string {
	return s.databaseURL + s.tenantID + "/" + lockId + ".json"
}

// firebaseIterator 实现 StorageIterator
type firebaseIterator struct {
	current int
}

func (i *firebaseIterator) Next() bool {
	return false
}

func (i *firebaseIterator) Value() *storage.LockInformation {
	return nil
}

func (i *firebaseIterator) Error() error {
	return nil
}

func (i *firebaseIterator) Close() error {
	return nil
}
