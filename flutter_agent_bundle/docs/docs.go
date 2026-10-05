package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/v1/auth/signup": {
            "post": {
                "description": "Registers a new user in MongoDB, returns JWT token, and triggers background presence initialization.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Authentication"
                ],
                "summary": "User Registration & Signup",
                "parameters": [
                    {
                        "description": "User profile details",
                        "name": "user",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "properties": {
                                "name": { "type": "string", "example": "John Doe" },
                                "age": { "type": "integer", "example": 24 },
                                "gender": { "type": "string", "example": "male" },
                                "bio": { "type": "string", "example": "Hello world!" },
                                "interest": { "type": "array", "items": { "type": "string" }, "example": ["coding", "gaming"] },
                                "deviceId": { "type": "string", "example": "dev-device-123" }
                            }
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "User created & token issued",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "token": { "type": "string", "example": "eyJhbGciOiJIUzI1Ni..." }
                            }
                        }
                    },
                    "400": { "description": "Invalid JSON or bad request" },
                    "500": { "description": "Internal server error" }
                }
            }
        },
        "/v1/auth/user": {
            "get": {
                "security": [
                    { "BearerAuth": [] }
                ],
                "description": "Retrieves authenticated user profile from MongoDB using Bearer JWT.",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Authentication"
                ],
                "summary": "Get Authenticated User Profile",
                "responses": {
                    "200": {
                        "description": "User profile data",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "user": { "type": "object" }
                            }
                        }
                    },
                    "401": { "description": "Unauthorized / Invalid JWT Token" },
                    "500": { "description": "Internal server error" }
                }
            },
            "patch": {
                "security": [
                    { "BearerAuth": [] }
                ],
                "description": "Partially updates the authenticated user profile using the JWT userId.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Authentication"
                ],
                "summary": "Update Authenticated User Profile",
                "parameters": [
                    {
                        "description": "User profile fields to update",
                        "name": "updateFields",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "properties": {
                                "name": { "type": "string", "example": "Jane Doe" },
                                "age": { "type": "integer", "example": 24 },
                                "bio": { "type": "string", "example": "Updated bio" },
                                "interest": { "type": "array", "items": { "type": "string" }, "example": ["coding", "gaming"] }
                            }
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "User updated successfully",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "message": { "type": "string", "example": "User updated successfully" },
                                "user": { "type": "object" }
                            }
                        }
                    },
                    "400": { "description": "Invalid JSON / No fields provided" },
                    "401": { "description": "Unauthorized / Invalid JWT Token" },
                    "500": { "description": "Internal server error" }
                }
            }
        },
        "/v1/presence": {
            "post": {
                "security": [
                    { "BearerAuth": [] }
                ],
                "description": "Initializes user presence state in Redis (presence:userID).",
                "consumes": [ "application/json" ],
                "produces": [ "application/json" ],
                "tags": [ "Presence" ],
                "summary": "Upsert Initial Presence",
                "parameters": [
                    {
                        "name": "presence",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "properties": {
                                "deviceId": { "type": "string", "example": "dev-device-123" }
                            }
                        }
                    }
                ],
                "responses": {
                    "201": { "description": "Presence created successfully" },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/presence/heartbeat": {
            "post": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Updates LastActiveAt timestamp in Redis for active user heartbeat.",
                "produces": [ "application/json" ],
                "tags": [ "Presence" ],
                "summary": "User Presence Heartbeat",
                "responses": {
                    "200": { "description": "Heartbeat updated successfully" },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/presence/update": {
            "patch": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Updates partial presence state such as typing state (typingTo) or active chat ID (activeChatId).",
                "consumes": [ "application/json" ],
                "produces": [ "application/json" ],
                "tags": [ "Presence" ],
                "summary": "Update Partial Presence Fields",
                "parameters": [
                    {
                        "name": "updateFields",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "properties": {
                                "typingTo": { "type": "string", "example": "target_user_id" },
                                "activeChatId": { "type": "string", "example": "chat_session_id" }
                            }
                        }
                    }
                ],
                "responses": {
                    "200": { "description": "Presence updated successfully" },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/matching/start": {
            "post": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Enters reciprocal matchmaking pool (Interest, Gender, GenderPref). Long-polls up to 30 seconds for a match.",
                "consumes": [ "application/json" ],
                "produces": [ "application/json" ],
                "tags": [ "Matchmaking" ],
                "summary": "Start Matchmaking (30s Long Poll)",
                "parameters": [
                    {
                        "name": "criteria",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "properties": {
                                "interest": { "type": "string", "example": "coding" },
                                "gender": { "type": "string", "example": "male" },
                                "genderPref": { "type": "string", "example": "female" }
                            }
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "Match found or retry requested",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "status": { "type": "string", "example": "matched" },
                                "match": { "type": "object" }
                            }
                        }
                    },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/friendship/requests": {
            "post": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Sends a friend request to a target user and triggers real-time WebSocket popup if target user is connected to /v1/ws.",
                "consumes": [ "application/json" ],
                "produces": [ "application/json" ],
                "tags": [ "Friendship" ],
                "summary": "Send Friend Request",
                "parameters": [
                    {
                        "name": "friendRequest",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "properties": {
                                "receiver_id": { "type": "string", "example": "66fa5a892b11d8c11e74a899" }
                            }
                        }
                    }
                ],
                "responses": {
                    "201": { "description": "Friend request sent successfully" },
                    "400": { "description": "Bad request" },
                    "401": { "description": "Unauthorized" },
                    "500": { "description": "Internal server error" }
                }
            },
            "get": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Retrieves all incoming pending friend requests for the authenticated user.",
                "produces": [ "application/json" ],
                "tags": [ "Friendship" ],
                "summary": "Get Incoming Friend Requests",
                "responses": {
                    "200": {
                        "description": "List of pending friend requests",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "requests": { "type": "array", "items": { "type": "object" } }
                            }
                        }
                    },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/friendship/requests/{requestId}/accept": {
            "post": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Accepts an incoming friend request, creates a Friendship record in MongoDB, and deletes the request.",
                "produces": [ "application/json" ],
                "tags": [ "Friendship" ],
                "summary": "Accept Friend Request",
                "parameters": [
                    {
                        "name": "requestId",
                        "in": "path",
                        "required": true,
                        "type": "string",
                        "description": "Friend Request ObjectID Hex"
                    }
                ],
                "responses": {
                    "200": { "description": "Friend request accepted" },
                    "400": { "description": "Bad request" },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/friendship/requests/{requestId}/decline": {
            "post": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Declines and removes an incoming friend request from MongoDB.",
                "produces": [ "application/json" ],
                "tags": [ "Friendship" ],
                "summary": "Decline Friend Request",
                "parameters": [
                    {
                        "name": "requestId",
                        "in": "path",
                        "required": true,
                        "type": "string",
                        "description": "Friend Request ObjectID Hex"
                    }
                ],
                "responses": {
                    "200": { "description": "Friend request declined" },
                    "400": { "description": "Bad request" },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/friendship/friends": {
            "get": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Retrieves all active established friendships for the authenticated user.",
                "produces": [ "application/json" ],
                "tags": [ "Friendship" ],
                "summary": "Get Active Friends List",
                "responses": {
                    "200": {
                        "description": "List of active friends",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "friends": { "type": "array", "items": { "type": "object" } }
                            }
                        }
                    },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/friendship/friends/{friendshipID}": {
            "delete": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Removes an active friendship record from MongoDB by friendship ID.",
                "produces": [ "application/json" ],
                "tags": [ "Friendship" ],
                "summary": "Remove Friend",
                "parameters": [
                    {
                        "name": "friendshipID",
                        "in": "path",
                        "required": true,
                        "type": "string",
                        "description": "Friendship ObjectID Hex"
                    }
                ],
                "responses": {
                    "200": { "description": "Friend removed successfully" },
                    "400": { "description": "Bad request" },
                    "401": { "description": "Unauthorized" }
                }
            }
        },
        "/v1/ws": {
            "get": {
                "security": [ { "BearerAuth": [] } ],
                "description": "Upgrades HTTP connection to WebSocket protocol. Registers client in WebSocketHub for real-time notifications.",
                "tags": [ "WebSocket" ],
                "summary": "Real-Time WebSocket Connection Endpoint",
                "responses": {
                    "101": { "description": "Switching Protocols to WebSocket" },
                    "401": { "description": "Unauthorized" }
                }
            }
        }
    },
    "securityDefinitions": {
        "BearerAuth": {
            "type": "apiKey",
            "name": "Authorization",
            "in": "header",
            "description": "JWT Authorization header using the Bearer scheme. Example: \"Bearer {token}\""
        }
    }
}`

// SwaggerInfo holds exported Swagger Info of embedded documentation
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "localhost:8080",
	BasePath:         "",
	Schemes:          []string{"http", "https", "ws"},
	Title:            "RanChat API Engine Documentation",
	Description:      "Comprehensive REST & WebSocket API specification for RanChat Omegle-like Chat Platform Backend.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
