# Ranchat Backend — API Reference

Base URL: `http://localhost:8080`

---

## 🔓 Public Endpoints

### Health Check
| | |
|---|---|
| **Method** | `GET` |
| **URL** | `/health` |
| **Auth** | None |

**Response**
```json
{ "status": "ok", "service": "ranchat-backend" }
```

---

### Anonymous Login
| | |
|---|---|
| **Method** | `POST` |
| **URL** | `/api/v1/auth/anonymous` |
| **Auth** | None |

**Request Body**
```json
{
  "fullName":  "Mayank",         // required
  "gender":    "male",           // required — "male" | "female" | "other"
  "age":       22,               // required — min 13, max 100
  "about":     "Love coding",    // optional
  "interests": ["gym", "ai"],    // optional
  "deviceId":  "device-123"      // required — unique per device
}
```

**Response** `201 Created` (new user) or `200 OK` (returning user)
```json
{
  "success": true,
  "message": "User created successfully",
  "data": {
    "token":     "<jwt_token>",
    "expiresAt": "2026-06-04T18:00:00Z",
    "user": {
      "userId":        "683966e5...",
      "fullName":      "Mayank",
      "gender":        "male",
      "age":           22,
      "about":         "Love coding",
      "interests":     ["gym", "ai"],
      "deviceId":      "device-123",
      "lastSeen":      "2026-05-06T00:00:00Z",
      "premiumTill":   null,
      "isSearching":   false,
      "isMatched":     false,
      "currentMatchId": null,
      "createdAt":     "2026-05-06T00:00:00Z",
      "updatedAt":     "2026-05-06T00:00:00Z"
    }
  }
}
```

> **Note:** Same `deviceId` always returns the same user (idempotent). A fresh JWT is issued every time.

---

## 🔒 Protected Endpoints

All protected endpoints require the following header:

```
Authorization: Bearer <token>
```

---

### Get My Profile
| | |
|---|---|
| **Method** | `GET` |
| **URL** | `/api/v1/users/me` |
| **Auth** | Bearer Token |

Reads the `userId` from the JWT — no URL param needed.

**Response** `200 OK`
```json
{
  "success": true,
  "message": "User fetched",
  "data": { ...user object... }
}
```

---

### Get User by ID
| | |
|---|---|
| **Method** | `GET` |
| **URL** | `/api/v1/users/:userId` |
| **Auth** | Bearer Token |

**URL Params**
| Param | Type | Description |
|-------|------|-------------|
| `userId` | string | MongoDB ObjectID of the target user |

**Response** `200 OK`
```json
{
  "success": true,
  "message": "User fetched",
  "data": { ...user object... }
}
```

**Errors**
| Status | Reason |
|--------|--------|
| `400` | Invalid `userId` format |
| `404` | User not found |

---

### Update My Profile
| | |
|---|---|
| **Method** | `PATCH` |
| **URL** | `/api/v1/users/me` |
| **Auth** | Bearer Token |

**All fields are optional** — only fields present in the request body are updated.

**Request Body**
```json
{
  "fullName":  "New Name",            // optional — min 1 char
  "gender":    "female",              // optional — "male" | "female" | "other"
  "age":       25,                    // optional — min 13, max 100
  "about":     "Updated bio",         // optional
  "interests": ["coding", "travel"]   // optional — replaces entire array
}
```

**Response** `200 OK` — returns the full updated user object
```json
{
  "success": true,
  "message": "User updated",
  "data": { ...updated user object... }
}
```

**Errors**
| Status | Reason |
|--------|--------|
| `400` | No fields provided / validation failed |
| `500` | Database error |

---

## Standard Error Response

All error responses follow this shape:

```json
{
  "success": false,
  "message": "description of the error"
}
```

---

## User Object Reference

```json
{
  "userId":         "683966e5...",      // MongoDB ObjectID (hex string)
  "fullName":       "Mayank",
  "gender":         "male",
  "age":            22,
  "about":          "Love coding",
  "interests":      ["gym", "ai"],
  "deviceId":       "device-123",
  "lastSeen":       "2026-05-06T00:00:00Z",
  "premiumTill":    null,               // null if not premium, ISO date if active
  "isSearching":    false,              // true when user is in matchmaking queue
  "isMatched":      false,              // true when actively in a match
  "currentMatchId": null,               // ObjectID of matched user, or null
  "createdAt":      "2026-05-06T00:00:00Z",
  "updatedAt":      "2026-05-06T00:00:00Z"
}
```

---

## MongoDB Indexes

| Collection | Field | Type | Purpose |
|------------|-------|------|---------|
| `users` | `deviceId` | Unique | One account per device, fast login lookup |
| `users` | `isSearching` | Regular | Fast matchmaking queue queries |

---

## Auth Flow

```
1. POST /api/v1/auth/anonymous  →  receive { token, expiresAt, user }
2. Store token on device
3. Every request: Authorization: Bearer <token>
4. Token expires in 30 days (JWT_EXPIRY_HOURS=720)
5. Re-login with same deviceId to get a fresh token
```
