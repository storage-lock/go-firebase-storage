# go-firebase-storage

Firebase Realtime Database storage implementation.

> **状态：不可用于分布式锁。** 这个实现可以存取 `LockInformation`，
> 但它不声明 `CapabilityCAS` 与 `CapabilityReliableTime`，`NewStorageLock`
> 会拒绝在它上面创建锁。需要 Firebase 上跑锁时，应改用 Firebase 的
> 事务 API 或专门的强一致后端。

## Features

- ✅ Free tier: 100K connections/day, 1GB storage
- ✅ No authentication required (public rules)
- ✅ Multi-tenant isolation via tenant ID
- ✅ REST API integration
- ⚠️ **不能用作分布式锁后端**：REST API 未使用 ETag/If-Match，UpdateWithVersion 是
  先 GET 再 PUT，不满足线性一致 CAS；GetTime 返回本机时间，不是服务端时钟

## Usage

```go
import (
    firebase_storage "github.com/storage-lock/go-firebase-storage"
)

options := &firebase_storage.FirebaseStorageOptions{
    DatabaseURL: "https://your-project.firebaseio.com",
    TenantID:    "user-uuid", // for data isolation
}
storage, err := firebase_storage.NewFirebaseStorage(ctx, options)
```

## Firebase Setup

1. Create a Firebase project
2. Enable Realtime Database
3. Configure public rules:
```json
{
  "rules": {
    ".read": true,
    ".write": true
  }
}
```

## Security Note

This implementation uses public Firebase rules for zero-configuration usage. Anyone with the database URL can read/write data. For production use, configure Firebase Authentication and restrict rules accordingly.

## License

MIT
