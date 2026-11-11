# go-firebase-storage

Firebase Realtime Database storage implementation for go-storage-lock.

## Features

- ✅ Free tier: 100K connections/day, 1GB storage
- ✅ No authentication required (public rules)
- ✅ Multi-tenant isolation via tenant ID
- ✅ REST API integration
- ✅ CAS support via conditional updates

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
