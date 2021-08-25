package util

import (
	uuid "github.com/google/uuid"
)

type UUID uuid.UUID

func GUID() string {
	return uuid.New().String()
}

func NewUUID() UUID {
	return UUID(uuid.New())
}
