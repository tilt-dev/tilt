package common

const (
	SecretKey = "Secret"
	RoomIDKey = "room_id"
)

type RoomInfo struct {
	RoomID string `json:"room_id"`
	Secret string `json:"secret"`
}
